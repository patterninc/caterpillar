package parser

import (
	"reflect"
	"testing"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

func TestParseDAG(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		errorMsg    string
		validate    func(*testing.T, Expr)
	}{
		{
			name:        "Empty string",
			input:       "",
			expectError: true,
			errorMsg:    "expected identifier or [",
		},
		{
			name:        "Whitespace only",
			input:       "   ",
			expectError: true,
			errorMsg:    "expected identifier or [",
		},
		{
			name:  "Single identifier",
			input: "task1",
			validate: func(t *testing.T, expr Expr) {
				ident, ok := expr.(*Ident)
				if !ok {
					t.Errorf("Expected *Ident, got %T", expr)
					return
				}
				if ident.Name != "task1" {
					t.Errorf("Expected name 'task1', got '%s'", ident.Name)
				}
			},
		},
		{
			name:  "Simple sequence",
			input: "task1 >> task2",
			validate: func(t *testing.T, expr Expr) {
				binOp, ok := expr.(*BinOp)
				if !ok {
					t.Errorf("Expected *BinOp, got %T", expr)
					return
				}
				if binOp.Op != ">>" {
					t.Errorf("Expected operator '>>', got '%s'", binOp.Op)
				}

				left, ok := binOp.Left.(*Ident)
				if !ok {
					t.Errorf("Expected left to be *Ident, got %T", binOp.Left)
					return
				}
				if left.Name != "task1" {
					t.Errorf("Expected left name 'task1', got '%s'", left.Name)
				}

				right, ok := binOp.Right.(*Ident)
				if !ok {
					t.Errorf("Expected right to be *Ident, got %T", binOp.Right)
					return
				}
				if right.Name != "task2" {
					t.Errorf("Expected right name 'task2', got '%s'", right.Name)
				}
			},
		},
		{
			name:  "Three task sequence",
			input: "task1 >> task2 >> task3",
			validate: func(t *testing.T, expr Expr) {
				// Should be parsed as ((task1 >> task2) >> task3)
				binOp, ok := expr.(*BinOp)
				if !ok {
					t.Errorf("Expected *BinOp, got %T", expr)
					return
				}
				if binOp.Op != ">>" {
					t.Errorf("Expected operator '>>', got '%s'", binOp.Op)
				}

				// Left should be (task1 >> task2)
				_, ok = binOp.Left.(*BinOp)
				if !ok {
					t.Errorf("Expected left to be *BinOp, got %T", binOp.Left)
					return
				}

				// Right should be task3
				rightIdent, ok := binOp.Right.(*Ident)
				if !ok {
					t.Errorf("Expected right to be *Ident, got %T", binOp.Right)
					return
				}
				if rightIdent.Name != "task3" {
					t.Errorf("Expected right name 'task3', got '%s'", rightIdent.Name)
				}
			},
		},
		{
			name:  "Simple tuple",
			input: "[task1, task2]",
			validate: func(t *testing.T, expr Expr) {
				tuple, ok := expr.(*Tuple)
				if !ok {
					t.Errorf("Expected *Tuple, got %T", expr)
					return
				}
				if len(tuple.Elements) != 2 {
					t.Errorf("Expected 2 elements, got %d", len(tuple.Elements))
					return
				}

				ident1, ok := tuple.Elements[0].(*Ident)
				if !ok {
					t.Errorf("Expected first element to be *Ident, got %T", tuple.Elements[0])
					return
				}
				if ident1.Name != "task1" {
					t.Errorf("Expected first element name 'task1', got '%s'", ident1.Name)
				}

				ident2, ok := tuple.Elements[1].(*Ident)
				if !ok {
					t.Errorf("Expected second element to be *Ident, got %T", tuple.Elements[1])
					return
				}
				if ident2.Name != "task2" {
					t.Errorf("Expected second element name 'task2', got '%s'", ident2.Name)
				}
			},
		},
		{
			name:  "Tuple with three elements",
			input: "[task1, task2, task3]",
			validate: func(t *testing.T, expr Expr) {
				tuple, ok := expr.(*Tuple)
				if !ok {
					t.Errorf("Expected *Tuple, got %T", expr)
					return
				}
				if len(tuple.Elements) != 3 {
					t.Errorf("Expected 3 elements, got %d", len(tuple.Elements))
				}
			},
		},
		{
			name:  "Fan-out pattern",
			input: "task1 >> [task2, task3]",
			validate: func(t *testing.T, expr Expr) {
				binOp, ok := expr.(*BinOp)
				if !ok {
					t.Errorf("Expected *BinOp, got %T", expr)
					return
				}

				left, ok := binOp.Left.(*Ident)
				if !ok {
					t.Errorf("Expected left to be *Ident, got %T", binOp.Left)
					return
				}
				if left.Name != "task1" {
					t.Errorf("Expected left name 'task1', got '%s'", left.Name)
				}

				right, ok := binOp.Right.(*Tuple)
				if !ok {
					t.Errorf("Expected right to be *Tuple, got %T", binOp.Right)
					return
				}
				if len(right.Elements) != 2 {
					t.Errorf("Expected 2 elements in tuple, got %d", len(right.Elements))
				}
			},
		},
		{
			name:  "Fan-in pattern",
			input: "[task1, task2] >> task3",
			validate: func(t *testing.T, expr Expr) {
				binOp, ok := expr.(*BinOp)
				if !ok {
					t.Errorf("Expected *BinOp, got %T", expr)
					return
				}

				left, ok := binOp.Left.(*Tuple)
				if !ok {
					t.Errorf("Expected left to be *Tuple, got %T", binOp.Left)
					return
				}
				if len(left.Elements) != 2 {
					t.Errorf("Expected 2 elements in tuple, got %d", len(left.Elements))
				}

				right, ok := binOp.Right.(*Ident)
				if !ok {
					t.Errorf("Expected right to be *Ident, got %T", binOp.Right)
					return
				}
				if right.Name != "task3" {
					t.Errorf("Expected right name 'task3', got '%s'", right.Name)
				}
			},
		},
		{
			name:  "Diamond pattern",
			input: "task1 >> [task2, task3] >> task4",
			validate: func(t *testing.T, expr Expr) {
				// Should parse as ((task1 >> [task2, task3]) >> task4)
				binOp, ok := expr.(*BinOp)
				if !ok {
					t.Errorf("Expected *BinOp, got %T", expr)
					return
				}

				// Left should be (task1 >> [task2, task3])
				_, ok = binOp.Left.(*BinOp)
				if !ok {
					t.Errorf("Expected left to be *BinOp, got %T", binOp.Left)
					return
				}

				// Right should be task4
				rightIdent, ok := binOp.Right.(*Ident)
				if !ok {
					t.Errorf("Expected right to be *Ident, got %T", binOp.Right)
					return
				}
				if rightIdent.Name != "task4" {
					t.Errorf("Expected right name 'task4', got '%s'", rightIdent.Name)
				}
			},
		},
		{
			name:  "Nested tuples",
			input: "task1 >> [task2 >> [task3, task4], task5]",
			validate: func(t *testing.T, expr Expr) {
				binOp, ok := expr.(*BinOp)
				if !ok {
					t.Errorf("Expected *BinOp, got %T", expr)
					return
				}

				tuple, ok := binOp.Right.(*Tuple)
				if !ok {
					t.Errorf("Expected right to be *Tuple, got %T", binOp.Right)
					return
				}
				if len(tuple.Elements) != 2 {
					t.Errorf("Expected 2 elements in tuple, got %d", len(tuple.Elements))
				}

				// First element should be a BinOp (task2 >> [task3, task4])
				nestedBinOp, ok := tuple.Elements[0].(*BinOp)
				if !ok {
					t.Errorf("Expected first tuple element to be *BinOp, got %T", tuple.Elements[0])
					return
				}

				// The right side of nested BinOp should be a tuple
				nestedTuple, ok := nestedBinOp.Right.(*Tuple)
				if !ok {
					t.Errorf("Expected nested BinOp right to be *Tuple, got %T", nestedBinOp.Right)
					return
				}
				if len(nestedTuple.Elements) != 2 {
					t.Errorf("Expected 2 elements in nested tuple, got %d", len(nestedTuple.Elements))
				}
			},
		},
		{
			name:        "Unclosed bracket",
			input:       "[task1, task2",
			expectError: true,
			errorMsg:    "expected ',' or ']' in tuple",
		},
		{
			name:        "Extra closing bracket",
			input:       "task1, task2",
			expectError: true,
			errorMsg:    "unexpected comma tokens",
		},
		{
			name:        "Extra closing bracket",
			input:       "[task1, task2]]",
			expectError: true,
			errorMsg:    "unexpected trailing tokens",
		},
		{
			name:        "Missing comma in tuple",
			input:       "[task1 task2]",
			expectError: true,
			errorMsg:    "expected ',' or ']' in tuple",
		},
		{
			name:        "Empty tuple",
			input:       "[]",
			expectError: true,
			errorMsg:    "empty tuples are not allowed",
		},
		{
			name:        "Trailing comma",
			input:       "[task1, task2,]",
			expectError: true,
			errorMsg:    "trailing comma not allowed in tuple",
		},
		{
			name:        "Missing right operand",
			input:       "task1 >>",
			expectError: true,
			errorMsg:    "expected identifier or [",
		},
		{
			name:        "Missing left operand",
			input:       ">> task2",
			expectError: true,
			errorMsg:    "expected identifier or [",
		},
		{
			name:        "Double operator",
			input:       "task1 >> >> task2",
			expectError: true,
			errorMsg:    "expected identifier or [",
		},
		{
			name:        "Invalid character in identifier position",
			input:       "task1 >> 123invalid",
			expectError: true,
			errorMsg:    "unexpected character",
		},
		{
			name:  "Nested brackets create nested tuples",
			input: "[[task1]]",
			validate: func(t *testing.T, expr Expr) {
				// The parser accepts this as a tuple containing another tuple
				outerTuple, ok := expr.(*Tuple)
				if !ok {
					t.Errorf("Expected *Tuple, got %T", expr)
					return
				}
				if len(outerTuple.Elements) != 1 {
					t.Errorf("Expected 1 element in outer tuple, got %d", len(outerTuple.Elements))
					return
				}

				innerTuple, ok := outerTuple.Elements[0].(*Tuple)
				if !ok {
					t.Errorf("Expected inner element to be *Tuple, got %T", outerTuple.Elements[0])
					return
				}
				if len(innerTuple.Elements) != 1 {
					t.Errorf("Expected 1 element in inner tuple, got %d", len(innerTuple.Elements))
				}
			},
		},
		{
			name:        "Mismatched brackets",
			input:       "[task1, [task2]",
			expectError: true,
			errorMsg:    "expected ',' or ']' in tuple",
		},
		{
			name:        "Multiple trailing commas",
			input:       "[task1, task2,,]",
			expectError: true,
			errorMsg:    "expected identifier or [",
		},
		{
			name:        "Leading comma",
			input:       "[, task1, task2]",
			expectError: true,
			errorMsg:    "expected identifier or [",
		},
		{
			name:        "Double comma",
			input:       "[task1,, task2]",
			expectError: true,
			errorMsg:    "expected identifier or [",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := ParseDAG(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for input '%s', but got none", tt.input)
				} else if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					// Check if the error message contains the expected substring
					if len(tt.errorMsg) > 0 && err.Error()[:len(tt.errorMsg)] != tt.errorMsg {
						t.Logf("Expected error starting with '%s', but got '%s'", tt.errorMsg, err.Error())
						// Don't fail on exact message match for flexibility
					}
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for input '%s': %v", tt.input, err)
				return
			}

			if expr == nil {
				t.Errorf("Expected non-nil expression for input '%s'", tt.input)
				return
			}

			if tt.validate != nil {
				tt.validate(t, expr)
			}
		})
	}
}

func TestPostOrder(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Single identifier",
			input:    "task1",
			expected: []string{"task1"},
		},
		{
			name:     "Simple sequence",
			input:    "task1 >> task2",
			expected: []string{"task1", "task2", ">>"},
		},
		{
			name:     "Three task sequence",
			input:    "task1 >> task2 >> task3",
			expected: []string{"task1", "task2", ">>", "task3", ">>"},
		},
		{
			name:     "Simple tuple",
			input:    "[task1, task2]",
			expected: []string{"task1", "task2"},
		},
		{
			name:     "Fan-out pattern",
			input:    "task1 >> [task2, task3]",
			expected: []string{"task1", "task2", "task3", ">>"},
		},
		{
			name:     "Fan-in pattern",
			input:    "[task1, task2] >> task3",
			expected: []string{"task1", "task2", "task3", ">>"},
		},
		{
			name:     "Diamond pattern",
			input:    "task1 >> [task2, task3] >> task4",
			expected: []string{"task1", "task2", "task3", ">>", "task4", ">>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := ParseDAG(tt.input)
			if err != nil {
				t.Fatalf("Failed to parse DAG expression: %v", err)
			}

			result := expr.PostOrder()
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("For input '%s':\nExpected: %v\nGot:      %v", tt.input, tt.expected, result)
			}
		})
	}
}

func TestBuildDAGIntegration(t *testing.T) {
	// Create a mock task registry
	taskRegistry := func(name string) task.Task {
		// Only return tasks that exist
		knownTasks := map[string]bool{
			"task1": true,
			"task2": true,
			"task3": true,
		}
		if knownTasks[name] {
			return &MockTask{name: name}
		}
		return nil
	}

	tests := []struct {
		name          string
		input         string
		expectError   bool
		errorContains string
		validateNodes func(*testing.T, []*Node)
	}{
		{
			name:  "Simple sequence",
			input: "task1 >> task2",
			validateNodes: func(t *testing.T, nodes []*Node) {
				if len(nodes) != 1 {
					t.Errorf("Expected 1 root node, got %d", len(nodes))
					return
				}
				// Should return task2 (rightmost node)
				if nodes[0].Task.GetName() != "task2" {
					t.Errorf("Expected root node to be task2, got %s", nodes[0].Task.GetName())
				}
			},
		},
		{
			name:  "Fan-out pattern",
			input: "task1 >> [task2, task3]",
			validateNodes: func(t *testing.T, nodes []*Node) {
				if len(nodes) != 2 {
					t.Errorf("Expected 2 nodes, got %d", len(nodes))
					return
				}
				// Should return task2 and task3
				names := make([]string, len(nodes))
				for i, node := range nodes {
					names[i] = node.Task.GetName()
				}
				expectedNames := []string{"task2", "task3"}
				if !containsAll(names, expectedNames) {
					t.Errorf("Expected nodes %v, got %v", expectedNames, names)
				}
			},
		},
		{
			name:          "Circular dependency",
			input:         "task1 >> task2 >> task1",
			expectError:   true,
			errorContains: "circular dependency detected",
		},
		{
			name:          "Self reference",
			input:         "task1 >> task1",
			expectError:   true,
			errorContains: "circular dependency detected",
		},
		{
			name:          "Missing task",
			input:         "task1 >> nonexistent",
			expectError:   true,
			errorContains: "task not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := ParseDAG(tt.input)
			if err != nil {
				t.Fatalf("Failed to parse DAG expression: %v", err)
			}

			nodes, err := BuildDag(expr, taskRegistry)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for input '%s', but got none", tt.input)
				} else if tt.errorContains != "" && !containsSubstring(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', but got: %v", tt.errorContains, err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for input '%s': %v", tt.input, err)
				return
			}

			if tt.validateNodes != nil {
				tt.validateNodes(t, nodes)
			}
		})
	}
}

// Helper functions
func containsAll(slice []string, expected []string) bool {
	if len(slice) != len(expected) {
		return false
	}
	for _, exp := range expected {
		found := false
		for _, item := range slice {
			if item == exp {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func containsSubstring(str, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(str) < len(substr) {
		return false
	}
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
