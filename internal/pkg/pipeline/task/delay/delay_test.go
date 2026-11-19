package delay

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/patterninc/caterpillar/internal/pkg/duration"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

func TestDelayDefaults(t *testing.T) {

	tk := newTestTask(t, "name: d")
	assert.Equal(t, duration.Duration(100*time.Millisecond), tk.(*delay).Duration)

}

func TestDelayWaitsPerRecord(t *testing.T) {

	tk := newTestTask(t, "name: d\nduration: 5ms")

	start := time.Now()
	emitted, err := runTestTask(t, tk, []byte("a"), []byte("b"), []byte("c"))
	require.NoError(t, err)

	assert.Equal(t, []string{"a", "b", "c"}, dataOf(emitted))
	assert.GreaterOrEqual(t, time.Since(start), 15*time.Millisecond)

}

func TestDelayZeroDuration(t *testing.T) {

	tk := newTestTask(t, "name: d\nduration: 0s")

	emitted, err := runTestTask(t, tk, []byte("a"))
	require.NoError(t, err)
	assert.Equal(t, []string{"a"}, dataOf(emitted))

}

func TestDelayInvalidDuration(t *testing.T) {

	tk, err := New()
	require.NoError(t, err)
	assert.Error(t, yaml.Unmarshal([]byte("duration: later"), tk))

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
