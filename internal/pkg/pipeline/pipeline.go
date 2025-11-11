package pipeline

import (
	"fmt"
	"sync"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

const (
	defaultChannelSize     = 10e3
	defaultTaskConcurrency = 1
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
	var mainWg sync.WaitGroup
	mainWg.Add(tasksCount)
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

		// Only use concurrency if the task supports it
		taskConcurrency := defaultTaskConcurrency
		if p.Tasks[i].SupportsTaskConcurrency() {
			taskConcurrency = p.Tasks[i].GetTaskConcurrency()
			if taskConcurrency <= 0 {
				taskConcurrency = defaultTaskConcurrency
			}
		}

		var taskWg sync.WaitGroup
		taskWg.Add(taskConcurrency)

		for c := 0; c < taskConcurrency; c++ {
			go func(taskIndex int, in <-chan *record.Record, out chan<- *record.Record) {
				defer taskWg.Done()
				if err := p.Tasks[taskIndex].Run(in, out); err != nil {
					// FIXME: add better error processing
					fmt.Printf("error in %s: %s\n", p.Tasks[taskIndex].GetName(), err)
					if p.Tasks[taskIndex].GetFailOnError() {
						defer locker.Unlock()
						locker.Lock()
						errors[p.Tasks[taskIndex].GetName()] = err
					}
				}
			}(i, input, output)
		}

		// Pipeline orchestrator closes the output channel after all workers complete
		go func(wg *sync.WaitGroup, out chan<- *record.Record) {
			wg.Wait()
			if out != nil {
				close(out)
			}
			mainWg.Done()
		}(&taskWg, output)

		output = input
	}

	// wait for all tasks completion
	mainWg.Wait()

	if len(errors) > 0 {
		var errorDetails string
		for taskName, err := range errors {
			errorDetails += fmt.Sprintf("Task '%s' failed with error: %s\n", taskName, err)
		}
		return fmt.Errorf("pipeline failed with errors:\n%s", errorDetails)
	}

	return nil

}
