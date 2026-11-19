package replace

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

func TestReplace(t *testing.T) {

	tests := []struct {
		name     string
		spec     string
		input    []string
		expected []string
	}{
		{
			name:     "Capture group reference",
			spec:     "expression: ^(.*)$\nreplacement: Hello, $1!",
			input:    []string{"Ann", "Bob"},
			expected: []string{"Hello, Ann!", "Hello, Bob!"},
		},
		{
			name:     "Every match is replaced",
			spec:     "expression: a\nreplacement: b",
			input:    []string{"banana"},
			expected: []string{"bbnbnb"},
		},
		{
			name:     "No match leaves data unchanged",
			spec:     "expression: z\nreplacement: y",
			input:    []string{"abc"},
			expected: []string{"abc"},
		},
		{
			name:     "Empty replacement deletes",
			spec:     "expression: '\\s+'\nreplacement: ''",
			input:    []string{"a b\tc"},
			expected: []string{"abc"},
		},
		{
			name:     "Wrap as JSON",
			spec:     "expression: ^(.*)$\nreplacement: '{\"name\":\"$1\"}'",
			input:    []string{"Zed"},
			expected: []string{`{"name":"Zed"}`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payloads := make([][]byte, len(tt.input))
			for i, s := range tt.input {
				payloads[i] = []byte(s)
			}
			emitted, err := runTestTask(t, newTestTask(t, tt.spec), payloads...)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, dataOf(emitted))
		})
	}

}

func TestReplaceKeepsContext(t *testing.T) {

	tk := newTestTask(t, "expression: x\nreplacement: y")

	r := &record.Record{Data: []byte("x"), Context: context.Background()}
	r.SetContextValue("k", "v")

	in := make(chan *record.Record, 1)
	in <- r
	close(in)
	out := make(chan *record.Record, 1)

	require.NoError(t, tk.Run(in, out))
	got := <-out
	v, ok := got.GetContextValue("k")
	assert.True(t, ok)
	assert.Equal(t, "v", v)

}

func TestReplaceInvalidExpression(t *testing.T) {

	_, err := runTestTask(t, newTestTask(t, "expression: '('"), []byte("x"))
	assert.Error(t, err)

}

func TestReplaceWithoutChannels(t *testing.T) {

	tk := newTestTask(t, "expression: a")

	assert.NoError(t, tk.Run(nil, nil))

	in := make(chan *record.Record, 1)
	in <- &record.Record{Data: []byte("a"), Context: context.Background()}
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
