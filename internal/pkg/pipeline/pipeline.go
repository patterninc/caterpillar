package pipeline

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
	"gopkg.in/yaml.v3"
)

const (
	defaultChannelSize = 10e3
)

// todo
// - parse DAG from YAML
// - execute DAG
// - handle errors properly
// - write tests
type Pipeline struct {
	Tasks       tasks `yaml:"tasks,omitempty" json:"tasks,omitempty"`
	ChannelSize int   `yaml:"channel_size,omitempty" json:"channel_size,omitempty"`
	DAG         *Node `yaml:"dag,omitempty" json:"dag,omitempty"`
	taskByName  map[string]task.Task
	wg          *sync.WaitGroup
	locker      *sync.Mutex
	errors      map[string]error
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

	// sync

	if p.DAG == nil {
		p.wg.Add(tasksCount)

		// data streams
		var input, output chan *record.Record

		for i := tasksCount - 1; i >= 0; i-- {
			if i != 0 {
				input = make(chan *record.Record, p.ChannelSize)
			} else {
				input = nil
			}
			go func(in <-chan *record.Record, out chan<- *record.Record) {
				defer p.wg.Done()
				if err := p.Tasks[i].Run(in, out); err != nil {
					// FIXME: add better error processing
					fmt.Printf("error in %s: %s\n", p.Tasks[i].GetName(), err)
					if p.Tasks[i].GetFailOnError() {
						defer p.locker.Unlock()
						p.locker.Lock()
						p.errors[p.Tasks[i].GetName()] = err
					}
				}
			}(input, output)
			output = input
		}

	} else {
		p.dfs(p.DAG,nil)
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

type Node struct {
	Name     string  `json:"name,omitempty"`
	Items    []*Node `json:"items,omitempty"`
	Children []*Node `json:"children,omitempty"`
}

func parseInput(input string) (string, *Node) {
	if len(input) == 0 {
		return "", &Node{}
	}
	inputString := strings.ReplaceAll(input, " ", "")
	inputString = strings.ReplaceAll(inputString, ">>", ">")
	inputString = inputString + "@"
	currentItem := &Node{}
	currentName := ""
	stack := []*Node{{
		Items: []*Node{currentItem},
	}}

	// groupLevel := 0
	for _, c := range inputString {
		if unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_' || c == '-' {
			currentName += string(c)
			continue
		}
		if len(currentName) > 0 {
			currentItem.Items = append(currentItem.Items, &Node{
				Name: currentName,
			})
			currentName = ""
		}
		switch c {
		case '@':
			break
		case ',':
			parent := stack[len(stack)-1]
			newItem := &Node{}
			parent.Items = append(parent.Items, newItem)
			currentItem = newItem
		case '[':
			newItem := &Node{}
			currentItem.Items = append(currentItem.Items, newItem)
			stack = append(stack, currentItem)
			currentItem = newItem
		case '>':
			newItem := &Node{}
			currentItem.Children = []*Node{newItem}
			currentItem = newItem
		case ']':
			currentItem = stack[len(stack)-1]
			stack = stack[:len(stack)-1]
		default:
			// unknown character
			panic("unknown character: " + string(c))
		}
	}
	ans := stack[0]
	res, err := json.Marshal(ans)
	if err != nil {
		panic(err)
	}
	return string(res), ans
}

func (p *Pipeline) tasksToMap() {
	taskMap := make(map[string]task.Task)
	for i := range p.Tasks {
		taskMap[p.Tasks[i].GetName()] = p.Tasks[i]
	}
	p.taskByName = taskMap
}


func (p *Pipeline) dfs(item *Node, input chan *record.Record) (chan *record.Record, error) {
	// process a single task
	// This type of task never has children or items
	if item.Name != "" {
		task, found := p.taskByName[item.Name]
		if !found {
			return nil, fmt.Errorf("task not found: %s", item.Name)
		}
		output := make(chan *record.Record, p.ChannelSize)
		p.wg.Add(1)
		go func(in <-chan *record.Record, out chan<- *record.Record) {
			defer p.wg.Done()
			if err := task.Run(in, out); err != nil {
				// FIXME: add better error processing
				fmt.Printf("error in %s: %s\n", task.GetName(), err)
				if task.GetFailOnError() {
					defer p.locker.Unlock()
					p.locker.Lock()
					p.errors[task.GetName()] = err
				}
			}
			println("closing output channel for task:", task.GetName())
		}(input, output)
		return output, nil
	}

	itemsInputChannels := make([]chan *record.Record, 0)
	itemsOutputChannels := make([]chan *record.Record, 0)

	time.Sleep(time.Second)
	// process all items in parallel
	for _, it := range item.Items {
		var inChan chan *record.Record
		if input != nil {
			inChan = make(chan *record.Record, p.ChannelSize)
			itemsInputChannels = append(itemsInputChannels, inChan)
		}
		outChan, err := p.dfs(it, inChan)
		if err != nil {
			return nil, err
		}
		itemsOutputChannels = append(itemsOutputChannels, outChan)
	}
	// send message to all items input channels
	go func() {
		for rec := range input {
			println("distributing record to items")
			for _, inChan := range itemsInputChannels {
				inChan <- rec
			}
		}
		for _, inChan := range itemsInputChannels {
			close(inChan)
		}
	}()

	itemsOutputChannel := make(chan *record.Record, p.ChannelSize)
	
	// collect all items outputs and merge them into one channel
	go func() {
		var wg sync.WaitGroup
		wg.Add(len(itemsOutputChannels))
		for _, outChan := range itemsOutputChannels {
			go func(c chan *record.Record) {
				defer wg.Done()
				for rec := range c {
					itemsOutputChannel <- rec
				}
			}(outChan)
		}
		wg.Wait()
		close(itemsOutputChannel)
	}()

	// process all children in parallel
	childrenInputChannels := make([]chan *record.Record, 0)
	childrenOutputChannels := make([]chan *record.Record, 0)

	for _, ch := range item.Children {
		inChan := make(chan *record.Record, p.ChannelSize)
		childrenInputChannels = append(childrenInputChannels, inChan)
		outChan, err := p.dfs(ch, inChan)
		if err != nil {
			return nil, err
		}
		childrenOutputChannels = append(childrenOutputChannels, outChan)
	}

	// send message to all children input channels
	go func() {
		for rec := range itemsOutputChannel {
			for _, inChan := range childrenInputChannels {
				inChan <- rec
			}
		}
		for _, inChan := range childrenInputChannels {
			close(inChan)
		}
	}()

	childrenOutputChannel := make(chan *record.Record, p.ChannelSize)
	
	// collect all children outputs and merge them into one channel
	go func() {
		var wg sync.WaitGroup
		wg.Add(len(childrenOutputChannels))
		for _, outChan := range childrenOutputChannels {
			go func(c chan *record.Record) {
				defer wg.Done()
				for rec := range c {
					childrenOutputChannel <- rec
				}
			}(outChan)
		}
		wg.Wait()
		close(childrenOutputChannel)
	}()
	return childrenOutputChannel, nil
}

func (t *Node) UnmarshalYAML(value *yaml.Node) error {

	_, n := parseInput(value.Value)
	*t = *n // copy parsed node into the receiver. Passing pointer to parseInput to avoid extra allocations
	return nil

}
