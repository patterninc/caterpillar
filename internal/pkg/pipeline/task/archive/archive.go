package archive

import (
	"context"
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

type channelStruct struct {
	InputChan  <-chan *record.Record
	OutputChan chan<- *record.Record
}

var (
	supportedFormats = map[string]func(bs *task.Base, chStruct *channelStruct) archiver{
		"zip": func(bs *task.Base, ch *channelStruct) archiver {
			return &zipArchive{
				Base:          bs,
				channelStruct: ch,
			}
		},
		"tar": func(bs *task.Base, ch *channelStruct) archiver {
			return &tarArchive{
				Base:          bs,
				channelStruct: ch,
			}
		},
	}

	supportedActions = map[string]func(archiver) func(){
		`pack`:   func(a archiver) func() { return a.Write },
		`unpack`: func(a archiver) func() { return a.Read },
	}
)

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

func (c *core) Run(ctx context.Context, input <-chan *record.Record, output chan<- *record.Record) (err error) {

	if input == nil {
		return task.ErrNilInput
	}

	var archiv archiver

	archiv = supportedFormats[c.Format](&c.Base, &channelStruct{
		InputChan:  input,
		OutputChan: output,
	})

	actionFunc := supportedActions[string(c.Action)](archiv)
	actionFunc()

	return nil
}
