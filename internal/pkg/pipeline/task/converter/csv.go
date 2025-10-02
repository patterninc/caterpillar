package converter

import (
	"bytes"
	ec "encoding/csv"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type csvColumn struct {
	Name      string `yaml:"name" json:"name"`
	IsNumeric bool   `yaml:"is_numeric,omitempty" json:"is_numeric,omitempty"`
}

type csv struct {
	SkipFirst bool         `yaml:"skip_first,omitempty" json:"skip_first,omitempty"`
	Columns   []*csvColumn `yaml:"columns" json:"columns"`
}

// Pre-compile regex for column name sanitization
var columnNameRegex = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func (c *csv) convert(data []byte, _ string) ([]byte, error) {
	// Initialize columns if not provided
	if len(c.Columns) == 0 {
		if err := c.initializeColumns(data); err != nil {
			return nil, err
		}
		return nil, nil
	}

	if c.SkipFirst {
		c.SkipFirst = false
		return nil, nil
	}

	// parse record
	columnsCount := len(c.Columns)
	reader := ec.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = columnsCount

	buffer, err := reader.Read()
	if err != nil {
		return nil, err
	}

	if l := len(buffer); l < columnsCount {
		return nil, fmt.Errorf("record columns count is less than expected number (%d < %d)", l, columnsCount)
	}

	// let's build output record
	record := make(map[string]any)
	for i, column := range c.Columns {
		// do we have a numeric data type?
		if column.IsNumeric {
			v, ok := toNumeric(buffer[i])
			if !ok {
				return nil, fmt.Errorf("expecting a numeric value, got `%v`", buffer[i])
			}
			record[column.Name] = v
			continue
		}
		record[column.Name] = buffer[i]
	}

	return json.Marshal(record)

}

// initializeColumns sets up column definitions based on the first row of CSV data
func (c *csv) initializeColumns(data []byte) error {
	reader := ec.NewReader(bytes.NewReader(data))
	firstRow, err := reader.Read()
	if err != nil {
		return err
	}

	c.Columns = make([]*csvColumn, len(firstRow))

	if c.SkipFirst {
		// Use first row as column headers
		for i, name := range firstRow {
			sanitizedName := strings.ToLower(columnNameRegex.ReplaceAllString(name, "_"))
			c.Columns[i] = &csvColumn{Name: sanitizedName}
		}
		c.SkipFirst = false // Already processed the header row
	} else {
		// Auto-generate column names
		for i := range firstRow {
			c.Columns[i] = &csvColumn{Name: fmt.Sprintf("col%d", i+1)}
		}
	}

	return nil
}

func toNumeric(s string) (any, bool) {

	s = strings.TrimSpace(s)

	if s == "" {
		return nil, false
	}

	// try integer
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		return v, true
	}

	// try float
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return v, true
	}

	return nil, false

}
