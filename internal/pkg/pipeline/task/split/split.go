package split

import (
	"context"
	"strings"

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

	for {
		r, ok := s.GetRecord(input)
		if !ok {
			break
		}
		lines := strings.Split(strings.TrimSuffix(string(r.Data), s.Delimiter), s.Delimiter)
		for _, line := range lines {
			s.SendData(r.Meta, []byte(line), output)
		}
	}

	return nil

}
