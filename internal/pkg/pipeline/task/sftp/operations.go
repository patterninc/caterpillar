package sftp

import (
	"bytes"
	"fmt"
	"io"
	"path"
	"strings"

	pkgsftp "github.com/pkg/sftp"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
	"github.com/patterninc/caterpillar/internal/pkg/textutil"
)

// upload (sink): write each incoming record's data to the server. When
// remote_path is a directory, the upstream filename (carried in the record
// context by the source task) is appended, so `file -> sftp` composes with no
// glue.
func (s *sftp) upload(client *pkgsftp.Client, input <-chan *record.Record) error {

	// Cache isDirLike per remote_path so a static (or repeated) destination is
	// stat-ed once per run rather than once per record. Local to this call so
	// concurrent workers don't share it.
	dirCache := make(map[string]bool)

	for {
		rc, ok := s.GetRecord(input)
		if !ok {
			break
		}

		remotePath, err := s.RemotePath.Get(rc)
		if err != nil {
			return err
		}

		remoteFile := remotePath
		if filename, found := rc.GetContextValue(string(task.CtxKeyFileNameWrite)); found && filename != `` && s.isDirLikeCached(client, remotePath, dirCache) {
			remoteFile = path.Join(remotePath, filename)
		}

		if err := s.uploadOne(client, remoteFile, rc.Data); err != nil {
			return err
		}
	}

	return nil

}

func (s *sftp) uploadOne(client *pkgsftp.Client, remoteFile string, data []byte) error {

	return s.retry(fmt.Sprintf(`upload %s`, remoteFile), func() error {

		if dir := path.Dir(remoteFile); dir != `` && dir != `.` {
			if err := client.MkdirAll(dir); err != nil {
				return fmt.Errorf(`creating remote dir %q: %w`, dir, err)
			}
		}

		f, err := client.Create(remoteFile)
		if err != nil {
			return fmt.Errorf(`creating remote file %q: %w`, remoteFile, err)
		}

		if _, err := io.Copy(f, bytes.NewReader(data)); err != nil {
			f.Close() // best effort; the copy error is the underlying failure
			return fmt.Errorf(`writing remote file %q: %w`, remoteFile, err)
		}

		// Check the Close error explicitly: for SFTP writes the final flush/
		// commit happens here and may be the only place a late failure (e.g.
		// server out of space) surfaces. Ignoring it could report success for
		// an incomplete upload.
		if err := f.Close(); err != nil {
			return fmt.Errorf(`closing remote file %q: %w`, remoteFile, err)
		}

		return nil

	})

}

// download (source): read file(s) from remote_path and emit one record per
// file. remote_path may be a single file, a glob, or a directory. The basename
// is stored in the record context so a downstream file task can name the object
// it writes (mirrors file.readFile).
func (s *sftp) download(client *pkgsftp.Client, output chan<- *record.Record) error {

	remotePath, err := s.RemotePath.Get(nil)
	if err != nil {
		return err
	}

	paths, err := s.resolveDownloadPaths(client, remotePath)
	if err != nil {
		return err
	}

	for _, p := range paths {
		data, err := s.downloadOne(client, p)
		if err != nil {
			return err
		}

		rc := &record.Record{Context: ctx}
		rc.SetContextValue(string(task.CtxKeyFileNameWrite), textutil.SlugifyFileName(path.Base(p)))
		s.SendData(rc.Context, data, output)
	}

	return nil

}

func (s *sftp) resolveDownloadPaths(client *pkgsftp.Client, remotePath string) ([]string, error) {

	if containsGlob(remotePath) {
		matches, err := client.Glob(remotePath)
		if err != nil {
			return nil, fmt.Errorf(`globbing %q: %w`, remotePath, err)
		}
		return matches, nil
	}

	// A directory expands to the (non-directory) files directly inside it.
	if info, err := client.Stat(remotePath); err == nil && info.IsDir() {
		entries, err := client.ReadDir(remotePath)
		if err != nil {
			return nil, fmt.Errorf(`reading remote dir %q: %w`, remotePath, err)
		}
		paths := make([]string, 0, len(entries))
		for _, e := range entries {
			if !e.IsDir() {
				paths = append(paths, path.Join(remotePath, e.Name()))
			}
		}
		return paths, nil
	}

	return []string{remotePath}, nil

}

func (s *sftp) downloadOne(client *pkgsftp.Client, remoteFile string) ([]byte, error) {

	var data []byte

	err := s.retry(fmt.Sprintf(`download %s`, remoteFile), func() error {
		f, err := client.Open(remoteFile)
		if err != nil {
			return fmt.Errorf(`opening remote file %q: %w`, remoteFile, err)
		}
		defer f.Close()

		b, err := io.ReadAll(f)
		if err != nil {
			return fmt.Errorf(`reading remote file %q: %w`, remoteFile, err)
		}
		data = b
		return nil
	})

	return data, err

}

// isDirLikeCached wraps isDirLike with a per-run cache keyed on remotePath, so
// the same destination is stat-ed at most once instead of once per record.
func (s *sftp) isDirLikeCached(client *pkgsftp.Client, remotePath string, cache map[string]bool) bool {
	if v, ok := cache[remotePath]; ok {
		return v
	}
	v := s.isDirLike(client, remotePath)
	cache[remotePath] = v
	return v
}

// isDirLike reports whether remotePath should be treated as a directory to
// place an uploaded file into: either it ends with "/" or it already exists as
// a directory on the server.
func (s *sftp) isDirLike(client *pkgsftp.Client, remotePath string) bool {

	if strings.HasSuffix(remotePath, `/`) {
		return true
	}
	if info, err := client.Stat(remotePath); err == nil && info.IsDir() {
		return true
	}
	return false

}

func containsGlob(p string) bool {
	return strings.ContainsAny(p, `*?[`)
}
