package task_test

import (
	"context"
	"testing"
	"time"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/ack"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/compress"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/delay"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/echo"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/flatten"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/join"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/jq"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/replace"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/sample"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/split"
	"gopkg.in/yaml.v3"
)

const settleTimeout = 5 * time.Second

// tracked builds n records, each carrying its own Ack the way an acking source
// emits them: the ack starts with no branches and the send registers the first.
func tracked(n int, data func(i int) []byte) ([]*record.Record, []*ack.Ack) {

	records := make([]*record.Record, n)
	acks := make([]*ack.Ack, n)

	for i := range records {
		a := ack.New()
		a.AddBranch(1) // the source's own emit
		acks[i] = a
		records[i] = &record.Record{
			ID:      i + 1,
			Data:    data(i),
			Context: ack.WithContext(context.Background(), a),
		}
	}

	return records, acks

}

// newTask builds a task of the given type from YAML, exactly as a pipeline would.
func newTask[T any](t *testing.T, new func() (task.Task, error), spec string) task.Task {

	t.Helper()

	tk, err := new()
	if err != nil {
		t.Fatalf("constructing task: %v", err)
	}
	if err := yaml.Unmarshal([]byte(spec), tk); err != nil {
		t.Fatalf("unmarshalling %q: %v", spec, err)
	}
	if err := tk.Init(); err != nil {
		t.Fatalf("initialising task: %v", err)
	}

	return tk

}

// runTask feeds records through a task and collects what it emits, without
// releasing any of it: the caller decides when a downstream sink "handles" each
// emitted record, which is what makes premature settling observable.
func runTask(t *testing.T, tk task.Task, records []*record.Record, withOutput bool) ([]*record.Record, error) {

	t.Helper()

	input := make(chan *record.Record, len(records))
	for _, r := range records {
		input <- r
	}
	close(input)

	var output chan *record.Record
	if withOutput {
		output = make(chan *record.Record, 4096)
	}

	var emitted []*record.Record
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		if output == nil {
			return
		}
		for r := range output {
			emitted = append(emitted, r)
		}
	}()

	err := tk.Run(input, chanOrNil(output))
	if output != nil {
		close(output)
	}
	<-drained

	return emitted, err

}

// sinkAll plays the part of the downstream sinks: every emitted record has one
// outstanding branch, and handling it releases exactly that branch.
func sinkAll(emitted []*record.Record) {
	for _, r := range emitted {
		ack.Release(r.Context)
	}
}

// chanOrNil keeps a nil *chan* nil when widened to a send-only channel: a typed
// nil wrapped in a non-nil interface would defeat every `output == nil` check.
func chanOrNil(c chan *record.Record) chan<- *record.Record {
	if c == nil {
		return nil
	}
	return c
}

// assertSettled waits for every ack to settle, then checks the outcome. An ack
// that never settles is the failure mode this whole suite exists to catch: it
// hangs a real pipeline at shutdown.
func assertSettled(t *testing.T, acks []*ack.Ack, wantFailed bool) {

	t.Helper()

	for i, a := range acks {
		select {
		case <-a.Wait():
			if got := a.Failed(); got != wantFailed {
				t.Errorf("record %d: Failed() = %v, want %v", i, got, wantFailed)
			}
		case <-time.After(settleTimeout):
			t.Fatalf("record %d never settled: a source would wait on it forever", i)
		}
	}

}

// TestStreamingTasksSettleTheirInput covers the shapes that make up most of the
// pipeline: a task consumes a record, emits zero or more, and must settle the
// input exactly once either way.
func TestStreamingTasksSettleTheirInput(t *testing.T) {

	tests := []struct {
		name        string
		task        func(t *testing.T) task.Task
		data        func(i int) []byte
		records     int
		withOutput  bool
		wantEmitted int
	}{
		{
			name:        "echo forwards",
			task:        func(t *testing.T) task.Task { return newTask[any](t, echo.New, "name: e\ntype: echo") },
			data:        func(int) []byte { return []byte(`{"a":1}`) },
			records:     3,
			withOutput:  true,
			wantEmitted: 3,
		},
		{
			name:        "echo as a terminal sink",
			task:        func(t *testing.T) task.Task { return newTask[any](t, echo.New, "name: e\ntype: echo") },
			data:        func(int) []byte { return []byte(`{"a":1}`) },
			records:     3,
			withOutput:  false,
			wantEmitted: 0,
		},
		{
			name:        "delay forwards",
			task:        func(t *testing.T) task.Task { return newTask[any](t, delay.New, "name: d\ntype: delay\nduration: 1ms") },
			data:        func(int) []byte { return []byte(`x`) },
			records:     3,
			withOutput:  true,
			wantEmitted: 3,
		},
		{
			name:        "flatten transforms one to one",
			task:        func(t *testing.T) task.Task { return newTask[any](t, flatten.New, "name: f\ntype: flatten") },
			data:        func(int) []byte { return []byte(`{"a":{"b":1}}`) },
			records:     3,
			withOutput:  true,
			wantEmitted: 3,
		},
		{
			name: "replace transforms one to one",
			task: func(t *testing.T) task.Task {
				return newTask[any](t, replace.New, "name: r\ntype: replace\nexpression: a\nreplacement: b")
			},
			data:        func(int) []byte { return []byte(`aaa`) },
			records:     3,
			withOutput:  true,
			wantEmitted: 3,
		},
		{
			name: "replace with no output still drains",
			task: func(t *testing.T) task.Task {
				return newTask[any](t, replace.New, "name: r\ntype: replace\nexpression: a\nreplacement: b")
			},
			data:        func(int) []byte { return []byte(`aaa`) },
			records:     3,
			withOutput:  false,
			wantEmitted: 0,
		},
		{
			// the shape that used to need a declared count: four lines out of one record
			name:        "split fans out",
			task:        func(t *testing.T) task.Task { return newTask[any](t, split.New, "name: s\ntype: split") },
			data:        func(int) []byte { return []byte("1\n2\n3\n4") },
			records:     3,
			withOutput:  true,
			wantEmitted: 12,
		},
		{
			name: "jq explodes an array",
			task: func(t *testing.T) task.Task {
				return newTask[any](t, jq.New, "name: j\ntype: jq\npath: .items\nexplode: true")
			},
			data:        func(int) []byte { return []byte(`{"items":[1,2,3]}`) },
			records:     4,
			withOutput:  true,
			wantEmitted: 12,
		},
		{
			name:        "jq dropping every record",
			task:        func(t *testing.T) task.Task { return newTask[any](t, jq.New, "name: j\ntype: jq\npath: .missing") },
			data:        func(int) []byte { return []byte(`{"items":[1,2,3]}`) },
			records:     3,
			withOutput:  true,
			wantEmitted: 0,
		},
		{
			name: "compress round-trips",
			task: func(t *testing.T) task.Task {
				return newTask[any](t, compress.New, "name: c\ntype: compress\nformat: gzip")
			},
			data:        func(int) []byte { return []byte(`hello world`) },
			records:     3,
			withOutput:  true,
			wantEmitted: 3,
		},
		{
			name: "compress drops empty records",
			task: func(t *testing.T) task.Task {
				return newTask[any](t, compress.New, "name: c\ntype: compress\nformat: gzip")
			},
			data:        func(int) []byte { return nil },
			records:     3,
			withOutput:  true,
			wantEmitted: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			records, acks := tracked(tc.records, tc.data)

			emitted, err := runTask(t, tc.task(t), records, tc.withOutput)
			if err != nil {
				t.Fatalf("Run returned %v", err)
			}
			if len(emitted) != tc.wantEmitted {
				t.Errorf("emitted %d records, want %d", len(emitted), tc.wantEmitted)
			}

			// a task that emitted nothing has legitimately finished with its input,
			// so it may settle right away. One that emitted must not have.
			if tc.wantEmitted > 0 {
				assertOutstanding(t, acks)
			}

			sinkAll(emitted)

			assertSettled(t, acks, false)

		})
	}

}

// assertOutstanding checks no ack settled while its descendants are still in
// flight — the premature-acknowledgement bug, which is silent data loss.
func assertOutstanding(t *testing.T, acks []*ack.Ack) {

	t.Helper()

	for i, a := range acks {
		select {
		case <-a.Wait():
			t.Fatalf("record %d settled while its emitted records were still outstanding", i)
		default:
		}
	}

}

// TestSamplersSettleDroppedRecords: a sampler that discards a record still has to
// settle it, and one that buffers must not settle until it drains.
func TestSamplersSettleDroppedRecords(t *testing.T) {

	tests := []struct {
		name        string
		spec        string
		records     int
		wantEmitted int
	}{
		{name: "head keeps the first two", spec: "name: s\ntype: sample\nfilter: head\nlimit: 2", records: 6, wantEmitted: 2},
		{name: "nth keeps every third", spec: "name: s\ntype: sample\nfilter: nth\ndivider: 3", records: 9, wantEmitted: 3},
		{name: "tail keeps the last two", spec: "name: s\ntype: sample\nfilter: tail\nlimit: 2", records: 6, wantEmitted: 2},
		{name: "percent keeps none", spec: "name: s\ntype: sample\nfilter: percent\npercent: 0", records: 6, wantEmitted: 0},
		{name: "percent keeps all", spec: "name: s\ntype: sample\nfilter: percent\npercent: 100", records: 6, wantEmitted: 6},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			records, acks := tracked(tc.records, func(i int) []byte { return []byte{byte('a' + i)} })

			emitted, err := runTask(t, newTask[any](t, sample.New, tc.spec), records, true)
			if err != nil {
				t.Fatalf("Run returned %v", err)
			}
			if len(emitted) != tc.wantEmitted {
				t.Errorf("emitted %d, want %d", len(emitted), tc.wantEmitted)
			}

			// the sinks handle the survivors; the records the sampler discarded were
			// already settled by the sampler itself
			sinkAll(emitted)

			assertSettled(t, acks, false)

		})
	}

}

// TestJoinHoldsInputsUntilFlush is the aggregation invariant: buffered records
// must stay unsettled until the joined record they went into is handled, or a
// source acknowledges data that has not been written anywhere.
func TestJoinHoldsInputsUntilFlush(t *testing.T) {

	const records = 5

	srcs, acks := tracked(records, func(i int) []byte { return []byte{byte('a' + i)} })

	input := make(chan *record.Record, records)
	for _, r := range srcs {
		input <- r
	}
	close(input)

	output := make(chan *record.Record, 16)

	tk := newTask[any](t, join.New, "name: j\ntype: join\nnumber: 5")

	if err := tk.Run(input, output); err != nil {
		t.Fatalf("Run returned %v", err)
	}
	close(output)

	joined := make([]*record.Record, 0, 1)
	for r := range output {
		joined = append(joined, r)
	}
	if len(joined) != 1 {
		t.Fatalf("expected 1 joined record, got %d", len(joined))
	}

	// the joined record exists but nothing has handled it yet
	assertOutstanding(t, acks)

	// a sink handles it: that settles every record that went into it
	ack.Release(joined[0].Context)

	assertSettled(t, acks, false)

}

// TestJoinFailurePropagates: if the joined record can't be written, every record
// that went into it must be left for redelivery.
func TestJoinFailurePropagates(t *testing.T) {

	const records = 3

	srcs, acks := tracked(records, func(i int) []byte { return []byte{byte('a' + i)} })

	input := make(chan *record.Record, records)
	for _, r := range srcs {
		input <- r
	}
	close(input)

	output := make(chan *record.Record, 16)

	tk := newTask[any](t, join.New, "name: j\ntype: join\nnumber: 3")
	if err := tk.Run(input, output); err != nil {
		t.Fatalf("Run returned %v", err)
	}
	close(output)

	for r := range output {
		ack.Reject(r.Context)
	}

	assertSettled(t, acks, true)

}

// TestUntrackedPipelineUnaffected: with no acking source, the mechanism is inert
// and tasks behave exactly as they did before it existed.
func TestUntrackedPipelineUnaffected(t *testing.T) {

	records := make([]*record.Record, 4)
	for i := range records {
		records[i] = &record.Record{
			ID:      i + 1,
			Data:    []byte("1\n2\n3"),
			Context: context.Background(),
		}
	}

	emitted, err := runTask(t, newTask[any](t, split.New, "name: s\ntype: split"), records, true)
	if err != nil {
		t.Fatalf("Run returned %v", err)
	}
	if len(emitted) != 12 {
		t.Errorf("emitted %d records, want 12", len(emitted))
	}

}
