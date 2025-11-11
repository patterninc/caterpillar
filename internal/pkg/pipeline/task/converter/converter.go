package converter

import (
	"fmt"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

type converter interface {
	convert(data []byte, delimiter string) ([]byte, error)
}

type core struct {
	task.Base `yaml:",inline" json:",inline"`
	convert   func([]byte, string) ([]byte, error) `yaml:"-" json:"-"`
	Delimiter string                               `yaml:"delimiter,omitempty" json:"delimiter,omitempty" default:"\t"`
}

type mapper struct {
	Format    string `yaml:"format" json:"format"`
	Delimiter string `yaml:"delimiter,omitempty" json:"delimiter,omitempty" default:"\t"`
}

func New() (task.Task, error) {
	return &core{}, nil
}

func (c *core) UnmarshalYAML(unmarshal func(interface{}) error) error {

	// supported formats
	formats := map[string]converter{
		`csv`:  new(csv),
		`html`: new(html),
		`sst`:  new(sst),
	}

	// let's figure out what converter we'll use
	m := &mapper{}
	if err := unmarshal(&m); err != nil {
		return err
	}

	obj, found := formats[m.Format]
	if !found {
		return fmt.Errorf(task.ErrUnsupportedFieldValue, `format`, m.Format)
	}

	// let's set context for converter
	if err := unmarshal(obj); err != nil {
		return err
	}

	c.convert = obj.convert
	c.Delimiter = m.Delimiter

	return nil

}

func (c *core) SupportsTaskConcurrency() bool {
	return true
}

func (c *core) Run(input <-chan *record.Record, output chan<- *record.Record) error {

	for {
		r, ok := c.GetRecord(input)
		if !ok {
			break
		}

		convertedData, err := c.convert(r.Data, c.Delimiter)
		if err != nil {
			return err
		}
		if convertedData != nil {
			c.SendData(r.Context, convertedData, output)
		}
	}

	return nil

}
