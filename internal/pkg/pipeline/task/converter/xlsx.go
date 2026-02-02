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
	Sheets    []string `yaml:"sheets,omitempty" json:"sheets,omitempty"`
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
		// Get all rows from the sheet
		rows, err := reader.GetRows(sheet)
		if err != nil {
			return nil, err
		}

		// Create CSV for this sheet
		var buff bytes.Buffer
		writer := csvEncoder.NewWriter(&buff)

		// Write all rows to CSV
		if err := writer.WriteAll(rows); err != nil {
			return nil, err
		}


		// Add output with sheet name in metadata
		outputs = append(outputs, converterOutput{
			Data: buff.Bytes(),
			Metadata: map[string]string{
				sheetName: sheet,
			},
		})
	}

	return outputs, nil
}
