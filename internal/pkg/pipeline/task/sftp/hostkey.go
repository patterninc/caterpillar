package sftp

import (
	"fmt"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// buildHostKeyCallback decides how we verify the SFTP server's identity.
//
// Verifying the host key prevents man-in-the-middle attacks: without it, an
// attacker who can intercept the connection could impersonate the server and
// capture the files (and credentials) we send. We therefore require a way to
// verify the server and fail closed — if neither host_key nor known_hosts_path
// is set, we refuse to connect.
//
// Precedence: an inline host_key wins, then a known_hosts file, otherwise an
// error.
func (s *sftp) buildHostKeyCallback() (ssh.HostKeyCallback, error) {

	switch {

	case s.HostKey != ``:
		// HostKey is a single authorized-key line, e.g.
		//   "ssh-ed25519 AAAAC3Nza..." (the key portion of a known_hosts entry).
		key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(s.HostKey))
		if err != nil {
			return nil, fmt.Errorf(`parsing host_key: %w`, err)
		}
		return ssh.FixedHostKey(key), nil

	case s.KnownHostsPath != ``:
		callback, err := knownhosts.New(s.KnownHostsPath)
		if err != nil {
			return nil, fmt.Errorf(`loading known_hosts file %q: %w`, s.KnownHostsPath, err)
		}
		return callback, nil

	default:
		return nil, fmt.Errorf(`host key verification required: set host_key or known_hosts_path`)

	}

}
