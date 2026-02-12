package split

import (
	"bytes"
	"context"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

const (
	defaultDelimiter = "\n"
)

type split struct {
	task.Base `yaml:",inline" json:",inline"`
	Delimiter string `yaml:"delimiter,omitempty" json:"delimiter,omitempty"`
}

func New() (task.Task, error) {
	return &split{
		Delimiter: defaultDelimiter,
	}, nil
}

func (s *split) Run(ctx context.Context, input <-chan *record.Record, output chan<- *record.Record) error {

	delimBytes := []byte(s.Delimiter)

	for {
		r, ok := s.GetRecord(input)
		if !ok {
			break
		}

		data := bytes.TrimSuffix(r.Data, delimBytes)
		lines := bytes.Split(data, delimBytes)
		for _, line := range lines {
			s.SendData(r.Meta, line, output)
		}
	}

	return nil

}
