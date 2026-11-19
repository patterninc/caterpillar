package flatten

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

func TestFlatten(t *testing.T) {

	tests := []struct {
		name     string
		spec     string
		input    string
		expected string
	}{
		{
			name:     "Already flat",
			spec:     "name: f",
			input:    `{"a":1,"b":"x"}`,
			expected: `{"a":1,"b":"x"}`,
		},
		{
			name:     "Nested objects join keys with underscores",
			spec:     "name: f",
			input:    `{"a":{"b":{"c":1},"d":2},"e":3}`,
			expected: `{"a_b_c":1,"a_d":2,"e":3}`,
		},
		{
			name:     "Arrays are kept intact",
			spec:     "name: f",
			input:    `{"a":{"list":[1,{"x":2}]}}`,
			expected: `{"a_list":[1,{"x":2}]}`,
		},
		{
			name:     "Null values are kept",
			spec:     "name: f",
			input:    `{"a":{"b":null}}`,
			expected: `{"a_b":null}`,
		},
		{
			name:     "Empty nested object disappears",
			spec:     "name: f",
			input:    `{"a":{},"b":1}`,
			expected: `{"b":1}`,
		},
		{
			name:     "Original is included under the configured key",
			spec:     "name: f\ninclude_original: _raw",
			input:    `{"a":{"b":1}}`,
			expected: `{"a_b":1,"_raw":{"a":{"b":1}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emitted, err := runTestTask(t, newTestTask(t, tt.spec), []byte(tt.input))
			require.NoError(t, err)
			require.Len(t, emitted, 1)
			assert.JSONEq(t, tt.expected, string(emitted[0].Data))
		})
	}

}

func TestFlattenRejectsNonObjects(t *testing.T) {

	for _, input := range []string{`not json`, `[1,2]`, `"str"`} {
		t.Run(input, func(t *testing.T) {
			emitted, err := runTestTask(t, newTestTask(t, "name: f"), []byte(input))
			assert.Error(t, err)
			assert.Empty(t, emitted)
		})
	}

}

func TestFlattenStopsAtFirstBadRecord(t *testing.T) {

	emitted, err := runTestTask(t, newTestTask(t, "name: f"), []byte(`{"a":1}`), []byte(`bad`), []byte(`{"b":2}`))
	assert.Error(t, err)
	assert.Equal(t, []string{`{"a":1}`}, dataOf(emitted))

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
