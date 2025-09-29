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

	"github.com/google/uuid"
	"github.com/itchyny/gojq"
)

type hashFunc func(string) string
type messageAuthenticationFunc func(data string, key []byte, pref []byte) string
type signFunc func(data string, privateKey []byte) (string, error)

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
				panic(fmt.Errorf("expected string for %s, got %T", name, raw))
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
				panic(fmt.Errorf("expected string for data for sign  %s, got %T", name, args[0]))
			}
			key, ok := args[1].(string)
			if !ok {
				panic(fmt.Errorf("expected string for key for sign %s, got %T", name, args[1]))
			}
			result, err := fn(data, []byte(key))
			if err != nil {
				panic(err)
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
				panic(fmt.Errorf("expected string for data %s, got %T", name, args[0]))
			}
			key, ok := args[1].(string)
			if !ok {
				panic(fmt.Errorf("expected string for key %s, got %T", name, args[0]))
			}
			var pref = []byte{}
			if len(args) == 3 {
				p, ok := args[2].(string)
				if !ok {
					panic(fmt.Errorf("expected string for pref %s, got %T", name, args[2]))
				}
				pref = []byte(p)
			}
			return fn(data, []byte(key), pref)
		})
		options = append(options, opt)
	}
	return options
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
