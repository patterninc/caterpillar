package mux

import (
	"context"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

type Mux struct {
	Name    string
	ctx     context.Context
	input   <-chan *record.Record
	outputs []chan<- *record.Record
}

func New(name string, input <-chan *record.Record) *Mux {
	return &Mux{
		Name:    name,
		ctx:     context.Background(),
		input:   input,
		outputs: make([]chan<- *record.Record, 0),
	}
}

func (m *Mux) AddOutputChannel(output chan<- *record.Record) {
	m.outputs = append(m.outputs, output)
}

func (m *Mux) Run() error {

	// defer func() {
	// 	for _, output := range m.outputs {
	// 		close(output)
	// 	}
	// }()

	for record := range m.input {
		r := *record
		r.Context = m.ctx
		// Send a copy of the record to each output channel
		for _, output := range m.outputs {
			output <- &r
		}
	}
	return nil
}
