package archive

import (
	"fmt"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

type actionType string

const (
	actionPack   actionType = `pack`
	actionUnpack actionType = `unpack`
)

const (
	defaultFormat = `zip`
	defaultAction = `pack`
)

type archiver interface {
	Read()
	Write()
}

type core struct {
	task.Base `yaml:",inline" json:",inline"`
	Format    string     `yaml:"format,omitempty" json:"format,omitempty"`
	Action    actionType `yaml:"action,omitempty" json:"action,omitempty"`
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

	if obj.Action != actionPack && obj.Action != actionUnpack {
		return fmt.Errorf("invalid action: %s (must be 'pack' or 'unpack')", obj.Action)
	}

	*c = core(obj)

	return nil
}

func (c *core) Run(input <-chan *record.Record, output chan<- *record.Record) (err error) {

	if input == nil {
		return task.ErrNilInput
	}

	var archiv archiver

	switch c.Format {
	case "tar":
		archiv = &tarArchive{
			Base:       &c.Base,
			OutputChan: output,
			InputChan:  input,
		}
	case "zip":
		archiv = &zipArchive{
			Base:       &c.Base,
			OutputChan: output,
			InputChan:  input,
		}
	default:
		return fmt.Errorf("unsupported format: %s", c.Format)
	}

	switch c.Action {
	case actionPack:
		archiv.Write()
	case actionUnpack:
		archiv.Read()
	}

	return nil
}
