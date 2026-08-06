package task

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/patterninc/caterpillar/internal/pkg/jq"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/ack"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

const (
	ErrUnsupportedFieldValue = `invalid value for field %s: %s`
)

var (
	ErrNilInput           = fmt.Errorf(`input channel must not be nil`)
	ErrPresentInput       = fmt.Errorf(`input channel must be nil`)
	ErrNilOutput          = fmt.Errorf(`output channel must not be nil`)
	ErrPresentInputOutput = fmt.Errorf(`either input or output must be set, not both`)
)

type contextKeyFile string

const (
	CtxKeyFileNameWrite        contextKeyFile = "CATERPILLAR_FILE_NAME_WRITE"
	CtxKeyFilePathWrite        contextKeyFile = "CATERPILLAR_FILE_PATH_WRITE"
	CtxKeyArchiveFileNameWrite contextKeyFile = "CATERPILLAR_ARCHIVE_FILE_NAME_WRITE"
)

type Task interface {
	Run(<-chan *record.Record, chan<- *record.Record) error
	GetName() string
	GetFailOnError() bool
	GetTaskConcurrency() int
	Init() error // Called once after unmarshaling, before pipeline execution
}

// Finisher is implemented by tasks with work to do only once their output
// channel has been closed. Deferred acknowledgement is the case this exists
// for: a source's acks settle only after every downstream task has drained,
// and a downstream task that emits on input close can't drain until the
// source's output channel is closed, so waiting inside Run would deadlock.
//
// The pipeline calls Finish exactly once per task, after every worker of that
// task has returned from Run and after the task's output channel is closed.
type Finisher interface {
	Finish() error
}

type Base struct {
	Name            string               `yaml:"name,omitempty" json:"name,omitempty"`
	Type            string               `yaml:"type,omitempty" json:"type,omitempty"`
	FailOnError     bool                 `yaml:"fail_on_error,omitempty" json:"fail_on_error,omitempty"`
	TaskConcurrency int                  `yaml:"task_concurrency,omitempty" json:"task_concurrency,omitempty"`
	Context         map[string]*jq.Query `yaml:"context,omitempty" json:"context,omitempty"`

	recordIndex int
	sync.RWMutex
}

func (b *Base) GetFailOnError() bool {
	return b.FailOnError
}

func (b *Base) GetName() string {
	return b.Name
}

func (b *Base) GetTaskConcurrency() int {
	if b.TaskConcurrency < 0 {
		fmt.Printf(`WARN: defaulting task_concurrency to 1 for task %s`, b.Name)
	}
	return max(1, b.TaskConcurrency)
}

// Init is called once after unmarshaling, before pipeline execution
// Default implementation does nothing. Tasks can override this for initialization.
func (b *Base) Init() error {
	return nil
}

func (b *Base) GetRecord(input <-chan *record.Record) (*record.Record, bool) {

	if input == nil {
		return nil, false
	}

	record, ok := <-input
	return record, ok

}

func (b *Base) Run(input <-chan *record.Record, output chan<- *record.Record) error {

	for r := range input {
		b.SendRecord(r, output)
	}

	return nil

}

func (b *Base) SendData(ctx context.Context, data []byte, output chan<- *record.Record) /* we should return error here */ {

	b.Lock()
	defer b.Unlock()

	b.recordIndex++

	record := &record.Record{
		ID:      b.recordIndex,
		Origin:  b.Name,
		Data:    data,
		Context: ctx,
	}

	b.SendRecord(record, output)

}

func (b *Base) SendRecord(r *record.Record, output chan<- *record.Record) /* we should return error here */ {

	if output == nil {
		// terminal task: nothing forwards r downstream, so settle its ack here
		// or a source deferring acknowledgement waits forever for it.
		ack.Drop(r.Context)
		return
	}

	defer func() {
		output <- r
	}()

	// before we set context, let's serialize the whole record
	data, err := json.Marshal(r)
	if err != nil {
		// TODO: do prom metrics / log event to syslog
		fmt.Println(`ERROR (marshal):`, err)
		return
	}
	// Set the context values for the record
	for name, query := range b.Context {
		queryResult, err := query.Execute(data)
		if err != nil {
			// TODO: do prom metrics / log event to syslog
			fmt.Println(`ERROR (query):`, err)
			return
		}
		// now, let's marshal it to json and set in the context...
		contextValueJson, err := json.Marshal(queryResult)
		if err != nil {
			// TODO: do prom metrics / log event to syslog
			fmt.Println(`ERROR (result):`, err)
			return
		}
		r.SetContextValue(name, string(contextValueJson))
	}

}
