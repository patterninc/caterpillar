package sftp

import (
	"fmt"

	"golang.org/x/crypto/ssh"
)

// buildAuthMethod turns the configured credentials into an ssh.AuthMethod.
// Exactly one of PrivateKey or Password must be set; we validate that here so
// a misconfigured pipeline fails at load time (Init) rather than mid-run.
//
// Security note: the credentials originate from SSM via the {{ secret }}
// template (resolved at config load). We must never include the raw key,
// passphrase, or password in an error message or log line — only the parse
// error itself, which does not echo the secret back.
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
