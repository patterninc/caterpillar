package pipeline

import (
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

type DAG struct {
	// if the node is a task, Name is set
	// if name is set, the rest fields are nil
	Name     string `json:"name,omitempty"`
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
	inputString = strings.ReplaceAll(inputString, ">>", ">")
	inputString = inputString + "@"
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
		case '@':
			break
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

	return stack[0]
}

func isNameChar(c rune) bool {
	return unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_' || c == '-'
}
