package converter

import (
	"fmt"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/ack"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

type converterOutput struct {
	Data     []byte
	Metadata map[string]string
}

type converter interface {
	convert(data []byte, delimiter string) ([]converterOutput, error)
}

type core struct {
	task.Base `yaml:",inline" json:",inline"`
	convert   func([]byte, string) ([]converterOutput, error) `yaml:"-" json:"-"`
	Delimiter string                                          `yaml:"delimiter,omitempty" json:"delimiter,omitempty" default:"\t"`
}

type mapper struct {
	Name      string `yaml:"name" json:"name"`
	Format    string `yaml:"format" json:"format"`
	Delimiter string `yaml:"delimiter,omitempty" json:"delimiter,omitempty" default:"\t"`
}

func New() (task.Task, error) {
	return &core{}, nil
}

func (c *core) UnmarshalYAML(unmarshal func(interface{}) error) error {

	// supported formats
	formats := map[string]converter{
		`csv`:      new(csv),
		`html`:     new(html),
		`sst`:      new(sst),
		`xlsx`:     new(xlsx),
		`xls`:      new(xls),
		`eml`:      new(eml),
		`protobuf`: new(protobuf),
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
	c.Base.Name = m.Name

	return nil

}

func (c *core) Run(input <-chan *record.Record, output chan<- *record.Record) error {

	for {
		r, ok := c.GetRecord(input)
		if !ok {
			break
		}

		outputs, err := c.convert(r.Data, c.Delimiter)
		if err != nil {
			// this record is being abandoned, so settle its ack as failed:
			// leaving it pending would strand a source that defers
			// acknowledgement, which waits for a completion that can no longer
			// come. Rejecting sends the message back for redelivery instead.
			return ack.Rejected(r.Context, err)
		}

		// this is a fan-out: one input record can convert into many outputs -
		// a csv row each, an xlsx sheet each - so the ack must represent all
		// of them, counted before any is sent, or a downstream Done for the
		// first could settle the whole record while the rest are still in
		// flight. A count of 0 (nothing converted) completes it right here.
		sends := 0
		for _, out := range outputs {
			if out.Data != nil {
				sends++
			}
		}
		ack.Fanout(r.Context, sends)

		for _, out := range outputs {
			if out.Data != nil {
				// Add metadata to context
				for k, v := range out.Metadata {
					r.SetContextValue(k, v)
				}

				c.SendData(r.Context, out.Data, output)
			}
		}
	}

	return nil

}
