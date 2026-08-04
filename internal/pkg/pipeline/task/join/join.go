package join

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/patterninc/caterpillar/internal/pkg/duration"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/ack"
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
}

func New() (task.Task, error) {
	return &join{
		Delimiter: defaultDelimiter,
	}, nil
}

func (j *join) Run(input <-chan *record.Record, output chan<- *record.Record) error {

	if input == nil || output == nil {
		return ErrIncorrectInputOutput
	}

	totalSize := 0
	var ticker *time.Ticker
	var tickerCh <-chan time.Time
	buffer := make([]*record.Record, 0, defaultBufferSize)

	if j.Duration > 0 {
		ticker = time.NewTicker(time.Duration(j.Duration))
		defer ticker.Stop()
		tickerCh = ticker.C
	}

	// the input receive has to live inside the select: with a default case the
	// select never blocks, so control would fall straight through to a bare
	// channel receive and tickerCh would only be serviced between records -
	// never firing while input is stalled, which is exactly when a partially
	// filled buffer needs flushing. With duration unset tickerCh is nil, so
	// this degenerates to today's plain blocking receive on input.
	for {
		select {
		case r, ok := <-input:
			if !ok {
				// Input channel closed, send any remaining records
				j.flushBuffer(&buffer, output)
				return nil
			}

			buffer = append(buffer, r)
			totalSize += len(r.Data)

			// Check if we've reached the size or number limit
			if (j.Size > 0 && totalSize >= j.Size) || (j.Number > 0 && len(buffer) >= j.Number) {
				j.flushBuffer(&buffer, output)
				totalSize = 0
			}

		case <-tickerCh:
			// Timer expired, send any buffered records
			j.flushBuffer(&buffer, output)
			totalSize = 0
		}
	}
}

// flushBuffer sends any buffered records and resets the buffer
func (j *join) flushBuffer(buffer *[]*record.Record, output chan<- *record.Record) {
	if len(*buffer) > 0 {
		j.sendJoinedRecords(*buffer, output)
		*buffer = (*buffer)[:0]
	}
}

func (j *join) sendJoinedRecords(buffer []*record.Record, output chan<- *record.Record) {

	// Join all data with the specified delimiter
	var joinedData strings.Builder
	ctxs := make([]context.Context, len(buffer))
	for i, r := range buffer {
		if i > 0 {
			joinedData.WriteString(j.Delimiter)
		}
		joinedData.Write(r.Data)
		ctxs[i] = r.Context
	}

	// this is a fan-in: one output record is produced from len(buffer)
	// inputs, so its ack must transitively complete every one of theirs
	// instead of discarding them.
	joinedAck := ack.Joined(ctxs...)
	j.SendData(ack.WithContext(ctx, joinedAck), []byte(joinedData.String()), output)

}
