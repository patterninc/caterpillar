// package pipeline

// import (
// 	"encoding/json"
// 	"fmt"
// 	"strings"
// 	"sync"
// 	"unicode"

// 	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
// 	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
// )

// const (
// 	defaultChannelSize = 10e3
// )

// type Pipeline struct {
// 	Tasks       tasks `yaml:"tasks,omitempty" json:"tasks,omitempty"`
// 	ChannelSize int   `yaml:"channel_size,omitempty" json:"channel_size,omitempty"`
// 	taskByName  map[string]task.Task
// 	wg          *sync.WaitGroup
// 	locker      *sync.Mutex
// 	errors      map[string]error
// }

// func (p *Pipeline) Run() error {

// 	tasksCount := len(p.Tasks)

// 	if tasksCount == 0 {
// 		fmt.Println(`nothing to do.`)
// 		return nil
// 	}

// 	if p.ChannelSize <= 0 {
// 		p.ChannelSize = defaultChannelSize
// 	}

// 	// sync
// 	var wg sync.WaitGroup
// 	wg.Add(tasksCount)

// 	// data streams
// 	var input, output chan *record.Record

// 	var locker sync.Mutex
// 	var errors = make(map[string]error)

// 	for i := tasksCount - 1; i >= 0; i-- {
// 		if i != 0 {
// 			input = make(chan *record.Record, p.ChannelSize)
// 		} else {
// 			input = nil
// 		}
// 		go func(in <-chan *record.Record, out chan<- *record.Record) {
// 			defer wg.Done()
// 			if err := p.Tasks[i].Run(in, out); err != nil {
// 				// FIXME: add better error processing
// 				fmt.Printf("error in %s: %s\n", p.Tasks[i].GetName(), err)
// 				if p.Tasks[i].GetFailOnError() {
// 					defer locker.Unlock()
// 					locker.Lock()
// 					errors[p.Tasks[i].GetName()] = err
// 				}
// 			}
// 		}(input, output)
// 		output = input
// 	}

// 	// wait for all tasks completion
// 	wg.Wait()

// 	if len(errors) > 0 {
// 		var errorDetails string
// 		for taskName, err := range errors {
// 			errorDetails += fmt.Sprintf("Task '%s' failed with error: %s\n", taskName, err)
// 		}
// 		return fmt.Errorf("pipeline failed with errors:\n%s", errorDetails)
// 	}

// 	return nil

// }

// type Item struct {
// 	Name     string  `json:"name,omitempty"`
// 	Items    []*Item `json:"items,omitempty"`
// 	Children []*Item `json:"children,omitempty"`
// }

// func main() {
// 	type testCase struct {
// 		name     string
// 		input    string
// 		expected string
// 	}
// 	testCases := []testCase{
// 		{
// 			name:     "testCase1",
// 			input:    "task1 >> [ task2 , task5 ] >> task6",
// 			expected: `{"items":[{"items":[{"name":"task1"}],"children":[{"items":[{"items":[{"name":"task2"}]},{"items":[{"name":"task5"}]}],"children":[{"items":[{"name":"task6"}]}]}]}]}`,
// 		},
// 		{
// 			name:     "testCase2",
// 			input:    "task1 >> task5  >> task6",
// 			expected: `{"items":[{"items":[{"name":"task1"}],"children":[{"items":[{"name":"task5"}],"children":[{"items":[{"name":"task6"}]}]}]}]}`,
// 		},
// 		{
// 			name:     "testCase3",
// 			input:    "task1 >> [ task2 >> [ task3 , task4 ] , task5 ]",
// 			expected: `{"items":[{"items":[{"name":"task1"}],"children":[{"items":[{"items":[{"name":"task2"}],"children":[{"items":[{"items":[{"name":"task3"}]},{"items":[{"name":"task4"}]}]}]},{"items":[{"name":"task5"}]}]}]}]}`,
// 		},
// 		{
// 			name:     "testCase4",
// 			input:    "task1 >> [ task2 >> [ task3 , task4 ] , task5 ] >> task6",
// 			expected: `{"items":[{"items":[{"name":"task1"}],"children":[{"items":[{"items":[{"name":"task2"}],"children":[{"items":[{"items":[{"name":"task3"}]},{"items":[{"name":"task4"}]}]}]},{"items":[{"name":"task5"}]}],"children":[{"items":[{"name":"task6"}]}]}]}]}`,
// 		},
// 		{
// 			name:     "testCase5",
// 			input:    "[task1, task2] >> task3",
// 			expected: `{"items":[{"items":[{"items":[{"name":"task1"}]},{"items":[{"name":"task2"}]}],"children":[{"items":[{"name":"task3"}]}]}]}`,
// 		},
// 	}
// 	for _, tc := range testCases {
// 		result, item := parseInput(tc.input)
// 		if result != tc.expected {
// 			panic("expected: " + tc.expected + ", got: " + result)
// 		}
// 		println("test case passed: " + tc.name)

// 	}

// }

// func parseInput(input string) (string, *Item) {
// 	inputString := strings.ReplaceAll(input, " ", "")
// 	inputString = strings.ReplaceAll(inputString, ">>", ">")
// 	inputString = inputString + "@"
// 	currentItem := &Item{}
// 	currentName := ""
// 	stack := []*Item{{
// 		Items: []*Item{currentItem},
// 	}}

// 	// groupLevel := 0
// 	for _, c := range inputString {
// 		if unicode.IsLetter(c) || unicode.IsDigit(c) {
// 			currentName += string(c)
// 			continue
// 		}
// 		if len(currentName) > 0 {
// 			currentItem.Items = append(currentItem.Items, &Item{
// 				Name: currentName,
// 			})
// 			currentName = ""
// 		}
// 		switch c {
// 		case '@':
// 			break
// 		case ',':
// 			parent := stack[len(stack)-1]
// 			newItem := &Item{}
// 			parent.Items = append(parent.Items, newItem)
// 			currentItem = newItem
// 		case '[':
// 			newItem := &Item{}
// 			currentItem.Items = append(currentItem.Items, newItem)
// 			stack = append(stack, currentItem)
// 			currentItem = newItem
// 		case '>':
// 			newItem := &Item{}
// 			currentItem.Children = []*Item{newItem}
// 			currentItem = newItem
// 		case ']':
// 			currentItem = stack[len(stack)-1]
// 			stack = stack[:len(stack)-1]
// 		default:
// 			// unknown character
// 			panic("unknown character: " + string(c))
// 		}
// 	}
// 	ans := stack[0]
// 	res, err := json.Marshal(ans)
// 	if err != nil {
// 		panic(err)
// 	}
// 	return string(res), ans
// }

// func (p *Pipeline) tasksToMap() {
// 	taskMap := make(map[string]*task.Task)
// 	for i := range p.Tasks {
// 		taskMap[p.Tasks[i].GetName()] = &p.Tasks[i]
// 	}
// 	p.taskByName = taskMap
// }

// // func (p *Pipeline) executeDag(root *Item) error {

// // }

// //	type Item struct {
// //		Name     string  `json:"name,omitempty"`
// //		Items    []*Item `json:"items,omitempty"`
// //		Children []*Item `json:"children,omitempty"`
// //	}
// func (p *Pipeline) dfs(item *Item, input chan *record.Record) (chan *record.Record, error) {
// 	// process a single task
// 	// This type of task never has children or items
// 	if item.Name != "" {
// 		task, found := p.taskByName[item.Name]
// 		if !found {
// 			return nil, fmt.Errorf("task not found: %s", item.Name)
// 		}
// 		output := make(chan *record.Record, p.ChannelSize)
// 		p.wg.Add(1)
// 		go func(in <-chan *record.Record, out chan<- *record.Record) {
// 			defer p.wg.Done()
// 			if err := task.Run(in, out); err != nil {
// 				// FIXME: add better error processing
// 				fmt.Printf("error in %s: %s\n", task.GetName(), err)
// 				if task.GetFailOnError() {
// 					defer p.locker.Unlock()
// 					p.locker.Lock()
// 					p.errors[task.GetName()] = err
// 				}
// 			}
// 		}(input, output)
// 		return output, nil
// 	}

// 	itemsInputChannels := make([]chan *record.Record, 0)
// 	itemsOutputChannels := make([]chan *record.Record, 0)

// 	// process all items in parallel
// 	for _, it := range item.Items {
// 		inChan := make(chan *record.Record, p.ChannelSize)
// 		itemsInputChannels = append(itemsInputChannels, inChan)
// 		outChan, err := p.dfs(it, inChan)
// 		if err != nil {
// 			return nil, err
// 		}
// 		itemsOutputChannels = append(itemsOutputChannels, outChan)
// 	}
// 	// send message to all items input channels
// 	go func() {
// 		for rec := range input {
// 			for _, inChan := range itemsInputChannels {
// 				inChan <- rec
// 			}
// 		}
// 		for _, inChan := range itemsInputChannels {
// 			close(inChan)
// 		}
// 	}()

// 	itemsOutputChannel:= make(chan *record.Record, p.ChannelSize)
// 	// collect all items outputs and merge them into one channel
// 	go func() {
// 		var wg sync.WaitGroup
// 		wg.Add(len(itemsOutputChannels))
// 		for _, outChan := range itemsOutputChannels {
// 			go func(c chan *record.Record) {
// 				defer wg.Done()
// 				for rec := range c {
// 					itemsOutputChannel <- rec
// 				}
// 			}(outChan)
// 		}
// 		wg.Wait()
// 		close(itemsOutputChannel)
// 	}()


// 	// process all children in parallel
// 	childrenInputChannels := make([]chan *record.Record, 0)
// 	childrenOutputChannels := make([]chan *record.Record, 0)

// 	for _, ch := range item.Children {
// 		inChan := make(chan *record.Record, p.ChannelSize)
// 		childrenInputChannels = append(childrenInputChannels, inChan)
// 		outChan, err := p.dfs(ch, inChan)
// 		if err != nil {
// 			return nil, err
// 		}
// 		childrenOutputChannels = append(childrenOutputChannels, outChan)
// 	}

// 	// send message to all children input channels
// 	go func() {
// 		for rec := range itemsOutputChannel {
// 			for _, inChan := range childrenInputChannels {
// 				inChan <- rec
// 			}
// 		}
// 		for _, inChan := range childrenInputChannels {
// 			close(inChan)
// 		}
// 	}()

// 	childrenOutputChannel:= make(chan *record.Record, p.ChannelSize)
// 	// collect all children outputs and merge them into one channel
// 	go func() {
// 		var wg sync.WaitGroup
// 		wg.Add(len(childrenOutputChannels))
// 		for _, outChan := range childrenOutputChannels {
// 			go func(c chan *record.Record) {
// 				defer wg.Done()
// 				for rec := range c {
// 					childrenOutputChannel <- rec
// 				}
// 			}(outChan)
// 		}
// 		wg.Wait()
// 		close(childrenOutputChannel)
// 	}()
// 	return childrenOutputChannel, nil
// }
