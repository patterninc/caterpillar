package join

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/patterninc/caterpillar/internal/pkg/duration"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

const (
	defaultDelimiter  = "\n"
	defaultBufferSize = 1000000
)

var (
	ctx = context.Background()
)

var (
	ErrIncorrectInputOutput = fmt.Errorf(`input and output channels must be provided`)
)

type join struct {
	task.Base `yaml:",inline" json:",inline"`
	Size      int               `yaml:"size,omitempty" json:"size,omitempty"`
	Number    int               `yaml:"number,omitempty" json:"number,omitempty"`
	Duration  duration.Duration `yaml:"duration,omitempty" json:"duration,omitempty"`
	Delimiter string            `yaml:"delimiter,omitempty" json:"delimiter,omitempty"`
	buffer    []*record.Record
}

func New() (task.Task, error) {
	return &join{
		buffer:    make([]*record.Record, 0, defaultBufferSize),
		Delimiter: defaultDelimiter,
	}, nil
}

func (j *join) Run(ctx context.Context, input <-chan *record.Record, output chan<- *record.Record) error {

	if input == nil || output == nil {
		return ErrIncorrectInputOutput
	}

	totalSize := 0
	var ticker *time.Ticker
	var tickerCh <-chan time.Time

	if j.Duration > 0 {
		ticker = time.NewTicker(time.Duration(j.Duration))
		defer ticker.Stop()
		tickerCh = ticker.C
	}

	for {
		select {
		default:
			// Try to get a record from input
			r, ok := j.GetRecord(input)
			if !ok {
				// Input channel closed, send any remaining records
				j.flushBuffer(output)
				return nil
			}

			j.buffer = append(j.buffer, r)
			totalSize += len(r.Data)

			// Check if we've reached the size or number limit
			if (j.Size > 0 && totalSize >= j.Size) || (j.Number > 0 && len(j.buffer) >= j.Number) {
				j.flushBuffer(output)
				totalSize = 0
			}

		case <-tickerCh:
			// Timer expired, send any buffered records
			j.flushBuffer(output)
			totalSize = 0
		}
	}
}

// flushBuffer sends any buffered records and resets the buffer
func (j *join) flushBuffer(output chan<- *record.Record) {
	if len(j.buffer) > 0 {
		j.sendJoinedRecords(output)
		// clear the buffer, explicitly nil out the elements to help GC
		for i := range j.buffer {
			j.buffer[i] = nil
		}
		j.buffer = j.buffer[:0]
	}
}

func (j *join) sendJoinedRecords(output chan<- *record.Record) {

	var joinedData bytes.Buffer
	delimBytes := []byte(j.Delimiter)
	for i, r := range j.buffer {
		if i > 0 {
			joinedData.Write(delimBytes)
		}
		joinedData.Write(r.Data)
	}

	j.SendData(nil, joinedData.Bytes(), output)

}
