package pipeline

import (
	"context"
	"fmt"
	"sync"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
	"gopkg.in/yaml.v3"
)

const (
	defaultChannelSize     = 10e3
	defaultTaskConcurrency = 1
)

type Pipeline struct {
	Tasks       tasks `yaml:"tasks,omitempty" json:"tasks,omitempty"`
	ChannelSize int   `yaml:"channel_size,omitempty" json:"channel_size,omitempty"`
	DAG         *DAG  `yaml:"dag,omitempty" json:"dag,omitempty"`
	taskByName  map[string]task.Task
	wg          *sync.WaitGroup
	locker      *sync.Mutex
	errors      map[string]error
	ctx         context.Context
	cancel      context.CancelFunc
}

func (p *Pipeline) Init() error {
	if p.DAG != nil {
		p.tasksToMap()
	}

	p.wg = &sync.WaitGroup{}
	p.locker = &sync.Mutex{}
	p.errors = make(map[string]error)
	return nil
}

func (p *Pipeline) UnmarshalYAML(value *yaml.Node) error {
	type pipeline Pipeline // avoid infinite recursion
	var temp pipeline

	if err := value.Decode(&temp); err != nil {
		return err
	}

	*p = Pipeline(temp)

	return p.Init()
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

	// Create pipeline-level context for cancellation
	p.ctx, p.cancel = context.WithCancel(context.Background())

	// sync
	if p.DAG == nil {
		// data streams
		var input, output chan *record.Record

		for i := tasksCount - 1; i >= 0; i-- {
			if i != 0 {
				input = make(chan *record.Record, p.ChannelSize)
			} else {
				input = nil
			}

			p.runTaskConcurrently(p.Tasks[i], input, output)

			output = input
		}
	} else {
		_, err := p.executeDag(p.DAG, nil, true)
		if err != nil {
			return err
		}
	}
	// wait for all tasks completion
	p.wg.Wait()

	if len(p.errors) > 0 {
		var errorDetails string
		for taskName, err := range p.errors {
			errorDetails += fmt.Sprintf("Task '%s' failed with error: %s\n", taskName, err)
		}
		return fmt.Errorf("pipeline failed with errors:\n%s", errorDetails)
	}

	return nil

}

func (p *Pipeline) tasksToMap() {
	taskMap := make(map[string]task.Task)
	for i := range p.Tasks {
		taskMap[p.Tasks[i].GetName()] = p.Tasks[i]
	}
	p.taskByName = taskMap
}

func (p *Pipeline) executeDag(item *DAG, input <-chan *record.Record, isLeaf bool) (<-chan *record.Record, error) {
	// Process a single task
	if item.Name != "" {
		return p.runTask(item.Name, input, isLeaf)
	}

	// Process items in parallel first
	itemsOutput, err := p.processItems(item.Items, input, isLeaf && len(item.Children) == 0) // is a leaf if is in the leaf path and has no children
	if err != nil {
		return nil, err
	}

	// Then process children with items output
	return p.processChildren(item.Children, itemsOutput, isLeaf) // is a leaf if in the leaf path
}

func (p *Pipeline) runTask(taskName string, input <-chan *record.Record, isLeaf bool) (<-chan *record.Record, error) {
	task, found := p.taskByName[taskName]
	if !found {
		return nil, fmt.Errorf("task not found: %s", taskName)
	}

	var output chan *record.Record
	if !isLeaf {
		output = make(chan *record.Record, p.ChannelSize)
	}

	p.runTaskConcurrently(task, input, output)

	return output, nil
}

func (p *Pipeline) processItems(items []*DAG, input <-chan *record.Record, isLeaf bool) (<-chan *record.Record, error) {
	// Create input channels for parallel processing
	inputChannels := make([]chan *record.Record, len(items))
	outputChannels := make([]<-chan *record.Record, len(items))

	for i, item := range items {
		if input != nil {
			inputChannels[i] = make(chan *record.Record, p.ChannelSize)
		}
		outChan, err := p.executeDag(item, inputChannels[i], isLeaf)
		if err != nil {
			return nil, err
		}
		if outChan != nil {
			outputChannels[i] = outChan
		}
	}

	// Distribute input to all parallel branches
	go p.distributeToChannels(input, inputChannels)

	// Merge outputs from all parallel branches
	return p.mergeChannels(outputChannels), nil
}

func (p *Pipeline) processChildren(children []*DAG, input <-chan *record.Record, isLeaf bool) (<-chan *record.Record, error) {
	if len(children) == 0 {
		return input, nil
	}

	currentOutput := input
	for _, child := range children {
		nextOutput, err := p.executeDag(child, currentOutput, isLeaf)
		if err != nil {
			return nil, err
		}
		currentOutput = nextOutput
	}

	return currentOutput, nil
}

func (p *Pipeline) distributeToChannels(input <-chan *record.Record, outputs []chan *record.Record) {
	defer func() {
		for _, ch := range outputs {
			if ch != nil {
				close(ch)
			}
		}
	}()

	for rec := range input {
		for _, ch := range outputs {
			if ch != nil {
				ch <- rec
			}
		}
	}
}

func (p *Pipeline) mergeChannels(inputs []<-chan *record.Record) <-chan *record.Record {
	output := make(chan *record.Record, p.ChannelSize)

	var wg sync.WaitGroup
	wg.Add(len(inputs))

	for _, input := range inputs {
		if input == nil {
			wg.Done()
			continue
		}

		go func(in <-chan *record.Record) {
			defer wg.Done()
			for rec := range in {
				output <- rec
			}
		}(input)
	}

	go func() {
		wg.Wait()
		close(output)
	}()

	return output
}

func (p *Pipeline) runTaskConcurrently(t task.Task, input <-chan *record.Record, output chan<- *record.Record) {
	// wait for all workers of this task to finish
	p.wg.Add(1)

	concurrency := t.GetTaskConcurrency()

	// wait group for task workers
	taskWg := sync.WaitGroup{}
	taskWg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(ctx context.Context, task task.Task, in <-chan *record.Record, out chan<- *record.Record) {
			defer taskWg.Done()

			if err := task.Run(ctx, in, out); err != nil {
				fmt.Printf("error in %s: %s\n", task.GetName(), err)
				if task.GetFailOnError() {
					p.locker.Lock()
					p.errors[task.GetName()] = err
					p.locker.Unlock()
				}
			}
		}(p.ctx, t, input, output)
	}

	go func(wg *sync.WaitGroup, out chan<- *record.Record) {
		wg.Wait()
		if out != nil {
			close(out)
		}
		p.wg.Done()
	}(&taskWg, output)
}
