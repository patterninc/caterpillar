package converter

import (
	"bytes"
	csvEncoder "encoding/csv"
	"fmt"

	"github.com/xuri/excelize/v2"
)

const (
	sheetName = "xlsx_sheet_name"
)

type xlsx struct {
	Sheets          []string       `yaml:"sheets,omitempty" json:"sheets,omitempty"`
	SkipRows        int            `yaml:"skip_rows,omitempty" json:"skip_rows,omitempty"`
	SkipRowsBySheet map[string]int `yaml:"skip_rows_by_sheet,omitempty" json:"skip_rows_by_sheet,omitempty"`
}

func (x *xlsx) convert(data []byte, _ string) ([]converterOutput, error) {
	reader, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	// Get sheets
	sheets := reader.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("no sheet found in the excel file")
	}

	if len(x.Sheets) > 0 {
		sheets = x.Sheets
	}

	// Create one output record per sheet
	outputs := make([]converterOutput, 0, len(sheets))

	for _, sheet := range sheets {
		output, err := readSheet(reader, sheet, x.getRowsToSkip(sheet))
		if err != nil {
			return nil, err
		}

		outputs = append(outputs, output)
	}

	return outputs, nil
}

func readSheet(reader *excelize.File, sheet string, rowsToSkip int) (converterOutput, error) {
	// Create buffer for this sheet
	var buff bytes.Buffer
	writer := csvEncoder.NewWriter(&buff)

	// Get all rows from the sheet
	rows, err := reader.Rows(sheet)
	if err != nil {
		return converterOutput{}, fmt.Errorf("error reading rows from sheet %s: %w", sheet, err)
	}
	defer rows.Close()

	// Write rows to buffer
	i := 0
	for rows.Next() {
		if i < rowsToSkip {
			i++
			continue
		}

		cols, err := rows.Columns()
		if err != nil {
			return converterOutput{}, err
		}

		if err := writer.Write(cols); err != nil {
			return converterOutput{}, err
		}
	}

	// Flush the writer
	writer.Flush()

	return converterOutput{
		Data: buff.Bytes(),
		Metadata: map[string]string{
			sheetName: sheet,
		},
	}, nil
}

func (x *xlsx) getRowsToSkip(sheet string) int {
	rowsToSkip := x.SkipRows
	if x.SkipRowsBySheet != nil {
		if val, found := x.SkipRowsBySheet[sheet]; found {
			rowsToSkip = val
		}
	}

	if rowsToSkip < 0 {
		rowsToSkip = 0
	}

	return rowsToSkip
}
