package echo

import (
	"fmt"
	"time"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

const (
	timeNowFormat = `2006-01-02 15:04:05`
)

type echo struct {
	task.Base `yaml:",inline" json:",inline"`
	OnlyData  bool `yaml:"only_data,omitempty" json:"only_data,omitempty"`
}

func New() (task.Task, error) {
	return &echo{}, nil
}

func (e *echo) Run(input <-chan *record.Record, output chan<- *record.Record) (err error) {

	for {
		r, ok := e.GetRecord(input)
		if !ok {
			break
		}

		var item []byte

		if e.OnlyData {
			item = r.Data
		} else {
			item = r.Bytes()
		}

		// fmt.Println(r.GetContextValue(`names`))
		fmt.Println(time.Now().Format(timeNowFormat), `-`, e.Name, `-`, string(item))

		if output != nil {
			e.SendRecord(r, output)
		}
	}

	return

}
