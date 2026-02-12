package task

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"sync"

	"github.com/patterninc/caterpillar/internal/pkg/jq"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

const (
	ErrUnsupportedFieldValue = `invalid value for field %s: %s`

	MetaKeyFileNameWrite        = "CATERPILLAR_FILE_NAME_WRITE"
	MetaKeyArchiveFileNameWrite = "CATERPILLAR_ARCHIVE_FILE_NAME_WRITE"
)

var (
	ErrNilInput           = fmt.Errorf(`input channel must not be nil`)
	ErrPresentInput       = fmt.Errorf(`input channel must be nil`)
	ErrNilOutput          = fmt.Errorf(`output channel must not be nil`)
	ErrPresentInputOutput = fmt.Errorf(`either input or output must be set, not both`)
)

var byteBufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

type Task interface {
	Run(ctx context.Context, input <-chan *record.Record, output chan<- *record.Record) error
	GetName() string
	GetFailOnError() bool
	GetTaskConcurrency() int
	Init() error // Called once after unmarshaling, before pipeline execution
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

func (b *Base) Run(ctx context.Context, input <-chan *record.Record, output chan<- *record.Record) error {

	for r := range input {
		b.SendRecord(r, output)
	}

	return nil

}

func (b *Base) SendData(meta map[string]string, data []byte, output chan<- *record.Record) /* we should return error here */ {

	b.Lock()
	defer b.Unlock()

	b.recordIndex++

	record := &record.Record{
		ID:     b.recordIndex,
		Origin: b.Name,
		Data:   data,
	}

	// Copy meta map if provided
	if meta != nil {
		record.Meta = make(map[string]string, len(meta))
		maps.Copy(record.Meta, meta)
	}

	b.SendRecord(record, output)

}

func (b *Base) SendRecord(r *record.Record, output chan<- *record.Record) /* we should return error here */ {

	if output == nil {
		return
	}

	defer func() {
		output <- r
	}()

	// get a buffer from the pool
	buf := byteBufferPool.Get().(*bytes.Buffer)
	defer byteBufferPool.Put(buf)
	buf.Reset()

	// before we set context, let's serialize the whole record
	if err := json.NewEncoder(buf).Encode(r); err != nil {
		fmt.Println(`ERROR (marshal):`, err)
		return
	}

	data := buf.Bytes()

	// get another buffer for the query results
	ctxBuf := byteBufferPool.Get().(*bytes.Buffer)
	defer byteBufferPool.Put(ctxBuf)
	ctxBuf.Reset()

	ctxEncoder := json.NewEncoder(ctxBuf)

	// Set the context values for the record
	for name, query := range b.Context {
		queryResult, err := query.Execute(data)
		if err != nil {
			// TODO: do prom metrics / log event to syslog
			fmt.Println(`ERROR (query):`, err)
			return
		}
		// now, let's marshal it to json and set in the context...
		if err := ctxEncoder.Encode(queryResult); err != nil {
			// TODO: do prom metrics / log event to syslog
			fmt.Println(`ERROR (result):`, err)
			return
		}
		// Remove trailing newline added by json.Encoder.Encode()
		value := ctxBuf.String()
		if len(value) > 0 && value[len(value)-1] == '\n' {
			value = value[:len(value)-1]
		}
		r.SetMetaValue(name, value)
		ctxBuf.Reset()
	}

}
