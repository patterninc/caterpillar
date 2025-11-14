package pipeline

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseInputValidGrammar(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(*testing.T, *DAG)
	}{
		// VALID: Basic expression cases per grammar
		{
			name:  "Empty string",
			input: "",
			validate: func(t *testing.T, dag *DAG) {
				if dag.Name != "" || len(dag.Items) != 0 || len(dag.Children) != 0 {
					t.Errorf("Expected empty DAG, got %+v", dag)
				}
			},
		},
		{
			name:  "Single identifier (IDENTIFIER group)",
			input: "task1",
			validate: func(t *testing.T, dag *DAG) {
				if len(dag.Items) != 1 {
					t.Errorf("Expected 1 item, got %d", len(dag.Items))
					return
				}
				if len(dag.Items[0].Items) != 1 {
					t.Errorf("Expected 1 nested item, got %d", len(dag.Items[0].Items))
					return
				}
				if dag.Items[0].Items[0].Name != "task1" {
					t.Errorf("Expected name 'task1', got '%s'", dag.Items[0].Items[0].Name)
				}
			},
		},

		// VALID: Chain patterns (group >> group >> ...)
		{
			name:  "Two task chain (group >> group)",
			input: "task1 >> task2",
			validate: func(t *testing.T, dag *DAG) {
				firstItem := dag.Items[0]
				if len(firstItem.Items) != 1 || firstItem.Items[0].Name != "task1" {
					t.Errorf("Expected task1, got %+v", firstItem.Items)
					return
				}
				if len(firstItem.Children) != 1 {
					t.Errorf("Expected 1 child, got %d", len(firstItem.Children))
					return
				}
				secondItem := firstItem.Children[0]
				if len(secondItem.Items) != 1 || secondItem.Items[0].Name != "task2" {
					t.Errorf("Expected task2, got %+v", secondItem.Items)
				}
			},
		},
		{
			name:  "Three task chain",
			input: "task1 >> task2 >> task3",
			validate: func(t *testing.T, dag *DAG) {
				current := dag.Items[0]
				expectedTasks := []string{"task1", "task2", "task3"}

				for i, expectedTask := range expectedTasks {
					if len(current.Items) != 1 || current.Items[0].Name != expectedTask {
						t.Errorf("Expected %s at position %d, got %+v", expectedTask, i, current.Items)
						return
					}
					if i < len(expectedTasks)-1 {
						if len(current.Children) != 1 {
							t.Errorf("Expected 1 child from %s, got %d", expectedTask, len(current.Children))
							return
						}
						current = current.Children[0]
					}
				}
			},
		},

		// VALID: non_single_expression_list (minimum 2 expressions)
		{
			name:  "Two element list [expr, expr]",
			input: "[task1, task2]",
			validate: func(t *testing.T, dag *DAG) {
				firstItem := dag.Items[0]
				if len(firstItem.Items) != 2 {
					t.Errorf("Expected 2 items in list, got %d", len(firstItem.Items))
					return
				}
				if len(firstItem.Items[0].Items) != 1 || firstItem.Items[0].Items[0].Name != "task1" {
					t.Errorf("Expected first item to be 'task1', got %+v", firstItem.Items[0])
				}
				if len(firstItem.Items[1].Items) != 1 || firstItem.Items[1].Items[0].Name != "task2" {
					t.Errorf("Expected second item to be 'task2', got %+v", firstItem.Items[1])
				}
			},
		},
		{
			name:  "Three element list [expr, expr, expr]",
			input: "[task1, task2, task3]",
			validate: func(t *testing.T, dag *DAG) {
				firstItem := dag.Items[0]
				if len(firstItem.Items) != 3 {
					t.Errorf("Expected 3 items in list, got %d", len(firstItem.Items))
					return
				}
				expectedNames := []string{"task1", "task2", "task3"}
				for i, expected := range expectedNames {
					if len(firstItem.Items[i].Items) != 1 || firstItem.Items[i].Items[0].Name != expected {
						t.Errorf("Expected item %d to be '%s', got %+v", i, expected, firstItem.Items[i])
					}
				}
			},
		},

		// VALID: Complex valid combinations
		{
			name:  "Chain with list: task >> [task, task]",
			input: "task1 >> [task2, task3]",
			validate: func(t *testing.T, dag *DAG) {
				firstItem := dag.Items[0]
				if len(firstItem.Items) != 1 || firstItem.Items[0].Name != "task1" {
					t.Errorf("Expected task1, got %+v", firstItem.Items)
					return
				}
				if len(firstItem.Children) != 1 {
					t.Errorf("Expected 1 child, got %d", len(firstItem.Children))
					return
				}
				tupleItem := firstItem.Children[0]
				if len(tupleItem.Items) != 2 {
					t.Errorf("Expected 2 items in list, got %d", len(tupleItem.Items))
					return
				}
			},
		},
		{
			name:  "List with chain: [task >> task, task]",
			input: "[task1 >> task2, task3]",
			validate: func(t *testing.T, dag *DAG) {
				firstItem := dag.Items[0]
				if len(firstItem.Items) != 2 {
					t.Errorf("Expected 2 items in list, got %d", len(firstItem.Items))
					return
				}
				// First element should be a chain (task1 >> task2)
				firstElement := firstItem.Items[0]
				if len(firstElement.Items) != 1 || firstElement.Items[0].Name != "task1" {
					t.Errorf("Expected task1 in first element, got %+v", firstElement.Items)
					return
				}
				if len(firstElement.Children) != 1 {
					t.Errorf("Expected children for chain in list, got %d", len(firstElement.Children))
				}
			},
		},
		{
			name:  "Nested lists: [[task, task], [task, task]]",
			input: "[[task1, task2], [task3, task4]]",
			validate: func(t *testing.T, dag *DAG) {
				firstItem := dag.Items[0]
				if len(firstItem.Items) != 2 {
					t.Errorf("Expected 2 items in outer list, got %d", len(firstItem.Items))
					return
				}
				// Each item should be a list with 2 elements
				for i := 0; i < 2; i++ {
					if len(firstItem.Items[i].Items) != 2 {
						t.Errorf("Expected 2 items in inner list %d, got %d", i, len(firstItem.Items[i].Items))
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Parser panicked unexpectedly for input '%s': %v", tt.input, r)
				}
			}()

			dag, err := parseInput(tt.input)
			if err != nil {
				t.Errorf("Unexpected error for valid input '%s': %v", tt.input, err)
				return
			}
			if dag == nil {
				t.Errorf("Expected non-nil DAG for valid input '%s'", tt.input)
				return
			}
			if tt.validate != nil {
				tt.validate(t, dag)
			}
		})
	}
}

func TestParseInputInvalidGrammar(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		reason string
	}{
		// INVALID: According to grammar - empty brackets not allowed
		{
			name:   "Empty brackets [] - violates grammar",
			input:  "[]",
			reason: "Grammar requires non_single_expression_list (min 2 expressions)",
		},

		// INVALID: According to grammar - single element in brackets not allowed
		{
			name:   "Single element in brackets [task] - violates grammar",
			input:  "[task1]",
			reason: "Grammar requires non_single_expression_list (min 2 expressions)",
		},
		{
			name:   "Single nested element [[task]] - violates grammar",
			input:  "[[task1]]",
			reason: "Inner brackets contain only 1 expression, violates non_single_expression_list",
		},
		{
			name:   "Single chain in brackets [task >> task] - violates grammar",
			input:  "[task1 >> task2]",
			reason: "Grammar requires non_single_expression_list (min 2 expressions)",
		},

		// INVALID: According to grammar - single > instead of >>
		{
			name:   "Single > operator - violates grammar",
			input:  "task1 > task2",
			reason: "Grammar specifies >> operator, not single >",
		},
		{
			name:   "Mixed > and >> - violates grammar",
			input:  "task1 > task2 >> task3",
			reason: "Grammar only allows >> operator",
		},
		{
			name:   "Leading >> operator - violates grammar",
			input:  ">> task1",
			reason: "Chain operator cannot start expression",
		},

		// INVALID: Comma without brackets
		{
			name:   "Comma without brackets - violates grammar",
			input:  "task1, task2",
			reason: "Comma only allowed within brackets in non_single_expression_list",
		},
		{
			name:   "Comma in chain - violates grammar",
			input:  "task1 >> task2, task3",
			reason: "Comma only allowed within brackets",
		},

		// INVALID: Malformed bracket structures
		{
			name:   "Unclosed bracket - violates grammar",
			input:  "[task1, task2",
			reason: "Brackets must be properly closed",
		},
		{
			name:   "Extra closing bracket - violates grammar",
			input:  "task1]",
			reason: "Unmatched closing bracket",
		},
		{
			name:   "Trailing comma in brackets - violates grammar",
			input:  "[task1, task2,]",
			reason: "Trailing comma creates empty expression",
		},
		{
			name:   "Leading comma in brackets - violates grammar",
			input:  "[,task1, task2]",
			reason: "Leading comma creates empty expression",
		},

		// INVALID: Characters not in grammar
		{
			name:   "Special characters - violates grammar",
			input:  "task1 $ task2",
			reason: "Special characters not defined in grammar",
		},
		{
			name:   "Parentheses - violates grammar",
			input:  "(task1, task2)",
			reason: "Grammar only allows square brackets",
		},
		{
			name:   "Curly braces - violates grammar",
			input:  "{task1, task2}",
			reason: "Grammar only allows square brackets",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Logf("Parser panicked as expected for invalid input '%s': %v", tt.input, r)
				}
			}()

			_, err := parseInput(tt.input)
			if err != nil {
				t.Logf("Parser returned error as expected for invalid input '%s': %v", tt.input, err)
			} else {
				t.Errorf("Parser accepted invalid input '%s' (reason: %s) - should have returned error", tt.input, tt.reason)
			}
		})
	}
}

func TestParseInputParserLimitations(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		validate func(*testing.T, *DAG)
	}{
		// Cases where parser correctly rejects invalid grammar
		{
			name:  "Parser correctly rejects empty brackets [] per grammar",
			input: "[]",
			validate: func(t *testing.T, dag *DAG) {
				t.Error("This should not be reached as parser should return error")
			},
		},
		{
			name:  "Parser correctly rejects single item in brackets per grammar",
			input: "[task1]",
			validate: func(t *testing.T, dag *DAG) {
				t.Error("This should not be reached as parser should return error")
			},
		},
		{
			name:  "Parser correctly rejects single > operator per grammar",
			input: "task1 > task2",
			validate: func(t *testing.T, dag *DAG) {
				t.Error("This should not be reached as parser should return error")
			},
		},
		{
			name:  "Parser correctly rejects commas outside brackets per grammar",
			input: "task1, task2",
			validate: func(t *testing.T, dag *DAG) {
				t.Error("This should not be reached as parser should return error")
			},
		},
		{
			name:  "Parser correctly rejects trailing commas per grammar",
			input: "[task1, task2,]",
			validate: func(t *testing.T, dag *DAG) {
				t.Error("This should not be reached as parser should return error")
			},
		},
		{
			name:  "Parser correctly rejects nested single items per grammar",
			input: "[[task1]]",
			validate: func(t *testing.T, dag *DAG) {
				t.Error("This should not be reached as parser should return error")
			},
		},
		{
			name:  "Parser handles whitespace flexibly",
			input: "   task1   >>   task2   ",
			validate: func(t *testing.T, dag *DAG) {
				// Verify parser strips whitespace properly
				firstItem := dag.Items[0]
				if len(firstItem.Items) != 1 || firstItem.Items[0].Name != "task1" {
					t.Errorf("Expected task1, got %+v", firstItem.Items)
				}
				t.Log("Parser handles whitespace well, which is good implementation choice")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Check if this test expects an error based on the name
			expectsError := strings.Contains(tt.name, "correctly rejects")

			if expectsError {
				_, err := parseInput(tt.input)
				if err == nil {
					t.Errorf("Expected error for invalid input '%s', but no error occurred", tt.input)
				} else {
					t.Logf("Parser correctly returned error for invalid input '%s': %v", tt.input, err)
				}
			} else {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Parser panicked unexpectedly for input '%s': %v", tt.input, r)
					}
				}()

				dag, err := parseInput(tt.input)
				if err != nil {
					t.Errorf("Unexpected error for valid input '%s': %v", tt.input, err)
					return
				}
				if dag == nil {
					t.Errorf("Expected non-nil DAG for input '%s'", tt.input)
					return
				}
				if tt.validate != nil {
					tt.validate(t, dag)
				}
			}
		})
	}
}

func TestParseInputStrictGrammarValidation(t *testing.T) {
	// These test cases are designed to fail, showing where parser
	// differs from the strict formal grammar specification

	tests := []struct {
		name        string
		input       string
		shouldError bool
		reason      string
	}{
		{
			name:        "Empty brackets should be rejected by strict grammar",
			input:       "[]",
			shouldError: true,
			reason:      "Grammar requires non_single_expression_list with min 2 expressions",
		},
		{
			name:        "Single item in brackets should be rejected by strict grammar",
			input:       "[task1]",
			shouldError: true,
			reason:      "Grammar requires non_single_expression_list with min 2 expressions",
		},
		{
			name:        "Comma outside brackets should be rejected by strict grammar",
			input:       "task1, task2",
			shouldError: true,
			reason:      "Grammar only allows comma within brackets",
		},
		{
			name:        "Single > should be rejected by strict grammar",
			input:       "task1 > task2",
			shouldError: true,
			reason:      "Grammar only specifies >> operator",
		},
		{
			name:        "Special characters should return error",
			input:       "task1 $ task2",
			shouldError: true,
			reason:      "Grammar doesn't include special characters",
		},
		{
			name:        "Unmatched brackets should cause error",
			input:       "task1]",
			shouldError: true,
			reason:      "Grammar requires matched brackets",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Parser unexpectedly panicked for input '%s': %v", tt.input, r)
				}
			}()

			_, err := parseInput(tt.input)
			if err != nil {
				if tt.shouldError {
					t.Logf("Parser correctly returned error for input '%s': %v", tt.input, err)
				} else {
					t.Errorf("Unexpected error for input '%s': %v", tt.input, err)
				}
			} else {
				if tt.shouldError {
					t.Errorf("Parser should have returned error for input '%s' (%s)", tt.input, tt.reason)
				} else {
					t.Logf("Parser accepts input '%s' but strict grammar would reject (%s)", tt.input, tt.reason)
				}
			}
		})
	}
}

func TestParseInputEdgeCases(t *testing.T) {
	edgeCaseTests := []struct {
		name     string
		input    string
		validate func(*testing.T, *DAG)
	}{
		{
			name:  "Tab characters - parser strips them",
			input: "task1\t>>\ttask2",
			validate: func(t *testing.T, dag *DAG) {
				// Parser strips tabs and processes normally
				firstItem := dag.Items[0]
				if len(firstItem.Items) != 1 || firstItem.Items[0].Name != "task1" {
					t.Errorf("Expected task1, got %+v", firstItem.Items)
				}
				t.Log("Parser handles tab characters by stripping them")
			},
		},
		{
			name:  "Newline characters - parser strips them",
			input: "task1\n>>\ntask2",
			validate: func(t *testing.T, dag *DAG) {
				// Parser strips newlines and processes normally
				firstItem := dag.Items[0]
				if len(firstItem.Items) != 1 || firstItem.Items[0].Name != "task1" {
					t.Errorf("Expected task1, got %+v", firstItem.Items)
				}
				t.Log("Parser handles newline characters by stripping them")
			},
		},

		// Invalid characters - should cause parser to return error
		{
			name:  "Invalid characters should return error",
			input: "task1 @ task2",
			validate: func(t *testing.T, dag *DAG) {
				t.Error("Parser should return error for invalid characters")
			},
		},

		{
			name:  "Numbers in identifier should be accepted",
			input: "task123 >> task456",
			validate: func(t *testing.T, dag *DAG) {
				firstItem := dag.Items[0]
				if len(firstItem.Items) != 1 || firstItem.Items[0].Name != "task123" {
					t.Errorf("Expected task123, got %+v", firstItem.Items)
				}
			},
		},
		{
			name:  "Underscore and hyphen in identifier - should be accepted",
			input: "[task-1, task_2]",
			validate: func(t *testing.T, dag *DAG) {
				firstItem := dag.Items[0]
				if len(firstItem.Items) != 2 {
					t.Errorf("Expected 2 items in list, got %d", len(firstItem.Items))
					return
				}
				if len(firstItem.Items[0].Items) != 1 || firstItem.Items[0].Items[0].Name != "task-1" {
					t.Errorf("Expected first item to be 'task-1', got %+v", firstItem.Items[0])
				}
				if len(firstItem.Items[1].Items) != 1 || firstItem.Items[1].Items[0].Name != "task_2" {
					t.Errorf("Expected second item to be 'task_2', got %+v", firstItem.Items[1])
				}
			},
		},

		// Performance and stress tests
		{
			name:  "Many tasks in sequence",
			input: "task1 >> task2 >> task3 >> task4 >> task5 >> task6 >> task7 >> task8 >> task9 >> task10",
			validate: func(t *testing.T, dag *DAG) {
				current := dag.Items[0]
				count := 0
				for {
					if len(current.Items) != 1 {
						t.Errorf("Expected 1 item at depth %d, got %d", count, len(current.Items))
						break
					}
					expectedName := fmt.Sprintf("task%d", count+1)
					if current.Items[0].Name != expectedName {
						t.Errorf("Expected %s at depth %d, got %s", expectedName, count, current.Items[0].Name)
					}
					count++
					if len(current.Children) == 0 {
						break
					}
					current = current.Children[0]
				}
				if count != 10 {
					t.Errorf("Expected 10 tasks in sequence, got %d", count)
				}
			},
		},
		{
			name:  "Many parallel tasks",
			input: "[task1, task2, task3, task4, task5, task6, task7, task8, task9, task10]",
			validate: func(t *testing.T, dag *DAG) {
				firstItem := dag.Items[0]
				if len(firstItem.Items) != 10 {
					t.Errorf("Expected 10 parallel tasks, got %d", len(firstItem.Items))
					return
				}
				for i := 0; i < 10; i++ {
					expectedName := fmt.Sprintf("task%d", i+1)
					if len(firstItem.Items[i].Items) != 1 || firstItem.Items[i].Items[0].Name != expectedName {
						t.Errorf("Expected %s at position %d, got %+v", expectedName, i, firstItem.Items[i])
					}
				}
			},
		},
	}

	for _, tt := range edgeCaseTests {
		t.Run(tt.name, func(t *testing.T) {
			// For tests that should cause errors, expect the error
			expectsError := strings.Contains(tt.name, "should return error")

			if expectsError {
				_, err := parseInput(tt.input)
				if err == nil {
					t.Errorf("Expected error for input '%s', but no error occurred", tt.input)
				} else {
					t.Logf("Parser correctly returned error for input '%s': %v", tt.input, err)
				}
			} else {
				// For normal edge cases, should not panic
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Parser unexpectedly panicked for input '%s': %v", tt.input, r)
					}
				}()

				dag, err := parseInput(tt.input)
				if err != nil {
					t.Errorf("Unexpected error for valid input '%s': %v", tt.input, err)
					return
				}
				if dag == nil {
					t.Errorf("Expected non-nil DAG for valid input '%s'", tt.input)
					return
				}
				if tt.validate != nil {
					tt.validate(t, dag)
				}
			}
		})
	}
}

func TestParseInputErrorCases(t *testing.T) {
	errorTests := []struct {
		name  string
		input string
	}{
		{"Special symbol", "task1 >> task$2"},
		{"Parentheses", "task1 >> (task2)"},
		{"Curly braces", "task1 >> {task2}"},
		{"Semicolon", "task1; task2"},
		{"Colon", "task1: task2"},
		{"Carriage return", "task1\rtask2"},
		{"Stack underflow", "task1]"},
		{"Multiple closing brackets", "]]]]"},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Logf("Parser panicked for input '%s': %v", tt.input, r)
				}
			}()

			_, err := parseInput(tt.input)
			if err == nil {
				t.Errorf("Expected error or panic for input '%s', but neither occurred", tt.input)
			} else {
				t.Logf("Parser correctly returned error for input '%s': %v", tt.input, err)
			}
		})
	}
}
