package sftp

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	pkgsftp "github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/patterninc/caterpillar/internal/pkg/config"
	"github.com/patterninc/caterpillar/internal/pkg/duration"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

// The package is named sftp, which collides with the github.com/pkg/sftp
// client package, so we import the library under the pkgsftp alias.

const (
	defaultPort       = 22
	defaultTimeout    = duration.Duration(30 * time.Second)
	defaultMaxRetries = 3
	defaultRetryDelay = duration.Duration(1 * time.Second)
)

// ctx is a package-level background context used when creating source records,
// mirroring the file task (internal/pkg/pipeline/task/file/file.go).
var ctx = context.Background()

type sftp struct {
	task.Base `yaml:",inline" json:",inline"`

	// Connection.
	Host     string `yaml:"host" json:"host" validate:"required"`
	Port     int    `yaml:"port,omitempty" json:"port,omitempty"`
	Username string `yaml:"username" json:"username" validate:"required"`

	// Authentication: exactly one of Password or PrivateKey. These come from
	// SSM via {{ secret }} in the pipeline YAML; never log them.
	Password   string `yaml:"password,omitempty" json:"password,omitempty"`
	PrivateKey string `yaml:"private_key,omitempty" json:"private_key,omitempty"`
	Passphrase string `yaml:"passphrase,omitempty" json:"passphrase,omitempty"`

	// Host key verification (required — see buildHostKeyCallback).
	HostKey        string `yaml:"host_key,omitempty" json:"host_key,omitempty"`
	KnownHostsPath string `yaml:"known_hosts_path,omitempty" json:"known_hosts_path,omitempty"`

	// RemotePath is the remote file/directory to download from (source mode) or
	// upload to (sink mode). config.String supports per-record templating.
	RemotePath config.String `yaml:"remote_path,omitempty" json:"remote_path,omitempty" validate:"required"`

	// Reliability.
	Timeout    duration.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	MaxRetries int               `yaml:"max_retries,omitempty" json:"max_retries,omitempty"`
	RetryDelay duration.Duration `yaml:"retry_delay,omitempty" json:"retry_delay,omitempty"`

	// Prepared in Init, used in Run.
	authMethod   ssh.AuthMethod
	hostKeyCB    ssh.HostKeyCallback
	hostKeyAlgos []string
}

func New() (task.Task, error) {
	return &sftp{
		Port:       defaultPort,
		Timeout:    defaultTimeout,
		MaxRetries: defaultMaxRetries,
		RetryDelay: defaultRetryDelay,
	}, nil
}

// Init validates the credentials and host-key settings and prepares the SSH
// auth method and host-key callback. It does not open a connection (that
// happens in Run): Init runs for every task at config-load time, and a session
// held open from then until Run could time out.
func (s *sftp) Init() error {

	authMethod, err := s.buildAuthMethod()
	if err != nil {
		return err
	}
	s.authMethod = authMethod

	hostKeyCB, hostKeyAlgos, err := s.buildHostKeyCallback()
	if err != nil {
		return err
	}
	s.hostKeyCB = hostKeyCB
	s.hostKeyAlgos = hostKeyAlgos

	return nil

}

// Run infers its role from the channels, exactly like the file task: with no
// input it is a source (download from the server); with an input it is a sink
// (upload to the server). It is never both.
func (s *sftp) Run(input <-chan *record.Record, output chan<- *record.Record) error {

	if input != nil && output != nil {
		return task.ErrPresentInputOutput
	}

	sshClient, sftpClient, err := s.connect()
	if err != nil {
		return err
	}
	// LIFO: the SFTP subsystem is torn down before the SSH transport.
	defer sshClient.Close()
	defer sftpClient.Close()

	if input == nil {
		return s.download(sftpClient, output)
	}

	return s.upload(sftpClient, input)

}

// connect dials the SSH transport and opens an SFTP session, retrying on
// transient failures. The dial honours Timeout so a hung server does not block
// the pipeline indefinitely.
func (s *sftp) connect() (*ssh.Client, *pkgsftp.Client, error) {

	addr := net.JoinHostPort(s.Host, strconv.Itoa(s.Port))

	clientConfig := &ssh.ClientConfig{
		User:            s.Username,
		Auth:            []ssh.AuthMethod{s.authMethod},
		HostKeyCallback: s.hostKeyCB,
		// Constrain host-key negotiation to the pinned key's algorithm, so the
		// server presents the same key type we pinned rather than a different
		// one (which would be a spurious mismatch). nil = client default.
		HostKeyAlgorithms: s.hostKeyAlgos,
		Timeout:           time.Duration(s.Timeout),
	}

	var (
		sshClient  *ssh.Client
		sftpClient *pkgsftp.Client
	)

	err := s.retry(`connect`, func() error {
		c, err := ssh.Dial(`tcp`, addr, clientConfig)
		if err != nil {
			return fmt.Errorf(`dialing %s: %w`, addr, err)
		}
		sc, err := pkgsftp.NewClient(c)
		if err != nil {
			c.Close()
			return fmt.Errorf(`opening sftp session on %s: %w`, addr, err)
		}
		sshClient, sftpClient = c, sc
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return sshClient, sftpClient, nil

}

// buildAuthMethod turns the configured credentials into an ssh.AuthMethod.
// Exactly one of PrivateKey or Password must be set. The credentials originate
// from SSM via {{ secret }}; never include them in an error or log line.
func (s *sftp) buildAuthMethod() (ssh.AuthMethod, error) {

	switch {

	case s.PrivateKey != `` && s.Password != ``:
		return nil, fmt.Errorf(`set only one of password or private_key, not both`)

	case s.PrivateKey != ``:
		var (
			signer ssh.Signer
			err    error
		)
		if s.Passphrase != `` {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(s.PrivateKey), []byte(s.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(s.PrivateKey))
		}
		if err != nil {
			return nil, fmt.Errorf(`parsing private_key: %w`, err)
		}
		return ssh.PublicKeys(signer), nil

	case s.Password != ``:
		return ssh.Password(s.Password), nil

	default:
		return nil, fmt.Errorf(`no authentication method configured: set either password or private_key`)

	}

}

// buildHostKeyCallback decides how we verify the server's identity, failing
// closed when neither host_key nor known_hosts_path is set. It also returns the
// host-key algorithms the client should negotiate: for a pinned host_key we
// restrict to that key's algorithm, otherwise the server may present a
// different host-key type than the one we pinned and cause a spurious
// mismatch. A nil slice means "use the client default".
func (s *sftp) buildHostKeyCallback() (ssh.HostKeyCallback, []string, error) {

	switch {

	case s.HostKey != ``:
		// HostKey is a single authorized-key line, e.g.
		//   "ssh-ed25519 AAAAC3Nza..." (the key portion of a known_hosts entry).
		key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(s.HostKey))
		if err != nil {
			return nil, nil, fmt.Errorf(`parsing host_key: %w`, err)
		}
		return ssh.FixedHostKey(key), []string{key.Type()}, nil

	case s.KnownHostsPath != ``:
		callback, err := knownhosts.New(s.KnownHostsPath)
		if err != nil {
			return nil, nil, fmt.Errorf(`loading known_hosts file %q: %w`, s.KnownHostsPath, err)
		}
		return callback, nil, nil

	default:
		return nil, nil, fmt.Errorf(`host key verification required: set host_key or known_hosts_path`)

	}

}

// withRetry runs fn up to attempts times, sleeping delay between tries. It
// returns nil on the first success, or the last error if every attempt fails.
// On each retried failure it logs a warning (matching the codebase's fmt-based
// logging) so a flaky connection is visible even when it eventually succeeds.
func withRetry(label string, attempts int, delay time.Duration, fn func() error) error {

	if attempts < 1 {
		attempts = 1
	}

	var err error
	for i := 0; i < attempts; i++ {
		if err = fn(); err == nil {
			return nil
		}
		// Don't log or sleep after the final attempt.
		if i < attempts-1 {
			fmt.Printf("WARN: %s: attempt %d/%d failed: %v; retrying in %s\n", label, i+1, attempts, err, delay)
			time.Sleep(delay)
		}
	}

	return err

}

// retry wraps withRetry with the task's configured attempt count and delay, and
// a label built from the task name and the action (for example "connect" or
// "upload /incoming/file.csv").
func (s *sftp) retry(action string, fn func() error) error {
	return withRetry(fmt.Sprintf(`sftp task %q: %s`, s.Name, action), s.MaxRetries, time.Duration(s.RetryDelay), fn)
}
