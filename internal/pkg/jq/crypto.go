package jq

import (
	"crypto"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/itchyny/gojq"
	"golang.org/x/crypto/blowfish"
)

type hashFunc func(string) string
type messageAuthenticationFunc func(data string, key []byte, pref []byte) string
type signFunc func(data string, privateKey []byte) (string, error)

const (
	bcryptRawSaltLength     = 16
	bcryptEncodedSaltLength = 22
	bcryptChecksumBytes     = 23
	bcryptMaxDataLength     = 72
	bcryptDefaultVersion    = "2a"
	bcryptDefaultCost       = 4
	bcryptMinCost           = 4
	bcryptMaxCost           = 31
)

// bcrypt orders its base64 alphabet differently from standard base64, so the two
// encodings are not interchangeable.
var bcryptEncoding = base64.NewEncoding("./ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789").WithPadding(base64.NoPadding)

// IV for bcrypt's 64 Blowfish encryption rounds.
var bcryptMagicCipherData = []byte("OrpheanBeholderScryDoubt")

// 2x is excluded deliberately: it exists to reproduce crypt_blowfish's 8-bit
// sign-extension bug, which Go does not implement, so output labelled 2x would
// disagree with a real 2x implementation for non-ASCII input.
var bcryptSupportedVersions = map[string]struct{}{
	"2":  {},
	"2a": {},
	"2b": {},
	"2y": {},
}

// Supported hash algorithms
var hashFuncs = map[string]hashFunc{
	"md5": func(s string) string {
		md5Hash := md5.Sum([]byte(s))
		return hex.EncodeToString(md5Hash[:])
	},
	"sha256": func(s string) string {
		sha256Hash := sha256.Sum256([]byte(s))
		return hex.EncodeToString(sha256Hash[:])
	},
	"sha512": func(s string) string {
		sha512Hash := sha512.Sum512([]byte(s))
		return hex.EncodeToString(sha512Hash[:])
	},
}

// Supported digital signing algorithms
var signFuncs = map[string]signFunc{
	"rsa_sha256": func(data string, privateKey []byte) (string, error) {
		rsaKey, err := parseAnyPrivateKey(privateKey)
		if err != nil {
			return "", err
		}
		hashed, err := hex.DecodeString(data)
		if err != nil {
			return "", err
		}
		signature, err := rsa.SignPKCS1v15(rand.Reader, rsaKey, crypto.SHA256, hashed)
		if err != nil {
			return "", err
		}
		return base64.StdEncoding.EncodeToString(signature), nil
	},
	"rsa_sha512": func(data string, privateKey []byte) (string, error) {
		rsaKey, err := parseAnyPrivateKey(privateKey)
		if err != nil {
			return "", err
		}
		hashed, err := hex.DecodeString(data)
		if err != nil {
			return "", err
		}
		signature, err := rsa.SignPKCS1v15(rand.Reader, rsaKey, crypto.SHA512, hashed)
		if err != nil {
			return "", err
		}
		return base64.StdEncoding.EncodeToString(signature), nil
	},
}

var messageAuthenticationFuncs = map[string]messageAuthenticationFunc{
	"hmac_sha256": func(data string, key []byte, pref []byte) string {
		h := hmac.New(sha256.New, key)
		h.Write([]byte(data))
		return hex.EncodeToString(h.Sum(pref))
	},
	"hmac_sha512": func(data string, key []byte, pref []byte) string {
		h := hmac.New(sha512.New, key)
		h.Write([]byte(data))
		return hex.EncodeToString(h.Sum(pref))
	},
	"hmac_md5": func(data string, key []byte, pref []byte) string {
		h := hmac.New(md5.New, key)
		h.Write([]byte(data))
		return hex.EncodeToString(h.Sum(pref))
	},
}

// Supported UUID generation algorithms
var uuidFuncs = map[string]func() string{
	"uuid": func() string {
		return uuid.New().String()
	},
}

// Returns gojq.CompilerOption supported hash algorithms
func cryptoHashOptions() []gojq.CompilerOption {
	options := make([]gojq.CompilerOption, 0, len(hashFuncs))
	for name, fn := range hashFuncs {
		opt := gojq.WithFunction(name, 0, 0, func(raw any, _ []any) any {
			str, ok := raw.(string)
			if !ok {
				return fmt.Errorf("expected string for %s, got %T", name, raw)
			}
			return fn(str)
		})
		options = append(options, opt)
	}
	return options
}

// Returns gojq.CompilerOption UUID generation algorithms
func uuidOptions() []gojq.CompilerOption {
	options := make([]gojq.CompilerOption, 0, len(uuidFuncs))
	for name, fn := range uuidFuncs {
		opt := gojq.WithFunction(name, 0, 0, func(_ any, _ []any) any {
			return fn()
		})
		options = append(options, opt)
	}
	return options
}

// Returns gojq.CompilerOption supported digital signing algorithms
func signOptions() []gojq.CompilerOption {
	options := make([]gojq.CompilerOption, 0, len(signFuncs))
	for name, fn := range signFuncs {
		opt := gojq.WithFunction(name, 2, 2, func(_ any, args []any) any {
			// args[0] is the data to sign, args[1] is the private key
			data, ok := args[0].(string)
			if !ok {
				return fmt.Errorf("expected string for data for sign  %s, got %T", name, args[0])
			}
			key, ok := args[1].(string)
			if !ok {
				return fmt.Errorf("expected string for key for sign %s, got %T", name, args[1])
			}
			result, err := fn(data, []byte(key))
			if err != nil {
				return err
			}
			return result
		})
		options = append(options, opt)
	}
	return options
}

// Returns gojq.CompilerOption supported message authentication algorithms
func messageAuthOptions() []gojq.CompilerOption {
	var options []gojq.CompilerOption
	for name, fn := range messageAuthenticationFuncs {
		opt := gojq.WithFunction(name, 2, 3, func(_ any, args []any) any {
			data, ok := args[0].(string)
			if !ok {
				return fmt.Errorf("expected string for data %s, got %T", name, args[0])
			}
			key, ok := args[1].(string)
			if !ok {
				return fmt.Errorf("expected string for key %s, got %T", name, args[0])
			}
			var pref = []byte{}
			if len(args) == 3 {
				p, ok := args[2].(string)
				if !ok {
					return fmt.Errorf("expected string for pref %s, got %T", name, args[2])
				}
				pref = []byte(p)
			}
			return fn(data, []byte(key), pref)
		})
		options = append(options, opt)
	}
	return options
}

// Returns gojq.CompilerOption for bcrypt hashing against a caller-supplied salt.
//
// x/crypto/bcrypt exposes only GenerateFromPassword, which always draws its own random
// salt, so any scheme deriving a value from bcrypt(data, fixed_salt) is unreachable
// through it. Usage: "data" | bcrypt($salt), where the salt's modular-crypt prefix
// selects the version and cost.
func bcryptOptions() []gojq.CompilerOption {

	opt := gojq.WithFunction("bcrypt", 1, 1, func(raw any, args []any) any {

		data, ok := raw.(string)
		if !ok {
			return fmt.Errorf("expected string for data for bcrypt, got %T", raw)
		}

		salt, ok := args[0].(string)
		if !ok {
			return fmt.Errorf("expected string for salt for bcrypt, got %T", args[0])
		}

		encodedSalt, version, cost, err := parseBcryptSalt(salt)
		if err != nil {
			return err
		}

		result, err := bcryptHash(data, version, cost, encodedSalt)
		if err != nil {
			return err
		}

		return result

	})

	return []gojq.CompilerOption{opt}

}

// Accepts a bare 22-character salt or a modular-crypt string ($2a$04$<salt>), returning
// the salt plus whichever version and cost the prefix carried. A full hash is also a
// valid argument - its salt is the leading 22 characters of the trailing field, the same
// way CompareHashAndPassword recovers one.
func parseBcryptSalt(salt string) (string, string, int, error) {

	if !strings.HasPrefix(salt, "$") {
		return salt, bcryptDefaultVersion, bcryptDefaultCost, nil
	}

	fields := strings.Split(strings.TrimPrefix(salt, "$"), "$")
	if len(fields) < 3 {
		return "", "", 0, fmt.Errorf("malformed bcrypt salt: want $version$cost$salt")
	}

	cost, err := strconv.Atoi(fields[1])
	if err != nil {
		return "", "", 0, fmt.Errorf("malformed bcrypt cost %q: %w", fields[1], err)
	}

	return fields[2], fields[0], cost, nil

}

func bcryptHash(data, version string, cost int, encodedSalt string) (string, error) {

	if _, ok := bcryptSupportedVersions[version]; !ok {
		return "", fmt.Errorf("unsupported bcrypt version %q, want 2, 2a, 2b or 2y", version)
	}

	if cost < bcryptMinCost || cost > bcryptMaxCost {
		return "", fmt.Errorf("bcrypt cost %d outside allowed range %d..%d", cost, bcryptMinCost, bcryptMaxCost)
	}

	// bcrypt reads only the first 72 bytes. Rejecting rather than truncating stops two
	// different payloads from yielding one hash.
	if len(data) > bcryptMaxDataLength {
		return "", fmt.Errorf("bcrypt data length %d exceeds %d bytes", len(data), bcryptMaxDataLength)
	}

	if len(encodedSalt) < bcryptEncodedSaltLength {
		return "", fmt.Errorf("bcrypt salt must be at least %d characters, got %d", bcryptEncodedSaltLength, len(encodedSalt))
	}
	encodedSalt = encodedSalt[:bcryptEncodedSaltLength]

	salt, err := bcryptEncoding.DecodeString(encodedSalt)
	if err != nil {
		return "", fmt.Errorf("bcrypt salt is not valid bcrypt base64: %w", err)
	}

	if len(salt) != bcryptRawSaltLength {
		return "", fmt.Errorf("bcrypt salt must decode to %d bytes, got %d", bcryptRawSaltLength, len(salt))
	}

	cipher, err := expensiveBlowfishSetup([]byte(data), uint32(cost), salt)
	if err != nil {
		return "", err
	}

	block := make([]byte, len(bcryptMagicCipherData))
	copy(block, bcryptMagicCipherData)

	for i := 0; i < len(block); i += 8 {
		for j := 0; j < 64; j++ {
			cipher.Encrypt(block[i:i+8], block[i:i+8])
		}
	}

	// Only 23 of the 24 encrypted bytes are encoded, matching the C implementations.
	// Encoding all 24 yields a checksum no other bcrypt agrees with.
	return fmt.Sprintf("$%s$%02d$%s%s", version, cost, encodedSalt, bcryptEncoding.EncodeToString(block[:bcryptChecksumBytes])), nil

}

// Runs bcrypt's cost-driven key schedule. The Blowfish calls are the same exported ones
// x/crypto/bcrypt uses internally, so no cryptographic primitive is reimplemented here.
func expensiveBlowfishSetup(key []byte, cost uint32, salt []byte) (*blowfish.Cipher, error) {

	// C implementations expand the key including its terminating NUL. Dropping this byte
	// produces output that matches no other bcrypt.
	ckey := append(key[:len(key):len(key)], 0)

	cipher, err := blowfish.NewSaltedCipher(ckey, salt)
	if err != nil {
		return nil, err
	}

	for i, rounds := uint64(0), uint64(1)<<cost; i < rounds; i++ {
		blowfish.ExpandKey(ckey, cipher)
		blowfish.ExpandKey(salt, cipher)
	}

	return cipher, nil

}

// there are two case one we directly pass the PEM encoded private key
// or the other is we pass the base64 decoded or encoded private key
// Supported formats are PKCS#1, PKCS#8
func parseAnyPrivateKey(keyBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(keyBytes)
	if block != nil {
		// PEM format
		switch block.Type {
		case "RSA PRIVATE KEY":
			key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("failed to parse PKCS#1 private key: %w", err)
			}
			return key, nil
		case "PRIVATE KEY":
			key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("failed to parse PKCS#8 private key: %w", err)
			}
			rsaKey, ok := key.(*rsa.PrivateKey)
			if !ok {
				return nil, fmt.Errorf("not an RSA private key")
			}
			return rsaKey, nil
		default:
			return nil, fmt.Errorf("unsupported key type: %s", block.Type)
		}
	} else {
		// Raw DER format (base64-decoded)
		// try one by one
		// PKSCS8
		anyKey, err := x509.ParsePKCS8PrivateKey(keyBytes)
		if err == nil {
			rsaKey, ok := anyKey.(*rsa.PrivateKey)
			if ok {
				return rsaKey, nil
			}
		}
		// PKSCS1
		key, err := x509.ParsePKCS1PrivateKey(keyBytes)
		if err == nil {
			return key, nil
		}
		return nil, fmt.Errorf("not a valid RSA private key or unsupported format")
	}
}
