package sftp

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/patterninc/caterpillar/internal/pkg/duration"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

// testKeyPair generates an ed25519 key and returns a PEM-encoded private key
// plus its matching authorized-key (host key) line, so auth and host-key tests
// use real, parseable material.
func testKeyPair(t *testing.T) (privatePEM string, authorizedKey string) {
	t.Helper()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	block, err := ssh.MarshalPrivateKey(priv, ``)
	require.NoError(t, err)

	signer, err := ssh.NewSignerFromKey(priv)
	require.NoError(t, err)

	return string(pem.EncodeToMemory(block)), string(ssh.MarshalAuthorizedKey(signer.PublicKey()))
}

func TestNewDefaults(t *testing.T) {
	tsk, err := New()
	require.NoError(t, err)

	s, ok := tsk.(*sftp)
	require.True(t, ok)

	assert.Equal(t, defaultPort, s.Port)
	assert.Equal(t, defaultTimeout, s.Timeout)
	assert.Equal(t, defaultMaxRetries, s.MaxRetries)
	assert.Equal(t, defaultRetryDelay, s.RetryDelay)
}

func TestWithRetry(t *testing.T) {
	tests := []struct {
		name      string
		attempts  int
		failTimes int // number of leading calls that return an error
		wantCalls int
		wantErr   bool
	}{
		{name: "success first try", attempts: 3, failTimes: 0, wantCalls: 1, wantErr: false},
		{name: "success after retries", attempts: 3, failTimes: 2, wantCalls: 3, wantErr: false},
		{name: "all attempts fail", attempts: 3, failTimes: 3, wantCalls: 3, wantErr: true},
		{name: "attempts below one is clamped", attempts: 0, failTimes: 0, wantCalls: 1, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			err := withRetry("test", tt.attempts, time.Millisecond, func() error {
				calls++
				if calls <= tt.failTimes {
					return errors.New("boom")
				}
				return nil
			})
			assert.Equal(t, tt.wantCalls, calls)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBuildAuthMethod(t *testing.T) {
	privatePEM, _ := testKeyPair(t)

	tests := []struct {
		name       string
		password   string
		privateKey string
		wantErr    bool
	}{
		{name: "password", password: "hunter2", wantErr: false},
		{name: "private key", privateKey: privatePEM, wantErr: false},
		{name: "both set is rejected", password: "x", privateKey: privatePEM, wantErr: true},
		{name: "neither set is rejected", wantErr: true},
		{name: "invalid private key", privateKey: "not a key", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &sftp{Password: tt.password, PrivateKey: tt.privateKey}
			method, err := s.buildAuthMethod()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, method)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, method)
			}
		})
	}
}

func TestBuildHostKeyCallback(t *testing.T) {
	_, authorizedKey := testKeyPair(t)

	tests := []struct {
		name    string
		hostKey string
		wantErr bool
	}{
		{name: "valid host key", hostKey: authorizedKey, wantErr: false},
		{name: "fail closed when nothing configured", wantErr: true},
		{name: "invalid host key", hostKey: "garbage", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &sftp{HostKey: tt.hostKey}
			cb, err := s.buildHostKeyCallback()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, cb)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cb)
			}
		})
	}
}

func TestValidateChannels(t *testing.T) {
	// A non-nil receive-only / send-only channel for the "present" cases.
	bidi := make(chan *record.Record)
	var inCh <-chan *record.Record = bidi
	var outCh chan<- *record.Record = bidi

	tests := []struct {
		name      string
		operation string
		input     <-chan *record.Record
		output    chan<- *record.Record
		wantErr   bool
	}{
		{name: "upload needs input", operation: opUpload, input: nil, output: nil, wantErr: true},
		{name: "upload with input ok", operation: opUpload, input: inCh, output: nil, wantErr: false},
		{name: "download needs output", operation: opDownload, input: nil, output: nil, wantErr: true},
		{name: "download with input rejected", operation: opDownload, input: inCh, output: outCh, wantErr: true},
		{name: "download as source ok", operation: opDownload, input: nil, output: outCh, wantErr: false},
		{name: "list as source ok", operation: opList, input: nil, output: outCh, wantErr: false},
		{name: "move has no channel requirement", operation: opMove, input: nil, output: nil, wantErr: false},
		{name: "delete has no channel requirement", operation: opDelete, input: nil, output: nil, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &sftp{Operation: tt.operation}
			err := s.validateChannels(tt.input, tt.output)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestContainsGlob(t *testing.T) {
	assert.True(t, containsGlob("/in/*.csv"))
	assert.True(t, containsGlob("/in/file?.txt"))
	assert.True(t, containsGlob("/in/[ab].txt"))
	assert.False(t, containsGlob("/in/file.txt"))
}

// TestStructValidation exercises the validate:"..." struct tags the way the
// pipeline does at config-load time (pipeline/tasks.go validates each task).
func TestStructValidation(t *testing.T) {
	validate := validator.New()

	valid := &sftp{Operation: opUpload, Host: "sftp.example.com", Username: "user"}
	assert.NoError(t, validate.Struct(valid))

	tests := []struct {
		name string
		task *sftp
	}{
		{name: "missing operation", task: &sftp{Host: "h", Username: "u"}},
		{name: "unknown operation", task: &sftp{Operation: "frobnicate", Host: "h", Username: "u"}},
		{name: "missing host", task: &sftp{Operation: opUpload, Username: "u"}},
		{name: "missing username", task: &sftp{Operation: opUpload, Host: "h"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Error(t, validate.Struct(tt.task))
		})
	}
}

// Ensure duration defaults convert as expected (guards against accidental unit
// changes in the constants).
func TestDefaultDurations(t *testing.T) {
	assert.Equal(t, 30*time.Second, time.Duration(defaultTimeout))
	assert.Equal(t, 1*time.Second, time.Duration(defaultRetryDelay))
	assert.Equal(t, duration.Duration(30*time.Second), defaultTimeout)
}
