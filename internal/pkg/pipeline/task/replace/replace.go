package replace

import (
	"regexp"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/ack"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

type replace struct {
	task.Base   `yaml:",inline" json:",inline"`
	Expression  string `yaml:"expression,omitempty" json:"expression,omitempty"`
	Replacement string `yaml:"replacement,omitempty" json:"replacement,omitempty"`
}

func New() (task.Task, error) {
	return &replace{}, nil
}

func (r *replace) Run(input <-chan *record.Record, output chan<- *record.Record) (err error) {

	rx, err := regexp.Compile(r.Expression)
	if err != nil {
		return err
	}

	if input == nil {
		return nil
	}

	if output == nil {
		// terminal: a replace with nowhere to send has no effect, but the
		// input still has to be drained and each record's ack settled, or a
		// source deferring acknowledgement never finishes.
		for {
			record, ok := r.GetRecord(input)
			if !ok {
				return nil
			}
			ack.Drop(record.Context)
		}
	}

	for {
		record, ok := r.GetRecord(input)
		if !ok {
			break
		}
		r.SendData(record.Context, []byte(rx.ReplaceAllString(string(record.Data), r.Replacement)), output)
	}

	return nil

}
