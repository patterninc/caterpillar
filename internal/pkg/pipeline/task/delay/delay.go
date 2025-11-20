package delay

import (
	"time"

	"github.com/patterninc/caterpillar/internal/pkg/duration"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

const (
	defaultDelay = duration.Duration(100 * time.Millisecond)
)

type delay struct {
	task.Base `yaml:",inline" json:",inline"`
	Duration  duration.Duration `yaml:"duration,omitempty" json:"duration,omitempty"`
}

func New() (task.Task, error) {
	return &delay{
		Duration: defaultDelay,
	}, nil
}

func (d *delay) Run(input <-chan *record.Record, output chan<- *record.Record) error {

	for {
		r, ok := d.GetRecord(input)
		if !ok {
			break
		}

		// Add delay between records
		if d.Duration > 0 {
			time.Sleep(time.Duration(d.Duration))
		}

		d.SendRecord(r, output)
	}

	return nil

}
