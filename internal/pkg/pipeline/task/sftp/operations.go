package sftp

import (
	"bytes"
	"fmt"
	"io"
	pathpkg "path"
	"strings"

	"github.com/bmatcuk/doublestar"
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

		file, err := s.Path.Get(rc)
		if err != nil {
			return err
		}

		if err := s.uploadOne(client, file, rc.Data); err != nil {
			return err
		}
	}

	return nil

}

func (s *sftp) uploadOne(client *pkgsftp.Client, file string, data []byte) error {

	return s.retry(fmt.Sprintf(`upload %s`, file), func() error {

		if dir := pathpkg.Dir(file); dir != `` && dir != `.` {
			if err := client.MkdirAll(dir); err != nil {
				return fmt.Errorf(`creating remote dir %q: %w`, dir, err)
			}
		}

		f, err := client.Create(file)
		if err != nil {
			return fmt.Errorf(`creating remote file %q: %w`, file, err)
		}

		if _, err := io.Copy(f, bytes.NewReader(data)); err != nil {
			f.Close()
			return fmt.Errorf(`writing remote file %q: %w`, file, err)
		}

		// Check Close: for SFTP writes the final flush happens here and may be the
		// only place a late failure (e.g. server out of space) surfaces.
		if err := f.Close(); err != nil {
			return fmt.Errorf(`closing remote file %q: %w`, file, err)
		}

		return nil

	})

}

// download (source): read file(s) at Path (a single file or a glob) and emit
// one record per file. The base name is stored in the record context so a
// downstream task can name what it writes (mirrors file.readFile).
func (s *sftp) download(client *pkgsftp.Client, output chan<- *record.Record) error {

	path, err := s.Path.Get(nil)
	if err != nil {
		return err
	}

	paths, err := s.parse(client, path)
	if err != nil {
		return err
	}

	for _, p := range paths {
		data, err := s.downloadOne(client, p)
		if err != nil {
			return err
		}

		rc := &record.Record{Context: ctx}
		rc.SetContextValue(string(task.CtxKeyFileNameWrite), textutil.SlugifyFileName(pathpkg.Base(p)))
		rc.SetContextValue(string(task.CtxKeyFilePathWrite), textutil.SlugifyFileName(p))
		s.SendData(rc.Context, data, output)
	}

	return nil

}

// parse turns Path into the list of files to download.
// A glob is matched with doublestar by walking the static base directory and matching
// each file against the pattern; a plain path matches itself. Matching no files
// is an error — the named file is missing, or the glob matched nothing.
func (s *sftp) parse(client *pkgsftp.Client, path string) ([]string, error) {

	var matches []string
	walker := client.Walk(globBase(path))
	for walker.Step() {
		if err := walker.Err(); err != nil {
			return nil, fmt.Errorf(`walking %q: %w`, path, err)
		}
		if walker.Stat().IsDir() {
			continue
		}
		ok, err := doublestar.Match(path, walker.Path())
		if err != nil {
			return nil, fmt.Errorf(`bad glob %q: %w`, path, err)
		}
		if ok {
			matches = append(matches, walker.Path())
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf(`no files found at %q`, path)
	}

	return matches, nil

}

func (s *sftp) downloadOne(client *pkgsftp.Client, file string) ([]byte, error) {

	var data []byte

	err := s.retry(fmt.Sprintf(`download %s`, file), func() error {
		f, err := client.Open(file)
		if err != nil {
			return fmt.Errorf(`opening remote file %q: %w`, file, err)
		}
		defer f.Close()

		b, err := io.ReadAll(f)
		if err != nil {
			return fmt.Errorf(`reading remote file %q: %w`, file, err)
		}
		data = b
		return nil
	})

	return data, err

}

// globBase returns the longest leading directory of pattern with no glob
// metacharacter — the point from which to start walking.
func globBase(pattern string) string {
	i := strings.IndexAny(pattern, `*?[{`)
	if i < 0 {
		return pattern
	}
	dir := pattern[:i]
	switch j := strings.LastIndex(dir, `/`); {
	case j < 0:
		return `.`
	case j == 0:
		return `/`
	default:
		return dir[:j]
	}
}
