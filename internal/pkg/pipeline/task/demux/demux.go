package demux

import (
	"context"
	"sync"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

type Task struct {
	Name   string
	ctx    context.Context
	input  []<-chan *record.Record
	output chan<- *record.Record
}

func New(name string, output chan<- *record.Record) *Task {
	return &Task{
		Name:   name,
		ctx:    context.Background(),
		input:  make([]<-chan *record.Record, 0),
		output: output,
	}
}

func (m *Task) AddInputChannel(input <-chan *record.Record) {
	m.input = append(m.input, input)
}

func (m *Task) Run() error {
	if m.output != nil {
		defer close(m.output)
	}

	var wg sync.WaitGroup

	for i := range m.input {
		wg.Add(1)
		go func(input <-chan *record.Record) {
			defer wg.Done()
			for record := range input {
				r := *record
				r.Context = m.ctx
				m.output <- &r
			}
		}(m.input[i])
	}

	wg.Wait()
	return nil
}
