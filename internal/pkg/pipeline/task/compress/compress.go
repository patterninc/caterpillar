package compress

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

const (
	defaultFormat = `gzip`
	defaultAction = `compress`
)

type core struct {
	task.Base `yaml:",inline" json:",inline"`
	Format    string `yaml:"format,omitempty" json:"format,omitempty"`
	Action    string `yaml:"action,omitempty" json:"action,omitempty"`
}

func New() (task.Task, error) {
	return &core{
		Format: defaultFormat,
		Action: defaultAction,
	}, nil
}

func (c *core) SupportsTaskConcurrency() bool {
	return true
}

func (c *core) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type raw core

	obj := raw{
		Format: defaultFormat,
		Action: defaultAction,
	}
	if err := unmarshal(&obj); err != nil {
		return err
	}

	if _, ok := formatHandlers[obj.Format]; !ok {
		return fmt.Errorf(task.ErrUnsupportedFieldValue, `format`, obj.Format)
	}

	*c = core(obj)

	return nil
}

func (c *core) Run(input <-chan *record.Record, output chan<- *record.Record) (err error) {

	if input == nil {
		return task.ErrNilInput
	}

	ctx := context.Background()

	for {
		r, ok := c.GetRecord(input)
		if !ok {
			break
		}

		// skip empty records
		if len(r.Data) == 0 {
			continue
		}

		var transformedData []byte
		var err error
		if c.Action == defaultAction {
			if transformedData, err = c.compress(r); err != nil {
				return err
			}
		} else {
			if transformedData, err = c.decompress(r); err != nil {
				return err
			}
		}

		// skip empty transformed data
		if len(transformedData) == 0 {
			continue
		}

		if output != nil {
			c.SendData(ctx, transformedData, output)
		}
	}

	return nil

}

// read compressed data
func (c *core) decompress(r *record.Record) ([]byte, error) {

	reader := bytes.NewReader(r.Data)

	decompressReader, err := formatHandlers[c.Format].NewReader(reader)
	if err != nil {
		return nil, err
	}

	content, err := io.ReadAll(decompressReader)
	if err != nil {
		return nil, err
	}

	if err := decompressReader.Close(); err != nil {
		return nil, err
	}

	return content, nil
}

// compress data
func (c *core) compress(r *record.Record) ([]byte, error) {
	var buffer bytes.Buffer

	writer := formatHandlers[c.Format].NewWriter(&buffer)

	_, err := writer.Write(r.Data)
	if err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}
