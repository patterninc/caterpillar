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

// upload (sink): write each incoming record's data to Path, used as-is per
// record. To name files from the source, template Path with a context value —
// e.g. {{ context "CATERPILLAR_FILE_NAME_WRITE" }} for a file source, or
// {{ context "CATERPILLAR_ARCHIVE_FILE_NAME_WRITE" }} for an archive source.
func (s *sftp) upload(client *pkgsftp.Client, input <-chan *record.Record) error {

	for {
		rc, ok := s.GetRecord(input)
		if !ok {
			break
		}

		remoteFile, err := s.Path.Get(rc)
		if err != nil {
			return err
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
			f.Close()
			return fmt.Errorf(`writing remote file %q: %w`, remoteFile, err)
		}

		// Check Close: for SFTP writes the final flush happens here and may be the
		// only place a late failure (e.g. server out of space) surfaces.
		if err := f.Close(); err != nil {
			return fmt.Errorf(`closing remote file %q: %w`, remoteFile, err)
		}

		return nil

	})

}

// download (source): read file(s) at Path (a file, glob, or directory) and emit
// one record per file. The base name is stored in the record context so a
// downstream task can name what it writes (mirrors file.readFile).
func (s *sftp) download(client *pkgsftp.Client, output chan<- *record.Record) error {

	remotePath, err := s.Path.Get(nil)
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

func containsGlob(p string) bool {
	return strings.ContainsAny(p, `*?[`)
}
