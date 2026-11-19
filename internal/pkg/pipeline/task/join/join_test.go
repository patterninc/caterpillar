package join

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/ack"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

func payloads(items ...string) [][]byte {

	result := make([][]byte, len(items))
	for i, s := range items {
		result[i] = []byte(s)
	}

	return result

}

func TestJoin(t *testing.T) {

	tests := []struct {
		name     string
		spec     string
		input    []string
		expected []string
	}{
		{
			name:     "No limits joins everything on close",
			spec:     "name: j",
			input:    []string{"a", "b", "c"},
			expected: []string{"a\nb\nc"},
		},
		{
			name:     "Number limit",
			spec:     "name: j\nnumber: 2\ndelimiter: '|'",
			input:    []string{"a", "b", "c", "d", "e"},
			expected: []string{"a|b", "c|d", "e"},
		},
		{
			name:     "Size limit flushes once reached",
			spec:     "name: j\nsize: 4\ndelimiter: ','",
			input:    []string{"ab", "cd", "e", "fgh", "i"},
			expected: []string{"ab,cd", "e,fgh", "i"},
		},
		{
			name:     "Number and size, whichever first",
			spec:     "name: j\nnumber: 3\nsize: 100\ndelimiter: ' '",
			input:    []string{"1", "2", "3", "4"},
			expected: []string{"1 2 3", "4"},
		},
		{
			name:     "Multi-character delimiter",
			spec:     "name: j\nnumber: 5\ndelimiter: ' ◆ '",
			input:    []string{"x", "y"},
			expected: []string{"x ◆ y"},
		},
		{
			name:     "No input emits nothing",
			spec:     "name: j",
			input:    nil,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emitted, err := runTestTask(t, newTestTask(t, tt.spec), payloads(tt.input...)...)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, dataOf(emitted))
		})
	}

}

func TestJoinRequiresBothChannels(t *testing.T) {

	tk := newTestTask(t, "name: j")
	in := make(chan *record.Record)
	out := make(chan *record.Record)

	assert.ErrorIs(t, tk.Run(nil, out), ErrIncorrectInputOutput)
	assert.ErrorIs(t, tk.Run(in, nil), ErrIncorrectInputOutput)

}

func TestJoinDurationFlushesStalledInput(t *testing.T) {

	tk := newTestTask(t, "name: j\nduration: 10ms")

	in := make(chan *record.Record)
	out := make(chan *record.Record, 4)
	done := make(chan error, 1)
	go func() { done <- tk.Run(in, out) }()

	in <- &record.Record{Data: []byte("partial"), Context: context.Background()}

	select {
	case r := <-out:
		assert.Equal(t, "partial", string(r.Data))
	case <-time.After(5 * time.Second):
		t.Fatal("duration elapsed without flushing the buffered record")
	}

	close(in)
	require.NoError(t, <-done)
	assert.Empty(t, out)

}

func TestJoinSettlesEveryInputThroughTheJoinedRecord(t *testing.T) {

	tk := newTestTask(t, "name: j\nnumber: 2")

	acks := make([]*ack.Ack, 2)
	in := make(chan *record.Record, 2)
	for i := range acks {
		acks[i] = ack.New()
		acks[i].AddBranch(1)
		in <- &record.Record{Data: []byte("x"), Context: ack.WithContext(context.Background(), acks[i])}
	}
	close(in)

	out := make(chan *record.Record, 1)
	require.NoError(t, tk.Run(in, out))

	joined := <-out
	for _, a := range acks {
		select {
		case <-a.Wait():
			t.Fatal("input settled before the joined record was handled")
		default:
		}
	}

	ack.Release(joined.Context)
	for _, a := range acks {
		select {
		case <-a.Wait():
			assert.False(t, a.Failed())
		case <-time.After(5 * time.Second):
			t.Fatal("input never settled")
		}
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
