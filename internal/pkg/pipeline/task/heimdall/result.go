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

func (h *heimdall) sendToOutput(result *result, output chan<- *record.Record) error {

	items, err := result.toSlice()
	if err != nil {
		return err
	}

	for _, item := range items {
		h.SendData(nil, item, output)
	}

	return nil

}
