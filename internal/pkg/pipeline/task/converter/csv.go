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
	SkipFirst          bool         `yaml:"skip_first,omitempty" json:"skip_first,omitempty"`
	TakeNamesFromFirst bool         `yaml:"take_names_from_first,omitempty" json:"take_names_from_first,omitempty"`
	Columns            []*csvColumn `yaml:"columns" json:"columns"`
}

func (c *csv) convert(data []byte, _ string) ([]byte, error) {

	if c.TakeNamesFromFirst {
		reader := ec.NewReader(bytes.NewReader(data))
		header, err := reader.Read()
		if err != nil {
			return nil, err
		}
		c.Columns = make([]*csvColumn, len(header))
		for i, name := range header {
			c.Columns[i] = &csvColumn{Name: strings.ToLower(regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(name, "_"))}
		}
		c.SkipFirst = false // in case it was set to true, we already read the first line which is the header, and don't want to skip the next line
		c.TakeNamesFromFirst = false
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
