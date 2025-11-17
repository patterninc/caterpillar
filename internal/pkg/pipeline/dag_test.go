package pipeline

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
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
			input:    "[ task1 >> task2 , task3 ]",
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
			err := validateInput(tt.input)
			if tt.expected != nil {
				if err == nil {
					t.Errorf("validateInput(%q) expected error, but got none", tt.input)
				} else if err.Error() != tt.expected.Error() {
					t.Errorf("validateInput(%q) expected error %q, but got %q", tt.input, tt.expected.Error(), err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("validateInput(%q) unexpected error: %v", tt.input, err)
				}
			}
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
			expected: fmt.Errorf("single > found"),
		},
		{
			name:     "Single > followed by >>",
			input:    "task1>task2>>task3",
			expected: fmt.Errorf("single > found"),
		},
		{
			name:     "Single > in brackets",
			input:    "[task1>task2,task3]",
			expected: fmt.Errorf("single > found"),
		},

		// Invalid cases - comma outside brackets
		{
			name:     "Comma outside brackets",
			input:    "task1,task2",
			expected: fmt.Errorf("comma outside brackets"),
		},
		{
			name:     "Comma in chain outside brackets",
			input:    "task1>>task2,task3",
			expected: fmt.Errorf("comma outside brackets"),
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
			expected: fmt.Errorf("unmatched closing brace ']' found"),
		},
		{
			name:     "Multiple unmatched closing brackets",
			input:    "]]]]",
			expected: fmt.Errorf("unmatched closing brace ']' found"),
		},

		// Invalid cases - too many consecutive >
		{
			name:     "Three consecutive >",
			input:    "task1>>>task2",
			expected: fmt.Errorf("more than two consecutive > found"),
		},
		{
			name:     "Five consecutive >",
			input:    "task1>>>>>task2",
			expected: fmt.Errorf("more than two consecutive > found"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGroups(tt.input)
			if tt.expected != nil {
				if err == nil {
					t.Errorf("validateGroups(%q) expected error, but got none", tt.input)
				} else if !strings.Contains(err.Error(), tt.expected.Error()) {
					t.Errorf("validateGroups(%q) expected error containing %q, but got: %v", tt.input, tt.expected.Error(), err)
				}
			} else {
				if err != nil {
					t.Errorf("validateGroups(%q) unexpected error: %v", tt.input, err)
				}
			}
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

			// Marshal DAG to JSON and compare
			actualBytes, err := json.Marshal(dag)
			if err != nil {
				t.Errorf("Failed to marshal DAG to JSON: %v", err)
				return
			}
			actual := string(actualBytes)

			if actual != tt.expected {
				t.Errorf("For input '%s':\nExpected: %s\nActual:   %s", tt.input, tt.expected, actual)
			}
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
			input:    "task1 >> task2",
			expected: `{"items":[{"items":[{"name":"task1"}],"children":[{"items":[{"name":"task2"}]}]}]}`,
		},
		{
			name:     "Full integration: list with spaces",
			input:    "[ task1 , task2 ]",
			expected: `{"items":[{"items":[{"items":[{"name":"task1"}]},{"items":[{"name":"task2"}]}]}]}`,
		},
		{
			name:     "Full integration: three task chain with spaces",
			input:    "task1 >> task2 >> task3",
			expected: `{"items":[{"items":[{"name":"task1"}],"children":[{"items":[{"name":"task2"}],"children":[{"items":[{"name":"task3"}]}]}]}]}`,
		},
		{
			name:     "Full integration: chain with list and spaces",
			input:    "task1 >> [ task2 , task3 ]",
			expected: `{"items":[{"items":[{"name":"task1"}],"children":[{"items":[{"items":[{"name":"task2"}]},{"items":[{"name":"task3"}]}]}]}]}`,
		},
		{
			name:     "Full integration: list with chain inside and spaces",
			input:    "[ task1 >> task2 , task3 ]",
			expected: `{"items":[{"items":[{"items":[{"name":"task1"}],"children":[{"items":[{"name":"task2"}]}]},{"items":[{"name":"task3"}]}]}]}`,
		},
		{
			name:     "Full integration: nested lists with spaces",
			input:    "[ [ task1 , task2 ] , [ task3 , task4 ] ]",
			expected: `{"items":[{"items":[{"items":[{"items":[{"name":"task1"}]},{"items":[{"name":"task2"}]}]},{"items":[{"items":[{"name":"task3"}]},{"items":[{"name":"task4"}]}]}]}]}`,
		},
		{
			name:     "Full integration: complex nested with mixed spaces",
			input:    "task1 >> [ task2 >> task3 , [ task4 , task5 ] ]",
			expected: `{"items":[{"items":[{"name":"task1"}],"children":[{"items":[{"items":[{"name":"task2"}],"children":[{"items":[{"name":"task3"}]}]},{"items":[{"items":[{"name":"task4"}]},{"items":[{"name":"task5"}]}]}]}]}]}`,
		},
		{
			name:     "Full integration: tabs and newlines",
			input:    "task1\t>>\ttask2\n>>\ntask3",
			expected: `{"items":[{"items":[{"name":"task1"}],"children":[{"items":[{"name":"task2"}],"children":[{"items":[{"name":"task3"}]}]}]}]}`,
		},
		{
			name:     "Full integration: mixed whitespace in list",
			input:    "[\ttask1\n,\t\ttask2\n\n,\ttask3\t]",
			expected: `{"items":[{"items":[{"items":[{"name":"task1"}]},{"items":[{"name":"task2"}]},{"items":[{"name":"task3"}]}]}]}`,
		},
		{
			name:     "Full integration: tasks with underscores and hyphens",
			input:    "task_1 >> task-2 >> task_3-final",
			expected: `{"items":[{"items":[{"name":"task_1"}],"children":[{"items":[{"name":"task-2"}],"children":[{"items":[{"name":"task_3-final"}]}]}]}]}`,
		},
		{
			name:     "Full integration: tasks with numbers",
			input:    "task123 >> [ task456 , task789 ]",
			expected: `{"items":[{"items":[{"name":"task123"}],"children":[{"items":[{"items":[{"name":"task456"}]},{"items":[{"name":"task789"}]}]}]}]}`,
		},
		{
			name:     "Full integration: single task with spaces",
			input:    "   task1   ",
			expected: `{"items":[{"items":[{"name":"task1"}]}]}`,
		},
		{
			name:     "Full integration: deeply nested lists",
			input:    "[ [ [ task1 , task2 ] , task3 ] , task4 ]",
			expected: `{"items":[{"items":[{"items":[{"items":[{"items":[{"name":"task1"}]},{"items":[{"name":"task2"}]}]},{"items":[{"name":"task3"}]}]},{"items":[{"name":"task4"}]}]}]}`,
		},
		{
			name:     "Full integration: chain ending with nested list",
			input:    "task1 >> task2 >> [ [ task3 , task4 ] , task5 ]",
			expected: `{"items":[{"items":[{"name":"task1"}],"children":[{"items":[{"name":"task2"}],"children":[{"items":[{"items":[{"items":[{"name":"task3"}]},{"items":[{"name":"task4"}]}]},{"items":[{"name":"task5"}]}]}]}]}]}`,
		},
		{
			name:     "Full integration: multiple chains in list",
			input:    "[ task1 >> task2 >> task3 , task4 >> task5 , task6 ]",
			expected: `{"items":[{"items":[{"items":[{"name":"task1"}],"children":[{"items":[{"name":"task2"}],"children":[{"items":[{"name":"task3"}]}]}]},{"items":[{"name":"task4"}],"children":[{"items":[{"name":"task5"}]}]},{"items":[{"name":"task6"}]}]}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This mimics the full UnmarshalYAML process
			cleanedInput := cleanInput(tt.input)

			err := validateInput(cleanedInput)
			if err != nil {
				t.Errorf("validateInput failed: %v", err)
				return
			}

			err = validateGroups(cleanedInput)
			if err != nil {
				t.Errorf("validateGroups failed: %v", err)
				return
			}

			// Normalize >> to >
			cleanedInput = strings.ReplaceAll(cleanedInput, ">>", ">")

			dag, err := parseInput(cleanedInput)
			if err != nil {
				t.Errorf("parseInput failed: %v", err)
				return
			}

			// Marshal DAG to JSON and compare
			actualBytes, err := json.Marshal(dag)
			if err != nil {
				t.Errorf("Failed to marshal DAG to JSON: %v", err)
				return
			}
			actual := string(actualBytes)

			if actual != tt.expected {
				t.Errorf("For input '%s':\\nExpected: %s\\nActual:   %s", tt.input, tt.expected, actual)
			}
		})
	}
}
