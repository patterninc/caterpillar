package jq

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestHashFunctions(t *testing.T) {

	tests := []struct {
		name     string
		expr     string
		expected string
	}{
		{
			name:     "md5",
			expr:     `"abc" | md5`,
			expected: "900150983cd24fb0d6963f7d28e17f72",
		},
		{
			name:     "sha256",
			expr:     `"abc" | sha256`,
			expected: "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
		},
		{
			name:     "sha512",
			expr:     `"abc" | sha512`,
			expected: "ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f",
		},
		{
			name:     "sha256 of empty string",
			expr:     `"" | sha256`,
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mustQuery(t, tt.expr).Execute([]byte(`null`))
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}

}

// Vectors from RFC 2104 / RFC 4231 test case 2.
func TestMessageAuthenticationFunctions(t *testing.T) {

	tests := []struct {
		name     string
		expr     string
		expected string
	}{
		{
			name:     "hmac_md5",
			expr:     `hmac_md5("what do ya want for nothing?"; "Jefe")`,
			expected: "750c783e6ab0b503eaa86e310a5db738",
		},
		{
			name:     "hmac_sha256",
			expr:     `hmac_sha256("what do ya want for nothing?"; "Jefe")`,
			expected: "5bdcc146bf60754e6a042426089575c75a003f089d2739839dec58b964ec3843",
		},
		{
			name:     "hmac_sha512",
			expr:     `hmac_sha512("what do ya want for nothing?"; "Jefe")`,
			expected: "164b7a7bfcf819e2e395fbe73b56e0a387bd64222e831fd610270cd7ea2505549758bf75c05a994a6d034f65f8f0e6fdcaeab1a34d4a6b4b636e070a38bce737",
		},
		{
			name:     "prefix bytes are prepended to the digest",
			expr:     `hmac_sha256("what do ya want for nothing?"; "Jefe"; "AB")`,
			expected: hex.EncodeToString([]byte("AB")) + "5bdcc146bf60754e6a042426089575c75a003f089d2739839dec58b964ec3843",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mustQuery(t, tt.expr).Execute([]byte(`null`))
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}

}

func TestUUIDFunction(t *testing.T) {

	pattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

	got, err := mustQuery(t, `[uuid, uuid]`).Execute([]byte(`null`))
	require.NoError(t, err)

	pair, ok := got.([]any)
	require.True(t, ok)
	require.Len(t, pair, 2)
	for _, v := range pair {
		assert.Regexp(t, pattern, v)
	}
	assert.NotEqual(t, pair[0], pair[1])

}

var (
	testKeyOnce sync.Once
	testKey     *rsa.PrivateKey
)

func rsaTestKey(t *testing.T) *rsa.PrivateKey {

	t.Helper()

	testKeyOnce.Do(func() {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			panic(err)
		}
		testKey = key
	})

	return testKey

}

func TestParseAnyPrivateKey(t *testing.T) {

	key := rsaTestKey(t)
	pkcs1 := x509.MarshalPKCS1PrivateKey(key)
	pkcs8, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)

	tests := []struct {
		name    string
		input   []byte
		wantErr string
	}{
		{
			name:  "PKCS#1 PEM",
			input: pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: pkcs1}),
		},
		{
			name:  "PKCS#8 PEM",
			input: pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}),
		},
		{
			name:  "PKCS#1 DER",
			input: pkcs1,
		},
		{
			name:  "PKCS#8 DER",
			input: pkcs8,
		},
		{
			name:    "Unsupported PEM block",
			input:   pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: []byte{1}}),
			wantErr: "unsupported key type",
		},
		{
			name:    "Corrupt PKCS#1 PEM",
			input:   pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: []byte{1, 2, 3}}),
			wantErr: "PKCS#1",
		},
		{
			name:    "Corrupt PKCS#8 PEM",
			input:   pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte{1, 2, 3}}),
			wantErr: "PKCS#8",
		},
		{
			name:    "Garbage",
			input:   []byte("not a key"),
			wantErr: "not a valid RSA private key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAnyPrivateKey(tt.input)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.True(t, key.Equal(got))
		})
	}

}

func TestSignFunctions(t *testing.T) {

	key := rsaTestKey(t)
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))

	digest256 := sha256.Sum256([]byte("payload"))
	digest512 := sha512.Sum512([]byte("payload"))

	tests := []struct {
		name   string
		fn     string
		digest []byte
		hash   crypto.Hash
	}{
		{name: "rsa_sha256", fn: "rsa_sha256", digest: digest256[:], hash: crypto.SHA256},
		{name: "rsa_sha512", fn: "rsa_sha512", digest: digest512[:], hash: crypto.SHA512},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := fmt.Sprintf(`%s(.digest; .key)`, tt.fn)
			doc := fmt.Sprintf(`{"digest":%q,"key":%q}`, hex.EncodeToString(tt.digest), keyPEM)

			got, err := mustQuery(t, expr).Execute([]byte(doc))
			require.NoError(t, err)

			signature, err := base64.StdEncoding.DecodeString(got.(string))
			require.NoError(t, err)
			assert.NoError(t, rsa.VerifyPKCS1v15(&key.PublicKey, tt.hash, tt.digest, signature))
		})
	}

	t.Run("non-hex digest is rejected", func(t *testing.T) {
		_, err := signFuncs["rsa_sha256"]("zz", []byte(keyPEM))
		assert.Error(t, err)
	})

	t.Run("bad key is rejected", func(t *testing.T) {
		_, err := signFuncs["rsa_sha512"](hex.EncodeToString(digest512[:]), []byte("nope"))
		assert.Error(t, err)
	})

}

func TestParseBcryptSalt(t *testing.T) {

	tests := []struct {
		name        string
		salt        string
		wantSalt    string
		wantVersion string
		wantCost    int
		wantErr     bool
	}{
		{
			name:        "Bare salt uses defaults",
			salt:        "XajjQvNhvvRt5GSeFk1xFe",
			wantSalt:    "XajjQvNhvvRt5GSeFk1xFe",
			wantVersion: "2a",
			wantCost:    4,
		},
		{
			name:        "Prefixed salt",
			salt:        "$2b$06$XajjQvNhvvRt5GSeFk1xFe",
			wantSalt:    "XajjQvNhvvRt5GSeFk1xFe",
			wantVersion: "2b",
			wantCost:    6,
		},
		{
			name:        "Full hash is accepted as a salt",
			salt:        "$2a$04$XajjQvNhvvRt5GSeFk1xFeFSjbRlNk/Ta1yxtAEbNtK1N.ZPZVHUu",
			wantSalt:    "XajjQvNhvvRt5GSeFk1xFeFSjbRlNk/Ta1yxtAEbNtK1N.ZPZVHUu",
			wantVersion: "2a",
			wantCost:    4,
		},
		{
			name:    "Too few fields",
			salt:    "$2a$04",
			wantErr: true,
		},
		{
			name:    "Non-numeric cost",
			salt:    "$2a$xx$XajjQvNhvvRt5GSeFk1xFe",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			salt, version, cost, err := parseBcryptSalt(tt.salt)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantSalt, salt)
			assert.Equal(t, tt.wantVersion, version)
			assert.Equal(t, tt.wantCost, cost)
		})
	}

}

func TestBcryptHash(t *testing.T) {

	const salt = "XajjQvNhvvRt5GSeFk1xFe"

	t.Run("matches the reference implementation", func(t *testing.T) {
		for _, password := range []string{"", "U*U", "correct horse battery staple", strings.Repeat("x", 72)} {
			got, err := bcryptHash(password, "2a", 4, salt)
			require.NoError(t, err)
			assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(got), []byte(password)), "password %q", password)
		}
	})

	t.Run("known answer", func(t *testing.T) {
		got, err := mustQuery(t, `bcrypt("U*U"; "$2a$05$CCCCCCCCCCCCCCCCCCCCC.")`).Execute([]byte(`null`))
		require.NoError(t, err)
		assert.Equal(t, "$2a$05$CCCCCCCCCCCCCCCCCCCCC.E5YPO9kmyuRGyh0XouQYb4YMJKvyOeW", got)
	})

	t.Run("salt longer than 22 characters is truncated", func(t *testing.T) {
		short, err := bcryptHash("pw", "2b", 4, salt)
		require.NoError(t, err)
		long, err := bcryptHash("pw", "2b", 4, salt+"ignored")
		require.NoError(t, err)
		assert.Equal(t, short, long)
		assert.True(t, strings.HasPrefix(short, "$2b$04$"+salt))
	})

	errorCases := []struct {
		name    string
		data    string
		version string
		cost    int
		salt    string
		wantErr string
	}{
		{name: "2x is unsupported", data: "pw", version: "2x", cost: 4, salt: salt, wantErr: "unsupported bcrypt version"},
		{name: "cost too low", data: "pw", version: "2a", cost: 3, salt: salt, wantErr: "outside allowed range"},
		{name: "cost too high", data: "pw", version: "2a", cost: 32, salt: salt, wantErr: "outside allowed range"},
		{name: "data longer than 72 bytes", data: strings.Repeat("x", 73), version: "2a", cost: 4, salt: salt, wantErr: "exceeds 72 bytes"},
		{name: "salt too short", data: "pw", version: "2a", cost: 4, salt: "short", wantErr: "at least 22 characters"},
		{name: "salt not bcrypt base64", data: "pw", version: "2a", cost: 4, salt: "!!!!!!!!!!!!!!!!!!!!!!", wantErr: "not valid bcrypt base64"},
	}

	for _, tt := range errorCases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := bcryptHash(tt.data, tt.version, tt.cost, tt.salt)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}

}
