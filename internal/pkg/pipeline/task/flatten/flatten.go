package flatten

import (
	"context"
	"encoding/json"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

type flatten struct {
	task.Base       `yaml:",inline" json:",inline"`
	IncludeOriginal string `yaml:"include_original,omitempty" json:"include_original,omitempty"`
}

func New() (task.Task, error) {
	return &flatten{}, nil
}

func (f *flatten) Run(ctx context.Context, input <-chan *record.Record, output chan<- *record.Record) error {

	for {
		r, ok := f.GetRecord(input)
		if !ok {
			break
		}

		var data map[string]any
		if err := json.Unmarshal(r.Data, &data); err != nil {
			return err
		}

		flat := make(map[string]any)
		flattenObject(data, ``, flat)

		if f.IncludeOriginal != `` {
			flat[f.IncludeOriginal] = data
		}

		flatJson, err := json.Marshal(flat)
		if err != nil {
			return err
		}

		f.SendData(r.Meta, flatJson, output)
	}

	return nil

}

func flattenObject(data any, prefix string, result map[string]any) {

	switch v := data.(type) {
	case map[string]any:
		for key, value := range v {
			newKey := key
			if prefix != `` {
				newKey = prefix + `_` + key
			}
			flattenObject(value, newKey, result)
		}
	default: // keep everything else intact
		result[prefix] = v
	}

}
