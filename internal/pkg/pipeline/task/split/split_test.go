package split

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

func TestSplit(t *testing.T) {

	tests := []struct {
		name     string
		spec     string
		input    string
		expected []string
	}{
		{
			name:     "Default newline delimiter",
			spec:     "name: s",
			input:    "a\nb\nc",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "Single trailing delimiter is dropped",
			spec:     "name: s",
			input:    "a\nb\n",
			expected: []string{"a", "b"},
		},
		{
			name:     "Only one trailing delimiter is dropped",
			spec:     "name: s",
			input:    "a\n\n",
			expected: []string{"a", ""},
		},
		{
			name:     "Interior empty lines are kept",
			spec:     "name: s",
			input:    "a\n\nb",
			expected: []string{"a", "", "b"},
		},
		{
			name:     "Custom multi-character delimiter",
			spec:     "name: s\ndelimiter: '||'",
			input:    "x||y||z",
			expected: []string{"x", "y", "z"},
		},
		{
			name:     "No delimiter present",
			spec:     "name: s\ndelimiter: ','",
			input:    "abc",
			expected: []string{"abc"},
		},
		{
			name:     "Empty input yields one empty record",
			spec:     "name: s",
			input:    "",
			expected: []string{""},
		},
		{
			name:     "CRLF is not special",
			spec:     "name: s",
			input:    "a\r\nb",
			expected: []string{"a\r", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emitted, err := runTestTask(t, newTestTask(t, tt.spec), []byte(tt.input))
			require.NoError(t, err)
			assert.Equal(t, tt.expected, dataOf(emitted))
		})
	}

}

func TestSplitAssignsSequentialIDs(t *testing.T) {

	emitted, err := runTestTask(t, newTestTask(t, "name: s"), []byte("a\nb"), []byte("c"))
	require.NoError(t, err)

	ids := make([]int, 0, len(emitted))
	for _, r := range emitted {
		ids = append(ids, r.ID)
		assert.Equal(t, "s", r.Origin)
	}
	assert.Equal(t, []int{1, 2, 3}, ids)

}

func TestSplitPropagatesContext(t *testing.T) {

	tk := newTestTask(t, "name: s")

	r := &record.Record{Data: []byte("a\nb"), Context: context.Background()}
	r.SetContextValue("file", "names.txt")

	in := make(chan *record.Record, 1)
	in <- r
	close(in)
	out := make(chan *record.Record, 2)
	require.NoError(t, tk.Run(in, out))
	close(out)

	for got := range out {
		v, ok := got.GetContextValue("file")
		assert.True(t, ok)
		assert.Equal(t, "names.txt", v)
	}

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
