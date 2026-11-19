package echo

import (
	"bytes"
	"context"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

// captureStdout runs fn with os.Stdout redirected and returns what it printed.
func captureStdout(t *testing.T, fn func()) string {

	t.Helper()

	r, w, err := os.Pipe()
	require.NoError(t, err)

	original := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = original }()

	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()

	require.NoError(t, w.Close())

	return <-done

}

var linePrefix = regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} - (\S*) - (.*)$`)

func TestEchoPrintsAndForwards(t *testing.T) {

	tests := []struct {
		name     string
		spec     string
		expected []string
	}{
		{
			name:     "Only data",
			spec:     "name: e\nonly_data: true",
			expected: []string{"hello", "world"},
		},
		{
			name:     "Full record",
			spec:     "name: e",
			expected: []string{`{"data":"hello","id":1}`, `{"data":"world","id":2}`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tk := newTestTask(t, tt.spec)

			var emitted []*record.Record
			var err error
			printed := captureStdout(t, func() {
				emitted, err = runTestTask(t, tk, []byte("hello"), []byte("world"))
			})
			require.NoError(t, err)
			assert.Equal(t, []string{"hello", "world"}, dataOf(emitted))

			lines := strings.Split(strings.TrimSuffix(printed, "\n"), "\n")
			require.Len(t, lines, len(tt.expected))
			for i, line := range lines {
				m := linePrefix.FindStringSubmatch(line)
				require.NotNil(t, m, "unexpected line %q", line)
				assert.Equal(t, "e", m[1])
				assert.Equal(t, tt.expected[i], m[2])
			}
		})
	}

}

func TestEchoAsSink(t *testing.T) {

	tk := newTestTask(t, "name: e\nonly_data: true")

	in := make(chan *record.Record, 1)
	in <- &record.Record{Data: []byte("x"), Context: context.Background()}
	close(in)

	printed := captureStdout(t, func() {
		require.NoError(t, tk.Run(in, nil))
	})
	assert.Contains(t, printed, " - e - x")

}

func TestEchoNilInput(t *testing.T) {

	tk := newTestTask(t, "name: e")
	assert.NoError(t, tk.Run(nil, nil))

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
