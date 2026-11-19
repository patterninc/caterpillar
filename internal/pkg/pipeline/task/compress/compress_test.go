package compress

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/golang/snappy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

func TestCompressDefaults(t *testing.T) {

	c := newTestTask(t, "name: c").(*core)
	assert.Equal(t, "gzip", c.Format)
	assert.Equal(t, "compress", c.Action)

}

func TestCompressUnknownFormat(t *testing.T) {

	tk, err := New()
	require.NoError(t, err)
	assert.ErrorContains(t, yaml.Unmarshal([]byte("format: lz4"), tk), "invalid value for field format")

}

func TestCompressRoundTrip(t *testing.T) {

	payload := []byte(strings.Repeat("caterpillar ", 100))

	for _, format := range []string{"gzip", "snappy"} {
		t.Run(format, func(t *testing.T) {
			compressed, err := runTestTask(t, newTestTask(t, "name: c\nformat: "+format), payload)
			require.NoError(t, err)
			require.Len(t, compressed, 1)
			assert.Less(t, len(compressed[0].Data), len(payload))

			restored, err := runTestTask(t, newTestTask(t, "name: d\naction: decompress\nformat: "+format), compressed[0].Data)
			require.NoError(t, err)
			require.Len(t, restored, 1)
			assert.Equal(t, payload, restored[0].Data)
		})
	}

}

func TestCompressInteroperatesWithStdlib(t *testing.T) {

	t.Run("gzip output is readable by compress/gzip", func(t *testing.T) {
		emitted, err := runTestTask(t, newTestTask(t, "name: c"), []byte("hello"))
		require.NoError(t, err)
		zr, err := gzip.NewReader(bytes.NewReader(emitted[0].Data))
		require.NoError(t, err)
		got, err := io.ReadAll(zr)
		require.NoError(t, err)
		assert.Equal(t, "hello", string(got))
	})

	t.Run("snappy framed input is decompressed", func(t *testing.T) {
		var buf bytes.Buffer
		w := snappy.NewBufferedWriter(&buf)
		_, err := w.Write([]byte("framed"))
		require.NoError(t, err)
		require.NoError(t, w.Close())

		emitted, err := runTestTask(t, newTestTask(t, "name: d\nformat: snappy\naction: decompress"), buf.Bytes())
		require.NoError(t, err)
		assert.Equal(t, []string{"framed"}, dataOf(emitted))
	})

}

func TestCompressSkipsEmptyRecords(t *testing.T) {

	emitted, err := runTestTask(t, newTestTask(t, "name: c"), []byte{}, nil)
	require.NoError(t, err)
	assert.Empty(t, emitted)

}

func TestDecompressEmptyResultIsSkipped(t *testing.T) {

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	require.NoError(t, zw.Close())

	emitted, err := runTestTask(t, newTestTask(t, "name: d\naction: decompress"), buf.Bytes())
	require.NoError(t, err)
	assert.Empty(t, emitted)

}

func TestDecompressCorruptInput(t *testing.T) {

	for _, format := range []string{"gzip", "snappy"} {
		t.Run(format, func(t *testing.T) {
			_, err := runTestTask(t, newTestTask(t, "name: d\naction: decompress\nformat: "+format), []byte("definitely not compressed"))
			assert.Error(t, err)
		})
	}

}

func TestCompressRequiresInput(t *testing.T) {

	assert.ErrorIs(t, newTestTask(t, "name: c").Run(nil, nil), task.ErrNilInput)

}

func TestCompressPropagatesContext(t *testing.T) {

	r := &record.Record{Data: []byte("x"), Context: context.Background()}
	r.SetContextValue("k", "v")

	in := make(chan *record.Record, 1)
	in <- r
	close(in)
	out := make(chan *record.Record, 1)
	require.NoError(t, newTestTask(t, "name: c").Run(in, out))

	v, ok := (<-out).GetContextValue("k")
	assert.True(t, ok)
	assert.Equal(t, "v", v)

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
