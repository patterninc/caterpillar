package task

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/ack"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

func baseFromYAML(t *testing.T, spec string) *Base {

	t.Helper()

	b := &Base{}
	require.NoError(t, yaml.Unmarshal([]byte(spec), b))

	return b

}

func TestBaseGetters(t *testing.T) {

	b := baseFromYAML(t, "name: n\ntype: echo\nfail_on_error: true\ntask_concurrency: 4")

	assert.Equal(t, "n", b.GetName())
	assert.Equal(t, "echo", b.GetType())
	assert.True(t, b.GetFailOnError())
	assert.Equal(t, 4, b.GetTaskConcurrency())
	assert.NoError(t, b.Init())

}

func TestBaseGetTaskConcurrency(t *testing.T) {

	tests := []struct {
		name        string
		concurrency int
		expected    int
	}{
		{name: "Unset defaults to one", concurrency: 0, expected: 1},
		{name: "Negative defaults to one", concurrency: -3, expected: 1},
		{name: "Positive is kept", concurrency: 8, expected: 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &Base{TaskConcurrency: tt.concurrency}
			assert.Equal(t, tt.expected, b.GetTaskConcurrency())
		})
	}

}

func TestBaseGetRecord(t *testing.T) {

	b := &Base{}

	t.Run("nil input", func(t *testing.T) {
		r, ok := b.GetRecord(nil)
		assert.Nil(t, r)
		assert.False(t, ok)
	})

	t.Run("closed input", func(t *testing.T) {
		in := make(chan *record.Record)
		close(in)
		_, ok := b.GetRecord(in)
		assert.False(t, ok)
	})

	t.Run("available record", func(t *testing.T) {
		in := make(chan *record.Record, 1)
		want := &record.Record{ID: 1}
		in <- want
		got, ok := b.GetRecord(in)
		assert.True(t, ok)
		assert.Same(t, want, got)
	})

}

func TestBaseSendData(t *testing.T) {

	b := &Base{Name: "src"}
	out := make(chan *record.Record, 3)

	for _, d := range []string{"a", "b", "c"} {
		b.SendData(context.Background(), []byte(d), out)
	}
	close(out)

	id := 0
	for r := range out {
		id++
		assert.Equal(t, id, r.ID)
		assert.Equal(t, "src", r.Origin)
	}
	assert.Equal(t, 3, id)

}

func TestBaseSendRecordNilOutput(t *testing.T) {

	a := ack.New()
	a.AddBranch(1)
	r := &record.Record{Context: ack.WithContext(context.Background(), a)}

	(&Base{}).SendRecord(r, nil)

	ack.Release(r.Context)
	select {
	case <-a.Wait():
	default:
		t.Fatal("a send to a nil output must not register a branch")
	}

}

func TestBaseSendRecordContext(t *testing.T) {

	b := baseFromYAML(t, `
name: ctx
context:
  first: '.data | fromjson | .items[0]'
  whole: '.data | fromjson'
  origin: '.origin'
`)

	out := make(chan *record.Record, 1)
	b.SendRecord(&record.Record{
		ID:      1,
		Origin:  "upstream",
		Data:    []byte(`{"items":["x","y"]}`),
		Context: context.Background(),
	}, out)
	r := <-out

	first, ok := r.GetContextValue("first")
	require.True(t, ok)
	assert.Equal(t, `"x"`, first)

	whole, ok := r.GetContextValue("whole")
	require.True(t, ok)
	assert.JSONEq(t, `{"items":["x","y"]}`, whole)

	origin, ok := r.GetContextValue("origin")
	require.True(t, ok)
	assert.Equal(t, `"upstream"`, origin)

}

func TestBaseSendRecordContextQueryErrorStillSends(t *testing.T) {

	b := baseFromYAML(t, "context:\n  bad: 'error(\"no\")'")

	out := make(chan *record.Record, 1)
	b.SendRecord(&record.Record{Data: []byte(`{}`), Context: context.Background()}, out)

	r := <-out
	_, ok := r.GetContextValue("bad")
	assert.False(t, ok)

}

func TestBaseRun(t *testing.T) {

	b := &Base{}
	in := make(chan *record.Record, 2)
	out := make(chan *record.Record, 2)

	in <- &record.Record{ID: 1, Context: context.Background()}
	in <- &record.Record{ID: 2, Context: context.Background()}
	close(in)

	require.NoError(t, b.Run(in, out))
	close(out)

	var ids []int
	for r := range out {
		ids = append(ids, r.ID)
	}
	assert.Equal(t, []int{1, 2}, ids)

}
