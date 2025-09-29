package pipeline

import (
	"fmt"
	"sync"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

const (
	defaultChannelSize = 10e3
)

type Pipeline struct {
	Tasks       tasks `yaml:"tasks,omitempty" json:"tasks,omitempty"`
	ChannelSize int   `yaml:"channel_size,omitempty" json:"channel_size,omitempty"`
}

func (p *Pipeline) Run() error {

	tasksCount := len(p.Tasks)

	if tasksCount == 0 {
		fmt.Println(`nothing to do.`)
		return nil
	}

	if p.ChannelSize <= 0 {
		p.ChannelSize = defaultChannelSize
	}

	// sync
	var wg sync.WaitGroup
	wg.Add(tasksCount)

	// data streams
	var input, output chan *record.Record

	var locker sync.Mutex
	var errors = make(map[string]error)

	for i := tasksCount - 1; i >= 0; i-- {
		if i != 0 {
			input = make(chan *record.Record, p.ChannelSize)
		} else {
			input = nil
		}
		go func(in <-chan *record.Record, out chan<- *record.Record) {
			defer wg.Done()
			if err := p.Tasks[i].Run(in, out); err != nil {
				// FIXME: add better error processing
				fmt.Printf("error in %s: %s\n", p.Tasks[i].GetName(), err)
				if p.Tasks[i].GetFailOnError() {
					defer locker.Unlock()
					locker.Lock()
					errors[p.Tasks[i].GetName()] = err
				}
			}
		}(input, output)
		output = input
	}

	// wait for all tasks completion
	wg.Wait()

	if len(errors) > 0 {
		var errorDetails string
		for taskName, err := range errors {
			errorDetails += fmt.Sprintf("Task '%s' failed with error: %s\n", taskName, err)
		}
		return fmt.Errorf("pipeline failed with errors:\n%s", errorDetails)
	}

	return nil

}
