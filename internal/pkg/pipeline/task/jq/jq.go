package jq

import (
	"encoding/json"
	"fmt"

	"github.com/patterninc/caterpillar/internal/pkg/config"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/ack"
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

func (j *jq) Run(input <-chan *record.Record, output chan<- *record.Record) (err error) {

	if input == nil {
		return nil
	}

	if output == nil {
		// terminal: the transform has no effect, but input still has to be
		// drained and each record's ack settled, or a source deferring
		// acknowledgement never finishes.
		for {
			r, ok := j.GetRecord(input)
			if !ok {
				return nil
			}
			ack.Drop(r.Context)
		}
	}

	for {
		r, ok := j.GetRecord(input)
		if !ok {
			break
		}

		// First evaluate config templates in the path
		query, err := j.Path.GetJQ(r)
		if err != nil {
			return ack.Rejected(r.Context, err)
		}

		// Execute the JQ query
		items, err := query.Execute(r.Data)
		if err != nil {
			return ack.Rejected(r.Context, err)
		}
		if items == nil {
			ack.Drop(r.Context)
			continue
		}
		if splitItems, ok := items.([]any); j.Explode && ok {
			// marshal every item before adjusting the ack: failing partway
			// through afterwards would leave branches counted but never sent,
			// and a partial fan-out can't be unwound.
			payloads := make([][]byte, 0, len(splitItems))
			for _, splitItem := range splitItems {
				if j.AsRaw {
					payloads = append(payloads, fmt.Appendf(nil, "%v", splitItem))
					continue
				}
				jsonItem, err := json.Marshal(splitItem)
				if err != nil {
					return ack.Rejected(r.Context, err)
				}
				payloads = append(payloads, jsonItem)
			}

			ack.Fanout(r.Context, len(payloads))

			for _, payload := range payloads {
				j.SendData(r.Context, payload, output)
			}
		} else {
			if j.AsRaw {
				j.SendData(r.Context, fmt.Appendf(nil, "%v", items), output)
			} else {
				jsonItem, err := json.Marshal(items)
				if err != nil {
					return ack.Rejected(r.Context, err)
				}
				j.SendData(r.Context, jsonItem, output)
			}
		}
	}

	return

}
