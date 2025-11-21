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
	if len(value.Value) == 0 {
		return fmt.Errorf("zero length DAG expression")
	}

	input := value.Value

	// validate input string for correct syntax
	if err := validateInput(input); err != nil {
		return fmt.Errorf("invalid DAG groups: %v", err)
	}

	// clean input string by removing whitespace characters
	// and normalizing consecutive arrows
	input = cleanInput(input)

	// parse input string to DAG structure
	d, err := parseInput(input)
	if err != nil {
		return err
	}

	*t = *d // copy parsed node into the receiver. Passing pointer to parseInput to avoid extra allocations
	return nil
}

func cleanInput(input string) string {
	inputString := strings.ReplaceAll(input, " ", "")
	inputString = strings.ReplaceAll(inputString, "\n", "")
	inputString = strings.ReplaceAll(inputString, "\t", "")
	return strings.ReplaceAll(inputString, ">>", ">")
}

func parseInput(input string) (*DAG, error) {
	currentItem := &DAG{}
	currentName := ""
	stack := []*DAG{{
		Items: []*DAG{currentItem},
	}}

	for _, c := range input {
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

// Validate the input check for invalid characters and group syntax
func validateInput(input string) error {
	// check for invalid characters
	invalidChars, err := regexp.Compile(`[^a-zA-Z0-9_\-\[\],>\s]`)
	if err != nil {
		return fmt.Errorf("failed to compile regex: %s", err.Error())
	}
	if invalidChars.MatchString(input) {
		return fmt.Errorf("invalid characters found")
	}

	// validate groups
	bracketCount := 0
	i := 0
	lastChar := rune(0)

	for i < len(input) {
		isPrevArrow := i > 0 && input[i-1] == '>'
		isNextArrow := i < len(input)-1 && input[i+1] == '>'

		if isNameChar(rune(input[i])) || input[i] == ' ' || input[i] == '\n' || input[i] == '\t' {
			i++
			continue
		}

		switch input[i] {
		case '[':
			bracketCount++
			if isNextArrow {
				return fmt.Errorf("error at index %d, invalid group: [> pattern found", i)
			}
			if input[i+1] == ']' {
				return fmt.Errorf("error at index %d, empty group '[]' found", i)
			}
			if input[i+1] == ',' {
				return fmt.Errorf("error at index %d, invalid group: [, pattern found", i)
			}
		case ']':
			bracketCount--
			if bracketCount < 0 {
				return fmt.Errorf("error at index %d, unmatched closing brace ']' found", i)
			}
			if isPrevArrow {
				return fmt.Errorf("error at index %d, invalid group: >] pattern found", i)
			}
			if lastChar == '[' {
				return fmt.Errorf("error at index %d, single identifier group '[identifier]' found", i)
			}
		case ',':
			if bracketCount == 0 {
				return fmt.Errorf("error at index %d, comma outside brackets found", i)
			}
			if isPrevArrow {
				return fmt.Errorf("error at index %d, invalid group: >, pattern found", i)
			}
			if isNextArrow {
				return fmt.Errorf("error at index %d, invalid group: ,> pattern found", i)
			}
			if input[i+1] == ']' {
				return fmt.Errorf("error at index %d, invalid group: ,] pattern found", i)
			}
			if input[i+1] == ',' {
				return fmt.Errorf("error at index %d, consecutive commas found", i)
			}
		case '>':
			if isPrevArrow && i <= 1 {
				return fmt.Errorf("error at index %d, leading >> found", i)
			}
			if isPrevArrow && isNextArrow {
				return fmt.Errorf("error at index %d, more than two consecutive > found", i)
			}
			if !isPrevArrow && !isNextArrow {
				return fmt.Errorf("error at index %d, single > found", i)
			}
		}

		lastChar = rune(input[i])
		i++
	}

	// Check for unclosed brackets
	if bracketCount > 0 {
		return fmt.Errorf("unmatched opening brace '[' found")
	}

	return nil
}
