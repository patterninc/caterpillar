package converter

import (
	"bytes"
	csvEncoder "encoding/csv"
	"fmt"
	"io"
	"math"
	"strconv"

	"github.com/yamitzky/xlrd-go/xlrd"

	"github.com/patterninc/caterpillar/internal/pkg/textutil"
)

type xls struct {
	Sheets             []string       `yaml:"sheets,omitempty" json:"sheets,omitempty"`
	SkipRows           int            `yaml:"skip_rows,omitempty" json:"skip_rows,omitempty"`
	SkipRowsBySheet    map[string]int `yaml:"skip_rows_by_sheet,omitempty" json:"skip_rows_by_sheet,omitempty"`
	SanitizeHeaders    bool           `yaml:"sanitize_headers,omitempty" json:"sanitize_headers,omitempty"`
	SanitizeSheetNames bool           `yaml:"sanitize_sheet_names,omitempty" json:"sanitize_sheet_names,omitempty"`
}

func (x *xls) convert(data []byte, _ string) (outputs []converterOutput, err error) {
	// recover to avoid crash due to panic.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic while parsing .xls file: %v", r)
		}
	}()

	// Logfile defaults to stdout, so redirect its diagnostics to avoid polluting task output.
	reader, err := xlrd.OpenWorkbook(``, &xlrd.OpenWorkbookOptions{
		FileContents: data,
		Logfile:      io.Discard,
	})
	if err != nil {
		return nil, err
	}

	// Get sheets
	sheets := reader.SheetNames()
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

func (x *xls) readSheet(reader *xlrd.Book, sheet string) (converterOutput, error) {
	rowsToSkip := x.getRowsToSkip(sheet)
	// Create buffer for this sheet
	var buff bytes.Buffer
	writer := csvEncoder.NewWriter(&buff)

	// Get all rows from the sheet
	// Unlike xlsx, which uses excelise, there is no api to get formatted rows as output,
	// so we require custom formatting over the cells based on their format types
	rows, err := x.sheetRows(reader, sheet)
	if err != nil {
		return converterOutput{}, fmt.Errorf("error reading rows from sheet %s: %w", sheet, err)
	}

	// Write rows to buffer
	isHeaderRow := true
	for i, cols := range rows {
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
	if err := writer.Error(); err != nil {
		return converterOutput{}, err
	}

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

// -------------------------- Everything below are helper functions ----------------------------//

func (x *xls) sheetRows(reader *xlrd.Book, sheet string) ([][]string, error) {
	sh, err := reader.SheetByName(sheet)
	if err != nil {
		return nil, err
	}

	rows := make([][]string, 0, sh.NRows)
	for r := 0; r < sh.NRows; r++ {
		cols := make([]string, 0, sh.NCols)
		for c := 0; c < sh.NCols; c++ {
			cols = append(cols, cellString(reader, sh, r, c))
		}
		rows = append(rows, trimTrailingEmpty(cols))
	}
	for len(rows) > 0 && len(rows[len(rows)-1]) == 0 {
		rows = rows[:len(rows)-1]
	}

	return rows, nil
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

// cellString renders a cell to its CSV text
func cellString(book *xlrd.Book, sheet *xlrd.Sheet, r, c int) string {
	switch sheet.RawCellType(r, c) {
	case xlrd.XL_CELL_TEXT:
		if s, ok := sheet.RawCellValue(r, c).(string); ok {
			return s
		}
	case xlrd.XL_CELL_NUMBER:
		f, ok := sheet.RawCellValue(r, c).(float64)
		if !ok {
			return ``
		}
		if isDateCell(book, sheet.RawCellXFIndex(r, c)) {
			if s, ok := formatDate(f, book.Datemode); ok {
				return s
			}
		}
		return formatNumber(f)
	case xlrd.XL_CELL_BOOLEAN:
		switch v := sheet.RawCellValue(r, c).(type) {
		case int:
			return boolText(v != 0)
		case bool:
			return boolText(v)
		}
	case xlrd.XL_CELL_ERROR:
		// xlrd-go stores the raw BIFF error code, so we emit "#ERR<code>".
		return fmt.Sprintf(`#ERR%v`, sheet.RawCellValue(r, c))
	}
	return ``
}

// isDateCell reports whether a numeric cell carries a date/time number format
func isDateCell(book *xlrd.Book, xfIndex int) bool {
	if xfIndex < 0 || xfIndex >= len(book.XFList) {
		return false
	}
	formatKey := book.XFList[xfIndex].FormatKey
	if isBuiltinDateFormat(formatKey) {
		return true
	}
	if book.FormatMap == nil {
		return false
	}
	format := book.FormatMap[formatKey]
	if format == nil || format.FormatString == `` {
		return false
	}
	return xlrd.IsDateFormatString(book, format.FormatString)
}

// isBuiltinDateFormat reports whether a built-in number-format key is a date/time
// format. Ranges match excelize
func isBuiltinDateFormat(key int) bool {
	switch {
	case key >= 14 && key <= 22,
		key >= 27 && key <= 36,
		key >= 45 && key <= 47,
		key >= 50 && key <= 58,
		key >= 71 && key <= 81:
		return true
	default:
		return false
	}
}

func formatDate(value float64, datemode int) (string, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return ``, false
	}
	t, err := xlrd.XldateAsDatetime(value, datemode)
	if err != nil {
		return ``, false
	}
	switch {
	case value >= 0 && value < 1:
		return t.Format(`15:04:05`), true
	case value != math.Floor(value):
		return t.Format(`2006-01-02 15:04:05`), true
	default:
		return t.Format(`2006-01-02`), true
	}
}

func boolText(b bool) string {
	if b {
		return `TRUE`
	}
	return `FALSE`
}

func formatNumber(f float64) string {
	if f == math.Trunc(f) && !math.IsInf(f, 0) && !math.IsNaN(f) && math.Abs(f) < 1e18 {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func trimTrailingEmpty(row []string) []string {
	end := len(row)
	for end > 0 && row[end-1] == "" {
		end--
	}
	return row[:end]
}
