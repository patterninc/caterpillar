package converter

import (
	"bytes"
	csvEncoder "encoding/csv"
	"fmt"
	"os"

	"github.com/patterninc/grate"
	gratexls "github.com/patterninc/grate/xls"

	"github.com/patterninc/caterpillar/internal/pkg/textutil"
)

func init() {
	gratexls.HandleHyperlink = gratexls.PreserveDisplayText
}

type xls struct {
	Sheets             []string       `yaml:"sheets,omitempty" json:"sheets,omitempty"`
	SkipRows           int            `yaml:"skip_rows,omitempty" json:"skip_rows,omitempty"`
	SkipRowsBySheet    map[string]int `yaml:"skip_rows_by_sheet,omitempty" json:"skip_rows_by_sheet,omitempty"`
	SanitizeHeaders    bool           `yaml:"sanitize_headers,omitempty" json:"sanitize_headers,omitempty"`
	SanitizeSheetNames bool           `yaml:"sanitize_sheet_names,omitempty" json:"sanitize_sheet_names,omitempty"`
}

func (x *xls) convert(data []byte, _ string) (outputs []converterOutput, err error) {
	// recover so a panic does not crash the task.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic while parsing .xls file: %v", r)
		}
	}()

	// grate.Open reads from a file path, so spill the in-memory bytes to a temp file.
	tmp, err := os.CreateTemp("", "caterpillar-*.xls")
	if err != nil {
		return nil, fmt.Errorf("creating temp file for .xls: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(data); err != nil {
		tmp.Close()
		return nil, fmt.Errorf("writing temp .xls: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return nil, fmt.Errorf("closing temp .xls: %w", err)
	}

	reader, err := grate.Open(tmp.Name())
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	// Get sheets (visible + hidden)
	sheets, err := reader.List()
	if err != nil {
		return nil, err
	}
	if h, ok := reader.(interface{ ListHidden() ([]string, error) }); ok {
		if hidden, herr := h.ListHidden(); herr == nil {
			sheets = append(sheets, hidden...)
		}
	}
	if len(sheets) == 0 {
		return nil, fmt.Errorf("no sheet found in the excel file")
	}

	if len(x.Sheets) > 0 {
		sheets = x.Sheets
	}

	// Create one output record per sheet
	outputs = make([]converterOutput, 0, len(sheets))
	for _, sheet := range sheets {
		output, err := x.readSheet(reader, sheet)
		if err != nil {
			return nil, err
		}

		outputs = append(outputs, output)
	}

	return outputs, nil
}

func (x *xls) readSheet(reader grate.Source, sheet string) (converterOutput, error) {
	rowsToSkip := x.getRowsToSkip(sheet)
	// Create buffer for this sheet
	var buff bytes.Buffer
	writer := csvEncoder.NewWriter(&buff)

	// Get all rows from the sheet
	rows, err := reader.Get(sheet)
	if err != nil {
		return converterOutput{}, fmt.Errorf("error reading rows from sheet %s: %w", sheet, err)
	}

	// grate pads every row to the sheet's max width and emits one trailing empty
	// row (the sheet dimension is "last row + 1").
	var allRows [][]string
	for rows.Next() {
		allRows = append(allRows, trimTrailingEmpty(rows.Strings()))
	}
	for len(allRows) > 0 && len(allRows[len(allRows)-1]) == 0 {
		allRows = allRows[:len(allRows)-1]
	}

	// Write rows to buffer
	isHeaderRow := true
	for i, cols := range allRows {
		if i < rowsToSkip {
			continue
		}

		if x.SanitizeHeaders && isHeaderRow {
			for j, col := range cols {
				cols[j] = textutil.Slugify(col)
			}
			isHeaderRow = false
		}

		if err := writer.Write(cols); err != nil {
			return converterOutput{}, err
		}
	}

	// Flush the writer
	writer.Flush()

	outputSheetName := sheet
	if x.SanitizeSheetNames {
		outputSheetName = textutil.Slugify(sheet)
	}

	return converterOutput{
		Data: buff.Bytes(),
		Metadata: map[string]string{
			sheetName: outputSheetName,
		},
	}, nil
}

func (x *xls) getRowsToSkip(sheet string) int {
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

func trimTrailingEmpty(row []string) []string {
	end := len(row)
	for end > 0 && row[end-1] == "" {
		end--
	}
	return row[:end]
}
