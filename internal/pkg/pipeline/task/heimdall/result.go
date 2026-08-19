package heimdall

import (
	"context"
	"encoding/json"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

var (
	ctx = context.Background()
)

type result struct {
	Columns []*column `yaml:"columns,omitempty" json:"columns,omitempty"`
	Data    [][]any   `yaml:"data,omitempty" json:"data,omitempty"`
}

type column struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func (r *result) toSlice() ([][]byte, error) {

	rowsCount := len(r.Data)
	if rowsCount == 0 {
		return nil, nil
	}

	rows := make([][]byte, 0, rowsCount)

	for _, slice := range r.Data {
		row := make(map[string]any)
		for i, element := range slice {
			row[r.Columns[i].Name] = element
		}
		rowJson, err := json.Marshal(row)
		if err != nil {
			return nil, err
		}
		rows = append(rows, rowJson)
	}

	return rows, nil

}

// srcCtx carries the ack of the record the job was built from, so the results
// descend from it. A fresh context here would emit untracked records and let the
// source acknowledge before any of them was written.
func (h *heimdall) sendToOutput(srcCtx context.Context, result *result, output chan<- *record.Record) error {

	items, err := result.toSlice()
	if err != nil {
		return err
	}

	for _, item := range items {
		h.SendData(srcCtx, item, output)
	}

	return nil

}
