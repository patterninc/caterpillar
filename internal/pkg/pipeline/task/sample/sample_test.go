package sample

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

func numbered(n int) [][]byte {

	result := make([][]byte, n)
	for i := range result {
		result[i] = []byte(fmt.Sprintf("%d", i))
	}

	return result

}

func TestSampleDefaults(t *testing.T) {

	s := newTestTask(t, "name: s").(*sample)
	assert.Equal(t, "random", s.Filter)
	assert.Equal(t, 10, s.Limit)
	assert.Equal(t, 1, s.Percent)
	assert.Equal(t, 1000, s.Divider)
	assert.Equal(t, 50000, s.Size)

}

func TestSampleDeterministicFilters(t *testing.T) {

	tests := []struct {
		name     string
		spec     string
		records  int
		expected []string
	}{
		{
			name:     "head",
			spec:     "filter: head\nlimit: 3",
			records:  10,
			expected: []string{"0", "1", "2"},
		},
		{
			name:     "head with fewer records than the limit",
			spec:     "filter: head\nlimit: 5",
			records:  2,
			expected: []string{"0", "1"},
		},
		{
			name:     "tail",
			spec:     "filter: tail\nlimit: 3",
			records:  10,
			expected: []string{"7", "8", "9"},
		},
		{
			name:     "tail with fewer records than the limit",
			spec:     "filter: tail\nlimit: 5",
			records:  2,
			expected: []string{"0", "1"},
		},
		{
			name:     "tail with exactly the limit",
			spec:     "filter: tail\nlimit: 3",
			records:  3,
			expected: []string{"0", "1", "2"},
		},
		{
			name:     "nth",
			spec:     "filter: nth\ndivider: 3",
			records:  10,
			expected: []string{"0", "3", "6", "9"},
		},
		{
			name:     "percent 100 keeps everything",
			spec:     "filter: percent\npercent: 100",
			records:  5,
			expected: []string{"0", "1", "2", "3", "4"},
		},
		{
			name:     "percent 0 keeps nothing",
			spec:     "filter: percent\npercent: 0",
			records:  5,
			expected: []string{},
		},
		{
			name:     "random with no input emits nothing",
			spec:     "filter: random",
			records:  0,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emitted, err := runTestTask(t, newTestTask(t, "name: s\n"+tt.spec), numbered(tt.records)...)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, dataOf(emitted))
		})
	}

}

func TestSampleRandom(t *testing.T) {

	t.Run("draws limit records from the buffer", func(t *testing.T) {
		emitted, err := runTestTask(t, newTestTask(t, "name: s\nfilter: random\nlimit: 20\nsize: 5"), numbered(50)...)
		require.NoError(t, err)
		require.Len(t, emitted, 20)
		for _, r := range emitted {
			assert.Contains(t, []string{"0", "1", "2", "3", "4"}, string(r.Data))
		}
	})

}

func TestSampleConfigErrors(t *testing.T) {

	t.Run("unknown filter", func(t *testing.T) {
		_, err := runTestTask(t, newTestTask(t, "name: s\nfilter: bogus"), numbered(1)...)
		assert.ErrorContains(t, err, "unknown filter: bogus")
	})

	t.Run("percent out of range", func(t *testing.T) {
		_, err := runTestTask(t, newTestTask(t, "name: s\nfilter: percent\npercent: 101"), numbered(1)...)
		assert.ErrorIs(t, err, ErrOutOfRange)
	})

	t.Run("must not be first or last", func(t *testing.T) {
		tk := newTestTask(t, "name: s")
		ch := make(chan *record.Record)
		assert.ErrorIs(t, tk.Run(nil, ch), ErrInvalidConfig)
		assert.ErrorIs(t, tk.Run(ch, nil), ErrInvalidConfig)
	})

}

// A zero divider or tail limit is accepted from config and then divides by
// zero on the first record. Unskip once sample validates them.
func TestSampleZeroDividerAndLimit(t *testing.T) {

	t.Skip("known bug: nth with divider 0 and tail with limit 0 panic with divide by zero / index out of range (internal/pkg/pipeline/task/sample/nth.go:25, tail.go:30)")

	for _, spec := range []string{"filter: nth\ndivider: 0", "filter: tail\nlimit: 0"} {
		t.Run(spec, func(t *testing.T) {
			assert.NotPanics(t, func() {
				_, _ = runTestTask(t, newTestTask(t, "name: s\n"+spec), numbered(3)...)
			})
		})
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
