package task

import (
	"context"
	"sync"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

type Demux struct {
	Name   string
	ctx    context.Context
	input  []<-chan *record.Record
	output chan<- *record.Record
}

func NewDemux(name string, output chan<- *record.Record) *Demux {
	return &Demux{
		Name:   name,
		ctx:    context.Background(),
		input:  make([]<-chan *record.Record, 0),
		output: output,
	}
}

func (d *Demux) AddInputChannel(input <-chan *record.Record) {
	d.input = append(d.input, input)
}

func (d *Demux) Run() error {
	if d.output != nil {
		defer close(d.output)
	}

	var wg sync.WaitGroup

	for i := range d.input {
		wg.Add(1)
		go func(input <-chan *record.Record) {
			defer wg.Done()
			for record := range input {
				r := *record
				r.Context = d.ctx
				d.output <- &r
			}
		}(d.input[i])
	}

	wg.Wait()
	return nil
}
