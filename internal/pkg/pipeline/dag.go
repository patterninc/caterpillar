package pipeline

import (
	"encoding/json"
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
	d := parseInput(value.Value)
	*t = *d // copy parsed node into the receiver. Passing pointer to parseInput to avoid extra allocations
	return nil
}

func parseInput(input string) *DAG {
	if len(input) == 0 {
		return &DAG{}
	}
	inputString := strings.ReplaceAll(input, " ", "")
	inputString = strings.ReplaceAll(inputString, "\n", "")
	inputString = strings.ReplaceAll(inputString, "\t", "")
	if isInvalidInput(inputString) {
		panic("invalid DAG input: " + input)
	}
	if !validateGroups(inputString) {
		panic("invalid DAG input, errors in groups: " + input)
	}
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

	jsonData, err := json.Marshal(stack[0])
	if err != nil {
		panic("failed to marshal DAG to JSON: " + err.Error())
	}
	fmt.Println(string(jsonData))
	return stack[0]
}

func isNameChar(c rune) bool {
	return unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_' || c == '-'
}

func isInvalidInput(input string) bool {
	// Check for invalid patterns: empty brackets, consecutive commas, trailing commas
	re, err := regexp.Compile(`\[\s*\]|\[[^,\[\]]+\]|\[[^\[\]]*,\s*\]|\[\s*,[^\[\]]*\]|,\s*,`)
	if err != nil {
		panic("failed to compile regex: " + err.Error())
	}
	return re.MatchString(input)
}

func validateGroups(input string) bool {
	stack := []rune{} // stack to track opening brackets
	arrowCount := 0
	for _, c := range input {
		switch c {
		case '[':
			stack = append(stack, c)
			// >[ is invalid
			if arrowCount == 1 {
				return false
			}
			arrowCount = 0 // >>[ is valid
		case ']':
			if len(stack) == 0 {
				return false
			}
			top := stack[len(stack)-1]
			if c == ']' && top != '[' {
				return false
			}
			stack = stack[:len(stack)-1]
			// >] and >>] are invalid
			if arrowCount == 1 {
				return false
			}
		case ',':
			// stack is zero, comma outside brackets
			if len(stack) == 0 {
				return false
			}
			// >, and >>, are invalid
			if arrowCount == 1 {
				return false
			}
		case '>':
			arrowCount++
			if arrowCount > 2 {
				return false
			}
		default:
			if arrowCount == 1 {
				return false
			}
			arrowCount = 0
		}
	}

	return len(stack) == 0
}
