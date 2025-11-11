package jq

import (
	"encoding/json"
	"fmt"

	"github.com/patterninc/caterpillar/internal/pkg/config"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

type jq struct {
	task.Base `yaml:",inline" json:",inline"`
	Path      config.String `yaml:"path,omitempty" json:"path,omitempty"`
	Explode   bool          `yaml:"explode,omitempty" json:"explode,omitempty"`
	AsRaw     bool          `yaml:"as_raw,omitempty" json:"as_raw,omitempty"`
}

func New() (task.Task, error) {
	return &jq{}, nil
}

func (j *jq) SupportsTaskConcurrency() bool {
	return true
}

func (j *jq) Run(input <-chan *record.Record, output chan<- *record.Record) (err error) {

	if output != nil {
		defer close(output)
	}

	if input != nil && output != nil {
		for {
			r, ok := j.GetRecord(input)
			if !ok {
				break
			}

			// First evaluate config templates in the path
			query, err := j.Path.GetJQ(r)
			if err != nil {
				return err
			}

			// Execute the JQ query
			items, err := query.Execute(r.Data)
			if err != nil {
				return err
			}
			if items == nil {
				continue
			}
			if splitItems, ok := items.([]any); j.Explode && ok {
				for _, splitItem := range splitItems {
					if j.AsRaw {
						j.SendData(r.Context, fmt.Appendf(nil, "%v", splitItem), output)
					} else {
						jsonItem, err := json.Marshal(splitItem)
						if err != nil {
							return err
						}
						j.SendData(r.Context, jsonItem, output)
					}
				}
			} else {
				if j.AsRaw {
					j.SendData(r.Context, fmt.Appendf(nil, "%v", items), output)
				} else {
					jsonItem, err := json.Marshal(items)
					if err != nil {
						return err
					}
					j.SendData(r.Context, jsonItem, output)
				}
			}
		}
	}

	return

}
