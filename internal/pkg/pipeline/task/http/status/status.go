package status

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/babourine/x/pkg/set"
)

const (
	itemsSeparator = `,`
	rangeSeparator = `..`
)

type Statuses set.Set[int]

func (s *Statuses) UnmarshalJSON(data []byte) error {

	items, err := New(string(data))
	if err != nil {
		return err
	}

	*s = *items
	return nil

}

func (s *Statuses) UnmarshalYAML(unmarshal func(interface{}) error) error {

	temp := ``

	if err := unmarshal(&temp); err != nil {
		return err
	}

	items, err := New(string(temp))
	if err != nil {
		return err
	}

	*s = *items
	return nil

}

func (s *Statuses) Has(item int) bool {
	return (*set.Set[int])(s).Has(item)
}

func New(input string) (*Statuses, error) {

	result := set.New([]int{})

	items := strings.Split(input, itemsSeparator)

	// process each item
	for _, item := range items {

		// trim space symbols
		item = strings.TrimSpace(item)

		// do we have a range?
		if strings.Contains(item, rangeSeparator) {
			itemParts := strings.Split(item, rangeSeparator)
			if len(itemParts) != 2 {
				return nil, fmt.Errorf("invalid range: %s", item)
			}
			start, err1 := strconv.Atoi(strings.TrimSpace(itemParts[0]))
			end, err2 := strconv.Atoi(strings.TrimSpace(itemParts[1]))
			if err1 != nil || err2 != nil {
				return nil, fmt.Errorf("invalid numbers in range: %s", item)
			}
			if start > end {
				return nil, fmt.Errorf("start greater than end in range: %s", item)
			}
			for i := start; i <= end; i++ {
				result.Add(i)
			}
		} else {
			// Handle single number
			num, err := strconv.Atoi(item)
			if err != nil {
				return nil, fmt.Errorf("invalid number: %s", item)
			}
			result.Add(num)
		}
	}

	return (*Statuses)(result), nil

}
