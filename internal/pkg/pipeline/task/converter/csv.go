package converter

import (
	"bytes"
	ec "encoding/csv"
	"encoding/json"
	"fmt"
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

func (c *csv) convert(data []byte, _ string) ([]byte, error) {

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
