package pipeline

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

// Test cleanInput function
func TestCleanInput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "String with spaces",
			input:    "task1 >> task2",
			expected: "task1>>task2",
		},
		{
			name:     "String with tabs",
			input:    "task1\t>>\ttask2",
			expected: "task1>>task2",
		},
		{
			name:     "String with newlines",
			input:    "task1\n>>\ntask2",
			expected: "task1>>task2",
		},
		{
			name:     "Mixed whitespace",
			input:    " task1 \t >> \n task2 ",
			expected: "task1>>task2",
		},
		{
			name:     "List with spaces",
			input:    "[ task1 , task2 ]",
			expected: "[task1,task2]",
		},
		{
			name:     "Complex nested with whitespace",
			input:    "[      task1 >> task2  , task3                                ]",
			expected: "[task1>>task2,task3]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanInput(tt.input)
			if result != tt.expected {
				t.Errorf("cleanInput(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Test validateInput function
func TestValidateInput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected error
	}{
		// Valid cases
		{
			name:     "Simple task",
			input:    "task1",
			expected: nil,
		},
		{
			name:     "Chain with >>",
			input:    "task1>>task2",
			expected: nil,
		},
		{
			name:     "List",
			input:    "[task1,task2]",
			expected: nil,
		},
		{
			name:     "Tasks with underscores",
			input:    "task_1>>task_2",
			expected: nil,
		},
		{
			name:     "Tasks with hyphens",
			input:    "task-1>>task-2",
			expected: nil,
		},
		{
			name:     "Tasks with numbers",
			input:    "task1>>task2>>task3",
			expected: nil,
		},

		// Invalid cases - invalid characters
		{
			name:     "Special character $",
			input:    "task1$task2",
			expected: fmt.Errorf("invalid characters found"),
		},
		{
			name:     "Special character @",
			input:    "task1@task2",
			expected: fmt.Errorf("invalid characters found"),
		},
		{
			name:     "Parentheses",
			input:    "(task1,task2)",
			expected: fmt.Errorf("invalid characters found"),
		},
		{
			name:     "Curly braces",
			input:    "{task1,task2}",
			expected: fmt.Errorf("invalid characters found"),
		},

		// Invalid cases - invalid patterns
		{
			name:     "Empty brackets",
			input:    "[]",
			expected: fmt.Errorf("empty brackets, consecutive commas, trailing commas, or leading arrows are not allowed"),
		},
		{
			name:     "Single item in brackets",
			input:    "[task1]",
			expected: fmt.Errorf("empty brackets, consecutive commas, trailing commas, or leading arrows are not allowed"),
		},
		{
			name:     "Trailing comma",
			input:    "[task1,task2,]",
			expected: fmt.Errorf("empty brackets, consecutive commas, trailing commas, or leading arrows are not allowed"),
		},
		{
			name:     "Leading comma",
			input:    "[,task1,task2]",
			expected: fmt.Errorf("empty brackets, consecutive commas, trailing commas, or leading arrows are not allowed"),
		},
		{
			name:     "Consecutive commas",
			input:    "[task1,,task2]",
			expected: fmt.Errorf("empty brackets, consecutive commas, trailing commas, or leading arrows are not allowed"),
		},
		{
			name:     "Leading >>",
			input:    ">>task1",
			expected: fmt.Errorf("empty brackets, consecutive commas, trailing commas, or leading arrows are not allowed"),
		},
		{
			name:     "Leading >",
			input:    ">task1",
			expected: fmt.Errorf("empty brackets, consecutive commas, trailing commas, or leading arrows are not allowed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testValidationFunction(t, "validateInput", tt.input, tt.expected, validateInput)
		})
	}
}

// Test validateGroups function
func TestValidateGroups(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected error
	}{
		// Valid cases
		{
			name:     "Simple task",
			input:    "task1",
			expected: nil,
		},
		{
			name:     "Chain with >>",
			input:    "task1>>task2",
			expected: nil,
		},
		{
			name:     "List",
			input:    "[task1,task2]",
			expected: nil,
		},
		{
			name:     "Nested list",
			input:    "[[task1,task2],[task3,task4]]",
			expected: nil,
		},
		{
			name:     "Complex structure",
			input:    "task1>>[task2,task3>>task4]",
			expected: nil,
		},

		// Invalid cases - single >
		{
			name:     "Single > operator",
			input:    "task1>task2",
			expected: fmt.Errorf("error at index 5, single > found"),
		},
		{
			name:     "Single > followed by >>",
			input:    "task1>task2>>task3",
			expected: fmt.Errorf("error at index 5, single > found"),
		},
		{
			name:     "Single > in brackets",
			input:    "[task1>task2,task3]",
			expected: fmt.Errorf("error at index 6, single > found"),
		},

		// Invalid cases - comma outside brackets
		{
			name:     "Comma outside brackets",
			input:    "task1,task2",
			expected: fmt.Errorf("error at index 5, comma outside brackets found"),
		},
		{
			name:     "Comma in chain outside brackets",
			input:    "task1>>task2,task3",
			expected: fmt.Errorf("error at index 12, comma outside brackets found"),
		},

		// Invalid cases - unmatched brackets
		{
			name:     "Unclosed opening bracket",
			input:    "[task1,task2",
			expected: fmt.Errorf("unmatched opening brace '[' found"),
		},
		{
			name:     "Multiple unclosed brackets",
			input:    "[[task1,task2]",
			expected: fmt.Errorf("unmatched opening brace '[' found"),
		},
		{
			name:     "Unmatched closing bracket",
			input:    "task1]",
			expected: fmt.Errorf("error at index 5, unmatched closing brace ']' found"),
		},
		{
			name:     "Multiple unmatched closing brackets",
			input:    "]]]]",
			expected: fmt.Errorf("error at index 0, unmatched closing brace ']' found"),
		},

		// Invalid cases - too many consecutive >
		{
			name:     "Three consecutive >",
			input:    "task1>>>task2",
			expected: fmt.Errorf("error at index 6, more than two consecutive > found"),
		},
		{
			name:     "Five consecutive >",
			input:    "task1>>>>>task2",
			expected: fmt.Errorf("error at index 6, more than two consecutive > found"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testValidationFunction(t, "validateGroups", tt.input, tt.expected, validateGroups)
		})
	}
}

// Test parseInput function (assumes clean input)
func TestParseInput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: `{"items":[{}]}`,
		},
		{
			name:     "Single task",
			input:    "task1",
			expected: `{"items":[{"items":[{"name":"task1"}]}]}`,
		},
		{
			name:     "Two task chain (cleaned >>)",
			input:    "task1>task2", // Note: >> has been normalized to > by preprocessing
			expected: `{"items":[{"items":[{"name":"task1"}],"children":[{"items":[{"name":"task2"}]}]}]}`,
		},
		{
			name:     "List",
			input:    "[task1,task2]",
			expected: `{"items":[{"items":[{"items":[{"name":"task1"}]},{"items":[{"name":"task2"}]}]}]}`,
		},
		{
			name:     "Chain with list",
			input:    "task1>[task2,task3]", // Note: >> normalized to >
			expected: `{"items":[{"items":[{"name":"task1"}],"children":[{"items":[{"items":[{"name":"task2"}]},{"items":[{"name":"task3"}]}]}]}]}`,
		},
		{
			name:     "Nested lists",
			input:    "[[task1,task2],[task3,task4]]",
			expected: `{"items":[{"items":[{"items":[{"items":[{"name":"task1"}]},{"items":[{"name":"task2"}]}]},{"items":[{"items":[{"name":"task3"}]},{"items":[{"name":"task4"}]}]}]}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("parseInput panicked unexpectedly for input '%s': %v", tt.input, r)
				}
			}()

			dag, err := parseInput(tt.input)
			if err != nil {
				t.Errorf("parseInput(%q) unexpected error: %v", tt.input, err)
				return
			}
			if dag == nil {
				t.Errorf("parseInput(%q) returned nil DAG", tt.input)
				return
			}

			testParserOutput(t, dag, tt.input, tt.expected)
		})
	}
}

// Test full integration
func TestIntegrationParsing(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Full integration: spaces removed and parsed",
			input:    "dag: task1 >> task2",
			expected: `{"items":[{"items":[{"name":"task1"}],"children":[{"items":[{"name":"task2"}]}]}]}`,
		},
		{
			name:     "Full integration: list with spaces",
			input:    "dag: \"[ task1 , task2 ]\"",
			expected: `{"items":[{"items":[{"items":[{"name":"task1"}]},{"items":[{"name":"task2"}]}]}]}`,
		},
		{
			name:     "Full integration: three task chain with spaces",
			input:    "dag: task1 >> task2 >> task3",
			expected: `{"items":[{"items":[{"name":"task1"}],"children":[{"items":[{"name":"task2"}],"children":[{"items":[{"name":"task3"}]}]}]}]}`,
		},
		{
			name:     "Full integration: chain with list and spaces",
			input:    "dag: task1 >> [ task2 , task3 ]",
			expected: `{"items":[{"items":[{"name":"task1"}],"children":[{"items":[{"items":[{"name":"task2"}]},{"items":[{"name":"task3"}]}]}]}]}`,
		},
		{
			name:     "Full integration: list with chain inside and spaces",
			input:    "dag: \"[ task1 >> task2 , task3 ]\"",
			expected: `{"items":[{"items":[{"items":[{"name":"task1"}],"children":[{"items":[{"name":"task2"}]}]},{"items":[{"name":"task3"}]}]}]}`,
		},
		{
			name:     "Full integration: nested lists with spaces",
			input:    "dag: \"[ [ task1 , task2 ] , [ task3 , task4 ] ]\"",
			expected: `{"items":[{"items":[{"items":[{"items":[{"name":"task1"}]},{"items":[{"name":"task2"}]}]},{"items":[{"items":[{"name":"task3"}]},{"items":[{"name":"task4"}]}]}]}]}`,
		},
		{
			name:     "Full integration: complex nested with mixed spaces",
			input:    "dag: task1 >> [ task2 >> task3 , [ task4 , task5 ] ]",
			expected: `{"items":[{"items":[{"name":"task1"}],"children":[{"items":[{"items":[{"name":"task2"}],"children":[{"items":[{"name":"task3"}]}]},{"items":[{"items":[{"name":"task4"}]},{"items":[{"name":"task5"}]}]}]}]}]}`,
		},
		{
			name:     "Full integration: tabs and newlines",
			input:    "dag: \"task1\t>>\ttask2>>task3\"",
			expected: `{"items":[{"items":[{"name":"task1"}],"children":[{"items":[{"name":"task2"}],"children":[{"items":[{"name":"task3"}]}]}]}]}`,
		},
		{
			name:     "Full integration: mixed whitespace in list",
			input:    "dag: \"[\ttask1,\ttask2,\ttask3]\"",
			expected: `{"items":[{"items":[{"items":[{"name":"task1"}]},{"items":[{"name":"task2"}]},{"items":[{"name":"task3"}]}]}]}`,
		},
		{
			name:     "Full integration: tasks with underscores and hyphens",
			input:    "dag: task_1 >> task-2 >> task_3-final",
			expected: `{"items":[{"items":[{"name":"task_1"}],"children":[{"items":[{"name":"task-2"}],"children":[{"items":[{"name":"task_3-final"}]}]}]}]}`,
		},
		{
			name:     "Full integration: tasks with numbers",
			input:    "dag: task123 >> [ task456 , task789 ]",
			expected: `{"items":[{"items":[{"name":"task123"}],"children":[{"items":[{"items":[{"name":"task456"}]},{"items":[{"name":"task789"}]}]}]}]}`,
		},
		{
			name:     "Full integration: single task with spaces",
			input:    "dag:    task1   ",
			expected: `{"items":[{"items":[{"name":"task1"}]}]}`,
		},
		{
			name:     "Full integration: deeply nested lists",
			input:    "dag: \"[ [ [ task1 , task2 ] , task3 ] , task4 ]\"",
			expected: `{"items":[{"items":[{"items":[{"items":[{"items":[{"name":"task1"}]},{"items":[{"name":"task2"}]}]},{"items":[{"name":"task3"}]}]},{"items":[{"name":"task4"}]}]}]}`,
		},
		{
			name:     "Full integration: chain ending with nested list",
			input:    "dag: task1 >> task2 >> [ [ task3 , task4 ] , task5 ]",
			expected: `{"items":[{"items":[{"name":"task1"}],"children":[{"items":[{"name":"task2"}],"children":[{"items":[{"items":[{"items":[{"name":"task3"}]},{"items":[{"name":"task4"}]}]},{"items":[{"name":"task5"}]}]}]}]}]}`,
		},
		{
			name:     "Full integration: multiple chains in list",
			input:    "dag: \"[ task1 >> task2 >> task3 , task4 >> task5 , task6 ]\"",
			expected: `{"items":[{"items":[{"items":[{"name":"task1"}],"children":[{"items":[{"name":"task2"}],"children":[{"items":[{"name":"task3"}]}]}]},{"items":[{"name":"task4"}],"children":[{"items":[{"name":"task5"}]}]},{"items":[{"name":"task6"}]}]}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Pipeline{}

			// Unmarshal input YAML into DAG
			if err := yaml.Unmarshal([]byte(tt.input), p); err != nil {
				t.Errorf("yaml.Unmarshal() unexpected error for input '%s': %v", tt.input, err)
				return
			}

			testParserOutput(t, p.DAG, tt.input, tt.expected)
		})
	}
}

// Helper function to test validation functions
func testValidationFunction(t *testing.T, testName string, input string, expected error, validationFunc func(string) error) {
	t.Helper()

	err := validationFunc(input)
	if expected == nil {
		if err != nil {
			t.Errorf("%s(%q) unexpected error: %v", testName, input, err)
		}
	} else {
		if !assert.EqualError(t, err, expected.Error()) {
			t.Errorf("%s(%q) error = %v, expected %v", testName, input, err, expected)
		}
	}
}

func testParserOutput(t *testing.T, dag *DAG, input, expected string) {
	t.Helper()

	// Marshal DAG to JSON and compare
	actualBytes, err := json.Marshal(dag)
	if err != nil {
		t.Errorf("Failed to marshal DAG to JSON: %v", err)
		return
	}
	actual := string(actualBytes)

	if actual != expected {
		t.Errorf("For input '%s':\nExpected: %s\nActual:   %s", input, expected, actual)
	}
}
