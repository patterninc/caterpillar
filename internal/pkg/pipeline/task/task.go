package task

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/patterninc/caterpillar/internal/pkg/jq"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

const (
	timeNowFormat            = `2006-01-02 15:04:05`
	ErrUnsupportedFieldValue = `invalid value for field %s: %s`
)

var (
	ErrNilInput           = fmt.Errorf(`input channel must not be nil`)
	ErrPresentInput       = fmt.Errorf(`input channel must be nil`)
	ErrNilOutput          = fmt.Errorf(`output channel must not be nil`)
	ErrPresentInputOutput = fmt.Errorf(`either input or output must be set, not both`)
)

type Task interface {
	Run(<-chan *record.Record, chan<- *record.Record) error
	GetName() string
	GetInputCount() int
	GetFailOnError() bool
}

type Base struct {
	Name          string               `yaml:"name,omitempty" json:"name,omitempty"`
	Type          string               `yaml:"type,omitempty" json:"type,omitempty"`
	FailOnError   bool                 `yaml:"fail_on_error,omitempty" json:"fail_on_error,omitempty"`
	Context       map[string]*jq.Query `yaml:"context,omitempty" json:"context,omitempty"`
	CurrentRecord *record.Record       // make record context available to entire task

	recordIndex int
	inputCount  int
	sync.RWMutex
}

func (b *Base) GetFailOnError() bool {
	return b.FailOnError
}

func (b *Base) GetName() string {
	return b.Name
}

func (b *Base) GetRecord(input <-chan *record.Record) (*record.Record, bool) {

	if input == nil {
		return nil, false
	}

	record, ok := <-input
	if ok {
		b.Lock()
		defer b.Unlock()
		b.inputCount++
		b.CurrentRecord = record
	}
	return record, ok

}

func (b *Base) GetInputCount() int {

	b.RLock()
	defer b.RUnlock()

	return b.inputCount

}

func (b *Base) Run(input <-chan *record.Record, output chan<- *record.Record) error {

	if output != nil {
		defer close(output)
	}

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
