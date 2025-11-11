package sample

import (
	"fmt"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

const (
	defaultLimit   = 10
	defaultPercent = 1
	defaultDivider = 1000
	defaultSize    = 50000
	defaultFilter  = `random`
)

var (
	ErrInvalidConfig = fmt.Errorf(`sample task cannot be first or last in the pipeline`)
)

var (
	samplers = map[string]func(*sample) (sampler, error){
		`head`:    newHead,
		`nth`:     newNth,
		`percent`: newPercent,
		`random`:  newRandom,
		`tail`:    newTail,
	}
)

type sample struct {
	task.Base `yaml:",inline" json:",inline"`
	Filter    string `yaml:"filter,omitempty" json:"filter,omitempty"`
	Limit     int    `yaml:"limit,omitempty" json:"limit,omitempty"`
	Percent   int    `yaml:"percent,omitempty" json:"percent,omitempty"`
	Divider   int    `yaml:"divider,omitempty" json:"divider,omitempty"`
	Size      int    `yaml:"size,omitempty" json:"size,omitempty"`
}

func New() (task.Task, error) {
	return &sample{
		Limit:   defaultLimit,
		Percent: defaultPercent,
		Filter:  defaultFilter,
		Divider: defaultDivider,
		Size:    defaultSize,
	}, nil
}

func (s *sample) SupportsTaskConcurrency() bool {
	return true
}

func (s *sample) Run(input <-chan *record.Record, output chan<- *record.Record) error {

	// if this task is firs or last in the pipeline, let's bail...
	if input == nil || output == nil {
		return ErrInvalidConfig
	}

	// let's setup filter function
	samplerNew, found := samplers[s.Filter]
	if !found {
		return fmt.Errorf("unknown filter: %s", s.Filter)
	}
	sampler, err := samplerNew(s)
	if err != nil {
		return err
	}

	// process input
	for {
		r, ok := s.GetRecord(input)
		if !ok {
			break
		}
		if err := sampler.filter(r, output); err != nil {
			return err
		}
	}

	// drain sampler
	if err := sampler.drain(output); err != nil {
		return err
	}

	return nil

}
