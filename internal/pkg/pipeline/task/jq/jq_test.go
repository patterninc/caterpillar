package jq

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

func TestJQTransform(t *testing.T) {

	tests := []struct {
		name     string
		spec     string
		input    string
		expected []string
	}{
		{
			name:     "Object projection",
			spec:     "path: '{id: .id}'",
			input:    `{"id":1,"skip":true}`,
			expected: []string{`{"id":1}`},
		},
		{
			name:     "Array without explode stays one record",
			spec:     "path: '.items'",
			input:    `{"items":[1,2,3]}`,
			expected: []string{`[1,2,3]`},
		},
		{
			name:     "Explode an array",
			spec:     "path: '.items'\nexplode: true",
			input:    `{"items":[1,{"a":2},"x"]}`,
			expected: []string{`1`, `{"a":2}`, `"x"`},
		},
		{
			name:     "Explode a multi-value stream",
			spec:     "path: '.items[]'\nexplode: true",
			input:    `{"items":["a","b"]}`,
			expected: []string{`"a"`, `"b"`},
		},
		{
			name:     "Explode on a scalar is a no-op",
			spec:     "path: '.n'\nexplode: true",
			input:    `{"n":5}`,
			expected: []string{`5`},
		},
		{
			name:     "Raw string output",
			spec:     "path: '.name'\nas_raw: true",
			input:    `{"name":"Ann"}`,
			expected: []string{`Ann`},
		},
		{
			name:     "Raw exploded output",
			spec:     "path: '.names'\nexplode: true\nas_raw: true",
			input:    `{"names":["Ann","Bob"]}`,
			expected: []string{`Ann`, `Bob`},
		},
		{
			name:     "Null result is dropped",
			spec:     "path: '.missing'",
			input:    `{}`,
			expected: []string{},
		},
		{
			name:     "select filters records",
			spec:     "path: 'select(.keep)'",
			input:    `{"keep":false}`,
			expected: []string{},
		},
		{
			name:     "Raw text input",
			spec:     "path: '{name: .}'",
			input:    `plain`,
			expected: []string{`{"name":"plain"}`},
		},
		{
			name:     "Custom hash function",
			spec:     "path: '. | sha256'",
			input:    `abc`,
			expected: []string{`"ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emitted, err := runTestTask(t, newTestTask(t, "name: q\n"+tt.spec), []byte(tt.input))
			require.NoError(t, err)
			assert.Equal(t, tt.expected, dataOf(emitted))
		})
	}

}

func TestJQErrors(t *testing.T) {

	const raise = `if . == "bad" then error("boom") else . end`

	t.Run("ignored by default: the bad record is skipped", func(t *testing.T) {
		tk := newTestTask(t, "name: q\npath: '"+raise+"'")
		emitted, err := runTestTask(t, tk, []byte("ok1"), []byte("bad"), []byte("ok2"))
		require.NoError(t, err)
		assert.Equal(t, []string{`"ok1"`, `"ok2"`}, dataOf(emitted))
	})

	t.Run("not ignored: the task stops", func(t *testing.T) {
		tk := newTestTask(t, "name: q\nignore_error: false\npath: '"+raise+"'")
		emitted, err := runTestTask(t, tk, []byte("ok1"), []byte("bad"), []byte("ok2"))
		assert.ErrorContains(t, err, "boom")
		assert.Equal(t, []string{`"ok1"`}, dataOf(emitted))
	})

	t.Run("invalid expression is rejected at run time", func(t *testing.T) {
		tk := newTestTask(t, "name: q\npath: '[.a'")
		_, err := runTestTask(t, tk, []byte(`{}`))
		assert.Error(t, err)
	})

}

func TestJQPathUsesRecordContext(t *testing.T) {

	tk := newTestTask(t, "name: q\npath: '. + {index: ___CATERPILLAR___CONTEXT___node_index___}'")

	r := &record.Record{Data: []byte(`{"a":1}`), Context: context.Background()}
	r.SetContextValue("node_index", "3")

	in := make(chan *record.Record, 1)
	in <- r
	close(in)
	out := make(chan *record.Record, 1)

	require.NoError(t, tk.Run(in, out))
	assert.JSONEq(t, `{"a":1,"index":3}`, string((<-out).Data))

}

func TestJQWithoutChannels(t *testing.T) {

	tk := newTestTask(t, "name: q\npath: .")

	assert.NoError(t, tk.Run(nil, nil))

	in := make(chan *record.Record, 1)
	in <- &record.Record{Data: []byte(`{}`), Context: context.Background()}
	close(in)
	assert.NoError(t, tk.Run(in, nil))

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
