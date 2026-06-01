package sftp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	pkgsftp "github.com/pkg/sftp"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
	"github.com/patterninc/caterpillar/internal/pkg/textutil"
)

// upload (sink): write each incoming record's data to the SFTP server. The
// destination is RemotePath; when RemotePath is a directory we append the
// upstream filename carried in the record context (the same key the file task
// sets), so `file (read s3://...) -> sftp (upload)` composes with no glue.
func (s *sftp) upload(client *pkgsftp.Client, input <-chan *record.Record, output chan<- *record.Record) error {

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
		if filename, found := rc.GetContextValue(string(task.CtxKeyFileNameWrite)); found && filename != `` && s.isDirLike(client, remotePath) {
			remoteFile = path.Join(remotePath, filename)
		}

		if err := s.uploadOne(client, remoteFile, rc.Data); err != nil {
			return err
		}

		if output != nil {
			s.SendRecord(rc, output)
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
		// server out of space) surfaces. Deferring and ignoring it could report
		// success for an incomplete upload.
		if err := f.Close(); err != nil {
			return fmt.Errorf(`closing remote file %q: %w`, remoteFile, err)
		}

		return nil

	})

}

// download (source): read file(s) from RemotePath and emit one record per file.
// RemotePath may be a single file, a glob, or a directory. The basename is
// stored in the record context so a downstream file task can name the object
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

// dirEntry is the JSON shape emitted by the list operation, one per record, so
// downstream jq/file tasks can act on directory contents.
type dirEntry struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
	IsDir   bool   `json:"is_dir"`
}

// list (source): emit one record per entry in the RemotePath directory.
func (s *sftp) list(client *pkgsftp.Client, output chan<- *record.Record) error {

	remotePath, err := s.RemotePath.Get(nil)
	if err != nil {
		return err
	}

	var entries []os.FileInfo
	err = s.retry(fmt.Sprintf(`list %s`, remotePath), func() error {
		e, err := client.ReadDir(remotePath)
		if err != nil {
			return fmt.Errorf(`reading remote dir %q: %w`, remotePath, err)
		}
		entries = e
		return nil
	})
	if err != nil {
		return err
	}

	for _, e := range entries {
		data, err := json.Marshal(dirEntry{
			Name:    e.Name(),
			Size:    e.Size(),
			ModTime: e.ModTime().UTC().Format(time.RFC3339),
			IsDir:   e.IsDir(),
		})
		if err != nil {
			return fmt.Errorf(`marshaling dir entry %q: %w`, e.Name(), err)
		}
		s.SendData(ctx, data, output)
	}

	return nil

}

// move/rename: rename RemotePath to DestinationPath. With an input it runs once
// per record (paths may be templated against the record); otherwise it is a
// single config-driven action.
func (s *sftp) move(client *pkgsftp.Client, input <-chan *record.Record, output chan<- *record.Record) error {
	return s.perRecordAction(client, input, output, s.moveOne)
}

func (s *sftp) moveOne(client *pkgsftp.Client, rc *record.Record) error {

	src, err := s.RemotePath.Get(rc)
	if err != nil {
		return err
	}
	dst, err := s.DestinationPath.Get(rc)
	if err != nil {
		return err
	}
	if src == `` || dst == `` {
		return fmt.Errorf(`operation %q requires both remote_path (source) and destination_path`, opMove)
	}

	return s.retry(fmt.Sprintf(`move %s -> %s`, src, dst), func() error {
		if dir := path.Dir(dst); dir != `` && dir != `.` {
			if err := client.MkdirAll(dir); err != nil {
				return fmt.Errorf(`creating remote dir %q: %w`, dir, err)
			}
		}
		if err := client.Rename(src, dst); err != nil {
			return fmt.Errorf(`renaming %q to %q: %w`, src, dst, err)
		}
		return nil
	})

}

// remove (delete): delete RemotePath. Like move, it runs per record when given
// an input, otherwise once from config.
func (s *sftp) remove(client *pkgsftp.Client, input <-chan *record.Record, output chan<- *record.Record) error {
	return s.perRecordAction(client, input, output, s.removeOne)
}

func (s *sftp) removeOne(client *pkgsftp.Client, rc *record.Record) error {

	target, err := s.RemotePath.Get(rc)
	if err != nil {
		return err
	}
	if target == `` {
		return fmt.Errorf(`operation %q requires remote_path`, opDelete)
	}

	return s.retry(fmt.Sprintf(`delete %s`, target), func() error {
		if err := client.Remove(target); err != nil {
			return fmt.Errorf(`deleting %q: %w`, target, err)
		}
		return nil
	})

}

// perRecordAction is shared by move and delete: when an input channel is
// present it applies action to each record (and passes the record through if
// an output is set); otherwise it performs the action once from static config.
func (s *sftp) perRecordAction(client *pkgsftp.Client, input <-chan *record.Record, output chan<- *record.Record, action func(*pkgsftp.Client, *record.Record) error) error {

	if input != nil {
		for {
			rc, ok := s.GetRecord(input)
			if !ok {
				break
			}
			if err := action(client, rc); err != nil {
				return err
			}
			if output != nil {
				s.SendRecord(rc, output)
			}
		}
		return nil
	}

	return action(client, nil)

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
