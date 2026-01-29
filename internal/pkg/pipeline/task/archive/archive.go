package archive

import (
	"fmt"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

type actionType string

const (
	actionCompress   actionType = `pack`
	actionDecompress actionType = `unpack`
)

const (
	defaultFormat = `zip`
	defaultAction = `pack`
)

type archiver interface {
	Read(b []byte)
	Write(b []byte)
}

type core struct {
	task.Base `yaml:",inline" json:",inline"`
	Format    string     `yaml:"format,omitempty" json:"format,omitempty"`
	Action    actionType `yaml:"action,omitempty" json:"action,omitempty"`
	FileName  string     `yaml:"file_name,omitempty" json:"file_name,omitempty"`
}

func New() (task.Task, error) {
	return &core{
		Format: defaultFormat,
		Action: defaultAction,
	}, nil
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

	if obj.Action != actionCompress && obj.Action != actionDecompress {
		return fmt.Errorf("invalid action: %s (must be 'compress' or 'decompress')", obj.Action)
	}

	if obj.Action == actionCompress {
		if obj.FileName == "" {
			return fmt.Errorf("file_name must be specified when action is 'pack'")
		}
	}

	*c = core(obj)

	return nil
}

func (c *core) Run(input <-chan *record.Record, output chan<- *record.Record) (err error) {

	if input == nil {
		return task.ErrNilInput
	}

	for {
		r, ok := c.GetRecord(input)
		if !ok {
			break
		}

		if len(r.Data) == 0 {
			continue
		}

		var archiv archiver

		switch c.Format {
		case "tar":
			archiv = &tarArchive{
				Base:       &c.Base,
				FileName:   c.FileName,
				Record:     r,
				OutputChan: output,
			}
		case "zip":
			archiv = &zipArchive{
				Base:       &c.Base,
				FileName:   c.FileName,
				Record:     r,
				OutputChan: output,
			}
		default:
			return fmt.Errorf("unsupported format: %s", c.Format)
		}

		switch c.Action {
		case actionCompress:
			archiv.Write(r.Data)
		case actionDecompress:
			archiv.Read(r.Data)
		}
	}
	return nil
}
