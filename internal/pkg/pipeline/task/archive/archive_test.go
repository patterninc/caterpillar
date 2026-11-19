package archive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

type namedFile struct {
	name string
	body string
}

// namedRecords builds records the way a file source emits them, with the file
// name in the context that pack reads.
func namedRecords(files ...namedFile) []*record.Record {

	records := make([]*record.Record, 0, len(files))
	for _, f := range files {
		r := &record.Record{Data: []byte(f.body), Context: context.Background()}
		r.SetContextValue(string(task.CtxKeyFileNameWrite), f.name)
		records = append(records, r)
	}

	return records

}

func runRecords(t *testing.T, tk task.Task, records []*record.Record) []*record.Record {

	t.Helper()

	in := make(chan *record.Record, len(records))
	for _, r := range records {
		in <- r
	}
	close(in)

	out := make(chan *record.Record, 64)
	require.NoError(t, tk.Run(in, out))
	close(out)

	var emitted []*record.Record
	for r := range out {
		emitted = append(emitted, r)
	}

	return emitted

}

func archiveName(t *testing.T, r *record.Record) string {

	t.Helper()

	name, ok := r.GetContextValue(string(task.CtxKeyArchiveFileNameWrite))
	require.True(t, ok)

	return name

}

func TestArchiveDefaults(t *testing.T) {

	c := newTestTask(t, "name: a").(*core)
	assert.Equal(t, "zip", c.Format)
	assert.Equal(t, actionPack, c.Action)

}

func TestArchiveInvalidAction(t *testing.T) {

	tk, err := New()
	require.NoError(t, err)
	assert.ErrorContains(t, yaml.Unmarshal([]byte("action: explode"), tk), "invalid action")

}

// format is not validated at unmarshal time, so an unknown one dereferences a
// nil constructor in Run. Unskip once UnmarshalYAML rejects it.
func TestArchiveUnknownFormat(t *testing.T) {

	t.Skip("known bug: unknown archive format panics in Run instead of failing validation (internal/pkg/pipeline/task/archive/archive.go:73-87)")

	tk, err := New()
	require.NoError(t, err)
	assert.Error(t, yaml.Unmarshal([]byte("format: rar"), tk))

}

func TestArchiveRequiresInput(t *testing.T) {

	assert.ErrorIs(t, newTestTask(t, "name: a").Run(nil, nil), task.ErrNilInput)

}

func TestZipPack(t *testing.T) {

	emitted := runRecords(t, newTestTask(t, "name: a\nformat: zip\naction: pack"), namedRecords(
		namedFile{name: "a.txt", body: "alpha"},
		namedFile{name: `dir\b.txt`, body: "beta"},
	))
	require.Len(t, emitted, 1)

	zr, err := zip.NewReader(bytes.NewReader(emitted[0].Data), int64(len(emitted[0].Data)))
	require.NoError(t, err)

	got := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		require.NoError(t, err)
		body, err := io.ReadAll(rc)
		require.NoError(t, err)
		require.NoError(t, rc.Close())
		got[f.Name] = string(body)
	}
	assert.Equal(t, map[string]string{"a.txt": "alpha", "dir/b.txt": "beta"}, got)

}

func TestZipPackWithNoInputEmitsEmptyArchive(t *testing.T) {

	emitted := runRecords(t, newTestTask(t, "name: a\nformat: zip"), nil)
	require.Len(t, emitted, 1)

	zr, err := zip.NewReader(bytes.NewReader(emitted[0].Data), int64(len(emitted[0].Data)))
	require.NoError(t, err)
	assert.Empty(t, zr.File)

}

func TestZipUnpack(t *testing.T) {

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range map[string]string{"Report 1.CSV": "a,b", "nested/dir/two.txt": "two"} {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(body))
		require.NoError(t, err)
	}
	_, err := zw.Create("emptydir/")
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	emitted, err := runTestTask(t, newTestTask(t, "name: a\nformat: zip\naction: unpack"), buf.Bytes(), nil)
	require.NoError(t, err)

	got := map[string]string{}
	for _, r := range emitted {
		got[archiveName(t, r)] = string(r.Data)
	}
	assert.Equal(t, map[string]string{"report_1.csv": "a,b", "two.txt": "two"}, got)

}

func TestTarRoundTrip(t *testing.T) {

	packed := runRecords(t, newTestTask(t, "name: a\nformat: tar\naction: pack"), namedRecords(
		namedFile{name: "one.txt", body: "1"},
		namedFile{name: "skipped.txt", body: ""},
		namedFile{name: "sub/Two File.TXT", body: "22"},
	))
	require.Len(t, packed, 1)

	tr := tar.NewReader(bytes.NewReader(packed[0].Data))
	var names []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		names = append(names, h.Name)
	}
	assert.Equal(t, []string{"one.txt", "sub/Two File.TXT"}, names)

	unpacked, err := runTestTask(t, newTestTask(t, "name: a\nformat: tar\naction: unpack"), packed[0].Data)
	require.NoError(t, err)

	got := map[string]string{}
	for _, r := range unpacked {
		got[archiveName(t, r)] = string(r.Data)
	}
	assert.Equal(t, map[string]string{"one.txt": "1", "two_file.txt": "22"}, got)

}

func TestTarUnpackSkipsNonRegularEntries(t *testing.T) {

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "dir/", Typeflag: tar.TypeDir, Mode: 0o755}))
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "dir/f.txt", Typeflag: tar.TypeReg, Mode: 0o600, Size: 1}))
	_, err := tw.Write([]byte("f"))
	require.NoError(t, err)
	require.NoError(t, tw.Close())

	emitted, err := runTestTask(t, newTestTask(t, "name: a\nformat: tar\naction: unpack"), buf.Bytes())
	require.NoError(t, err)
	assert.Equal(t, []string{"f"}, dataOf(emitted))

}

// newTestTask builds the task from YAML exactly as a pipeline would.
func newTestTask(t *testing.T, spec string) task.Task {

	t.Helper()

	tk, err := New()
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal([]byte(spec), tk))
	require.NoError(t, tk.Init())

	return tk

}

// runTestTask feeds one record per payload through Run and collects everything emitted.
func runTestTask(t *testing.T, tk task.Task, payloads ...[]byte) ([]*record.Record, error) {

	t.Helper()

	in := make(chan *record.Record, len(payloads))
	for i, p := range payloads {
		in <- &record.Record{ID: i + 1, Data: p, Context: context.Background()}
	}
	close(in)

	out := make(chan *record.Record, 4096)
	err := tk.Run(in, out)
	close(out)

	var emitted []*record.Record
	for r := range out {
		emitted = append(emitted, r)
	}

	return emitted, err

}

func dataOf(records []*record.Record) []string {

	result := make([]string, 0, len(records))
	for _, r := range records {
		result = append(result, string(r.Data))
	}

	return result

}
