package jq

import (
	"encoding/json"
	"fmt"

	"github.com/patterninc/caterpillar/internal/pkg/config"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

type jq struct {
	task.Base   `yaml:",inline" json:",inline"`
	Path        config.String `yaml:"path,omitempty" json:"path,omitempty"`
	Explode     bool          `yaml:"explode,omitempty" json:"explode,omitempty"`
	AsRaw       bool          `yaml:"as_raw,omitempty" json:"as_raw,omitempty"`
	IgnoreError bool          `yaml:"ignore_error" json:"ignore_error"`
}

func New() (task.Task, error) {
	return &jq{IgnoreError: true}, nil
}

func (j *jq) Run(input <-chan *record.Record, output chan<- *record.Record) (err error) {

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
				// An ignored query error costs only its own record, since it describes
				// that record rather than the pipeline, and warns because the run is not
				// failing over it. Otherwise it is critical: return, and let fail_on_error
				// judge the verdict as it does for every other task.
				if j.IgnoreError {
					fmt.Printf("WARN: %s: skipping record %d: %s\n", j.GetName(), r.ID, err)
					continue
				}
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
