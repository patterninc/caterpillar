package replace

import (
	"regexp"

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

	if output != nil {
		for {
			record, ok := r.GetRecord(input)
			if !ok {
				break
			}
			r.SendData(record.Context, []byte(rx.ReplaceAllString(string(record.Data), r.Replacement)), output)
		}
	}

	return nil

}
