package file

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

func newFileTask(t *testing.T, spec string) task.Task {

	t.Helper()

	tk, err := New()
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal([]byte(spec), tk))
	require.NoError(t, tk.Init())

	return tk

}

func readAll(t *testing.T, tk task.Task) ([]*record.Record, error) {

	t.Helper()

	out := make(chan *record.Record, 64)
	err := tk.Run(nil, out)
	close(out)

	var emitted []*record.Record
	for r := range out {
		emitted = append(emitted, r)
	}
	sort.Slice(emitted, func(i, j int) bool { return string(emitted[i].Data) < string(emitted[j].Data) })

	return emitted, err

}

func writeAll(t *testing.T, tk task.Task, records ...*record.Record) error {

	t.Helper()

	in := make(chan *record.Record, len(records))
	for _, r := range records {
		in <- r
	}
	close(in)

	return tk.Run(in, nil)

}

func writeFixture(t *testing.T, path, body string) {

	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))

}

func TestFileReadSingle(t *testing.T) {

	dir := t.TempDir()
	path := filepath.Join(dir, "My Names.TXT")
	writeFixture(t, path, "a\nb\n")

	emitted, err := readAll(t, newFileTask(t, "name: f\npath: "+path))
	require.NoError(t, err)
	require.Len(t, emitted, 1)
	assert.Equal(t, "a\nb\n", string(emitted[0].Data))
	assert.Equal(t, "f", emitted[0].Origin)

	name, ok := emitted[0].GetContextValue(string(task.CtxKeyFileNameWrite))
	require.True(t, ok)
	assert.Equal(t, "my_names.txt", name)

	slugPath, ok := emitted[0].GetContextValue(string(task.CtxKeyFilePathWrite))
	require.True(t, ok)
	assert.Equal(t, "my_names.txt", filepath.Base(slugPath))

}

func TestFileReadGlob(t *testing.T) {

	dir := t.TempDir()
	writeFixture(t, filepath.Join(dir, "a", "one.txt"), "1")
	writeFixture(t, filepath.Join(dir, "b", "c", "two.txt"), "2")
	writeFixture(t, filepath.Join(dir, "b", "skip.csv"), "x")

	emitted, err := readAll(t, newFileTask(t, "name: f\ntask_concurrency: 2\npath: "+dir+"/**/*.txt"))
	require.NoError(t, err)
	assert.Equal(t, []string{"1", "2"}, dataOf(emitted))

}

func TestFileReadErrors(t *testing.T) {

	t.Run("no matches", func(t *testing.T) {
		_, err := readAll(t, newFileTask(t, "name: f\npath: "+filepath.Join(t.TempDir(), "*.none")))
		assert.ErrorContains(t, err, "no files found")
	})

	t.Run("unknown scheme", func(t *testing.T) {
		_, err := readAll(t, newFileTask(t, "name: f\npath: ftp://host/x"))
		assert.ErrorContains(t, err, "unknown scheme: ftp")
	})

	t.Run("input and output together", func(t *testing.T) {
		tk := newFileTask(t, "name: f")
		assert.ErrorIs(t, tk.Run(make(chan *record.Record), make(chan *record.Record)), task.ErrPresentInputOutput)
	})

	t.Run("invalid storage class", func(t *testing.T) {
		tk := newFileTask(t, "name: f\nstorage_class: COLD_AS_ICE")
		assert.ErrorContains(t, tk.Run(nil, make(chan *record.Record)), "storage_class")
	})

}

func TestFileWrite(t *testing.T) {

	dir := t.TempDir()
	tk := newFileTask(t, "name: w\npath: "+dir+"/out/___CATERPILLAR___CONTEXT___name___.json")

	records := make([]*record.Record, 0, 2)
	for _, name := range []string{"first", "second"} {
		r := &record.Record{Data: []byte(name + "-data"), Context: context.Background()}
		r.SetContextValue("name", `"`+name+`"`)
		records = append(records, r)
	}

	require.NoError(t, writeAll(t, tk, records...))

	for _, name := range []string{"first", "second"} {
		got, err := os.ReadFile(filepath.Join(dir, "out", name+".json"))
		require.NoError(t, err)
		assert.Equal(t, name+"-data", string(got))
	}
	assert.NoFileExists(t, filepath.Join(dir, "out", "_SUCCESS"))

}

func TestFileWriteSuccessFile(t *testing.T) {

	dir := t.TempDir()

	t.Run("default name", func(t *testing.T) {
		tk := newFileTask(t, "name: w\nsuccess_file: true\npath: "+dir+"/a/data.txt")
		require.NoError(t, writeAll(t, tk, &record.Record{Data: []byte("x"), Context: context.Background()}))
		assert.FileExists(t, filepath.Join(dir, "a", "_SUCCESS"))
	})

	t.Run("custom name", func(t *testing.T) {
		tk := newFileTask(t, "name: w\nsuccess_file: true\nsuccess_file_name: DONE\npath: "+dir+"/b/data.txt")
		require.NoError(t, writeAll(t, tk, &record.Record{Data: []byte("x"), Context: context.Background()}))
		assert.FileExists(t, filepath.Join(dir, "b", "DONE"))
	})

}

func TestFileWriteAppendsArchiveFileName(t *testing.T) {

	dir := t.TempDir()
	tk := newFileTask(t, "name: w\npath: "+dir+"/unpacked/")

	r := &record.Record{Data: []byte("inner"), Context: context.Background()}
	r.SetContextValue(string(task.CtxKeyArchiveFileNameWrite), `nested\inner.txt`)
	require.NoError(t, writeAll(t, tk, r))

	got, err := os.ReadFile(filepath.Join(dir, "unpacked", "nested", "inner.txt"))
	require.NoError(t, err)
	assert.Equal(t, "inner", string(got))

}

func TestFileWriteErrors(t *testing.T) {

	t.Run("missing context key", func(t *testing.T) {
		tk := newFileTask(t, "name: w\npath: "+t.TempDir()+"/___CATERPILLAR___CONTEXT___missing___")
		err := writeAll(t, tk, &record.Record{Data: []byte("x"), Context: context.Background()})
		assert.ErrorContains(t, err, "context keys were not set: missing")
	})

	t.Run("unknown scheme", func(t *testing.T) {
		tk := newFileTask(t, "name: w\npath: gs://bucket/x")
		err := writeAll(t, tk, &record.Record{Data: []byte("x"), Context: context.Background()})
		assert.ErrorContains(t, err, "unknown scheme: gs")
	})

}

func TestGetPathIndex(t *testing.T) {

	assert.Equal(t, 0, getPathIndex("/tmp/x"))
	assert.Equal(t, len("file://"), getPathIndex("file:///tmp/x"))
	assert.Equal(t, 0, getPathIndex("s3://file://x"))

}

// The file:// prefix is stripped when opening or creating a file, but not when
// globbing or creating the parent directory, so file:// paths fail to read and
// write into a relative "file:" directory. Unskip once both paths strip it.
func TestFileSchemePrefix(t *testing.T) {

	t.Skip("known bug: file:// prefix is not stripped in localReader.parse or writeLocalFile's MkdirAll (internal/pkg/pipeline/task/file/local.go:39,60)")

	dir := t.TempDir()
	writeFixture(t, filepath.Join(dir, "in.txt"), "x")

	emitted, err := readAll(t, newFileTask(t, "name: f\npath: file://"+dir+"/in.txt"))
	require.NoError(t, err)
	assert.Equal(t, []string{"x"}, dataOf(emitted))

	tk := newFileTask(t, "name: w\npath: file://"+dir+"/new/out.txt")
	require.NoError(t, writeAll(t, tk, &record.Record{Data: []byte("y"), Context: context.Background()}))
	assert.FileExists(t, filepath.Join(dir, "new", "out.txt"))

}

func dataOf(records []*record.Record) []string {

	result := make([]string, 0, len(records))
	for _, r := range records {
		result = append(result, string(r.Data))
	}

	return result

}
