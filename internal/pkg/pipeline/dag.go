package pipeline

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

type DAG struct {
	// if the node is a task, Name is set
	// if name is set, the rest fields are nil
	Name string `json:"name,omitempty"`
	// if the node is a group, Items and/or Children are set
	Items    []*DAG `json:"items,omitempty"`
	Children []*DAG `json:"children,omitempty"`
}

func (t *DAG) UnmarshalYAML(value *yaml.Node) error {
	d, err := parseInput(value.Value)
	if err != nil {
		return err
	}
	*t = *d // copy parsed node into the receiver. Passing pointer to parseInput to avoid extra allocations
	return nil
}

func parseInput(input string) (*DAG, error) {
	if len(input) == 0 {
		return &DAG{}, nil
	}
	inputString := strings.ReplaceAll(input, " ", "")
	inputString = strings.ReplaceAll(inputString, "\n", "")
	inputString = strings.ReplaceAll(inputString, "\t", "")

	// Validate input for invalid characters and patterns
	if err := validateInput(inputString); err != nil {
		return nil, fmt.Errorf("invalid DAG input, contains invalid patterns: %s", err.Error())
	}
	// Validate groups are well-formed
	if err := validateGroups(inputString); err != nil {
		return nil, fmt.Errorf("invalid DAG input, errors in groups: %v", err)
	}
	// Normalize consecutive arrows for parsing
	inputString = strings.ReplaceAll(inputString, ">>", ">")

	currentItem := &DAG{}
	currentName := ""
	stack := []*DAG{{
		Items: []*DAG{currentItem},
	}}

	for _, c := range inputString {
		if isNameChar(c) {
			currentName += string(c)
			continue
		}
		if len(currentName) > 0 {
			currentItem.Items = append(currentItem.Items, &DAG{
				Name: currentName,
			})
			currentName = ""
		}
		switch c {
		case ',':
			parent := stack[len(stack)-1]
			newItem := &DAG{}
			parent.Items = append(parent.Items, newItem)
			currentItem = newItem
		case '[':
			newItem := &DAG{}
			currentItem.Items = append(currentItem.Items, newItem)
			stack = append(stack, currentItem)
			currentItem = newItem
		case '>':
			newItem := &DAG{}
			currentItem.Children = []*DAG{newItem}
			currentItem = newItem
		case ']':
			currentItem = stack[len(stack)-1]
			stack = stack[:len(stack)-1]
		default:
			// unknown character
			panic("unknown character: " + string(c))
		}
	}

	// Add any remaining name at the end
	if len(currentName) > 0 {
		currentItem.Items = append(currentItem.Items, &DAG{
			Name: currentName,
		})
	}

	return stack[0], nil
}

func isNameChar(c rune) bool {
	return unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_' || c == '-'
}

// Check for invalid patterns: empty brackets, consecutive commas, trailing commas, leading arrows
func validateInput(input string) error {
	invalidChars, err := regexp.Compile(`[^a-zA-Z0-9_\-\[\],>\s]`)
	if err != nil {
		return fmt.Errorf("failed to compile regex: %s", err.Error())
	}
	if invalidChars.MatchString(input) {
		return fmt.Errorf("invalid characters found")
	}

	invalidPatterns, err := regexp.Compile(`\[\s*\]|\[[^,\[\]]+\]|\[[^\[\]]*,\s*\]|\[\s*,[^\[\]]*\]|,\s*,|^>{1,2}`)
	if err != nil {
		return fmt.Errorf("failed to compile regex: %s", err.Error())
	}
	if invalidPatterns.MatchString(input) {
		return fmt.Errorf("empty brackets, consecutive commas, trailing commas, or leading arrows are not allowed")
	}
	return nil
}

func validateGroups(input string) error {
	bracketCount := 0
	arrowCount := 0

	for _, c := range input {
		switch c {
		case '[':
			bracketCount++
			// >[ is invalid
			if arrowCount == 1 {
				return fmt.Errorf("invalid group: >[ pattern found")
			}
			// >>[ is valid
			arrowCount = 0
		case ']':
			bracketCount--
			if bracketCount < 0 {
				return fmt.Errorf("unmatched closing brace ']' found")
			}
			// >] and >>] are invalid
			if arrowCount == 1 {
				return fmt.Errorf("invalid group: >] pattern found")
			}
		case ',':
			// bracket count is zero, comma outside brackets
			if bracketCount == 0 {
				return fmt.Errorf("comma outside brackets found")
			}
			// >, and >>, are invalid
			if arrowCount == 1 {
				return fmt.Errorf("invalid group: >, pattern found")
			}
		case '>':
			arrowCount++
			if arrowCount > 2 {
				return fmt.Errorf("more than two consecutive > found")
			}
		default:
			if arrowCount == 1 {
				return fmt.Errorf("single > found")
			}
			arrowCount = 0
		}
	}

	// Check for unclosed brackets
	if bracketCount > 0 {
		return fmt.Errorf("unmatched opening brace '[' found")
	}

	return nil
}
