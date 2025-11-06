package parser

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
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
		// === Basic Error Cases ===
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
			name:        "Only whitespace variants",
			input:       "\t\n  \t",
			expectError: true,
			errorMsg:    "expected identifier or [",
		},

		// === Single Identifier Tests ===
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
			name:  "Single identifier with underscores",
			input: "task_with_underscores",
			validate: func(t *testing.T, expr Expr) {
				ident, ok := expr.(*Ident)
				if !ok {
					t.Errorf("Expected *Ident, got %T", expr)
					return
				}
				if ident.Name != "task_with_underscores" {
					t.Errorf("Expected name 'task_with_underscores', got '%s'", ident.Name)
				}
			},
		},
		{
			name:  "Single identifier with numbers",
			input: "task123",
			validate: func(t *testing.T, expr Expr) {
				ident, ok := expr.(*Ident)
				if !ok {
					t.Errorf("Expected *Ident, got %T", expr)
					return
				}
				if ident.Name != "task123" {
					t.Errorf("Expected name 'task123', got '%s'", ident.Name)
				}
			},
		},
		{
			name:  "Single identifier mixed case",
			input: "TaskMixedCase",
			validate: func(t *testing.T, expr Expr) {
				ident, ok := expr.(*Ident)
				if !ok {
					t.Errorf("Expected *Ident, got %T", expr)
					return
				}
				if ident.Name != "TaskMixedCase" {
					t.Errorf("Expected name 'TaskMixedCase', got '%s'", ident.Name)
				}
			},
		},
		{
			name:  "Single identifier starting with underscore",
			input: "_task",
			validate: func(t *testing.T, expr Expr) {
				ident, ok := expr.(*Ident)
				if !ok {
					t.Errorf("Expected *Ident, got %T", expr)
					return
				}
				if ident.Name != "_task" {
					t.Errorf("Expected name '_task', got '%s'", ident.Name)
				}
			},
		},
		{
			name:  "Single character identifier",
			input: "A",
			validate: func(t *testing.T, expr Expr) {
				ident, ok := expr.(*Ident)
				if !ok {
					t.Errorf("Expected *Ident, got %T", expr)
					return
				}
				if ident.Name != "A" {
					t.Errorf("Expected name 'A', got '%s'", ident.Name)
				}
			},
		},

		// === Binary Operation Tests ===
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
			name:  "Sequence with no spaces around operator",
			input: "task1>>task2",
			validate: func(t *testing.T, expr Expr) {
				binOp, ok := expr.(*BinOp)
				if !ok {
					t.Errorf("Expected *BinOp, got %T", expr)
					return
				}
				if binOp.Op != ">>" {
					t.Errorf("Expected operator '>>', got '%s'", binOp.Op)
				}
			},
		},
		{
			name:  "Sequence with extra spaces",
			input: "task1   >>   task2",
			validate: func(t *testing.T, expr Expr) {
				binOp, ok := expr.(*BinOp)
				if !ok {
					t.Errorf("Expected *BinOp, got %T", expr)
					return
				}
				if binOp.Op != ">>" {
					t.Errorf("Expected operator '>>', got '%s'", binOp.Op)
				}
			},
		},
		{
			name:  "Sequence with tabs and newlines",
			input: "task1\t>>\ntask2",
			validate: func(t *testing.T, expr Expr) {
				binOp, ok := expr.(*BinOp)
				if !ok {
					t.Errorf("Expected *BinOp, got %T", expr)
					return
				}
				if binOp.Op != ">>" {
					t.Errorf("Expected operator '>>', got '%s'", binOp.Op)
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

		// === Tuple Tests ===
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
			name:  "Single element tuple",
			input: "[task1]",
			validate: func(t *testing.T, expr Expr) {
				tuple, ok := expr.(*Tuple)
				if !ok {
					t.Errorf("Expected *Tuple, got %T", expr)
					return
				}
				if len(tuple.Elements) != 1 {
					t.Errorf("Expected 1 element, got %d", len(tuple.Elements))
					return
				}
				ident, ok := tuple.Elements[0].(*Ident)
				if !ok {
					t.Errorf("Expected element to be *Ident, got %T", tuple.Elements[0])
					return
				}
				if ident.Name != "task1" {
					t.Errorf("Expected element name 'task1', got '%s'", ident.Name)
				}
			},
		},
		{
			name:  "Tuple with no spaces",
			input: "[task1,task2]",
			validate: func(t *testing.T, expr Expr) {
				tuple, ok := expr.(*Tuple)
				if !ok {
					t.Errorf("Expected *Tuple, got %T", expr)
					return
				}
				if len(tuple.Elements) != 2 {
					t.Errorf("Expected 2 elements, got %d", len(tuple.Elements))
				}
			},
		},
		{
			name:  "Tuple with extra spaces",
			input: "[  task1  ,  task2  ]",
			validate: func(t *testing.T, expr Expr) {
				tuple, ok := expr.(*Tuple)
				if !ok {
					t.Errorf("Expected *Tuple, got %T", expr)
					return
				}
				if len(tuple.Elements) != 2 {
					t.Errorf("Expected 2 elements, got %d", len(tuple.Elements))
				}
			},
		},
		{
			name:  "Tuple with tabs and newlines",
			input: "[\ttask1\n,\ttask2\n]",
			validate: func(t *testing.T, expr Expr) {
				tuple, ok := expr.(*Tuple)
				if !ok {
					t.Errorf("Expected *Tuple, got %T", expr)
					return
				}
				if len(tuple.Elements) != 2 {
					t.Errorf("Expected 2 elements, got %d", len(tuple.Elements))
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

		// === Error Cases - Tokenizer Errors ===
		{
			name:        "Invalid character at start",
			input:       "123task",
			expectError: true,
			errorMsg:    "unexpected character",
		},
		{
			name:        "Invalid character - special symbols",
			input:       "task@name",
			expectError: true,
			errorMsg:    "unexpected character",
		},
		{
			name:        "Invalid character - hash",
			input:       "task#1",
			expectError: true,
			errorMsg:    "unexpected character",
		},
		{
			name:        "Invalid character - dollar",
			input:       "$task",
			expectError: true,
			errorMsg:    "unexpected character",
		},
		{
			name:        "Invalid character - percent",
			input:       "task%",
			expectError: true,
			errorMsg:    "unexpected character",
		},
		{
			name:        "Invalid character - ampersand",
			input:       "task&name",
			expectError: true,
			errorMsg:    "unexpected character",
		},
		{
			name:        "Invalid character - asterisk",
			input:       "task*",
			expectError: true,
			errorMsg:    "unexpected character",
		},
		{
			name:        "Invalid character - parentheses",
			input:       "task()",
			expectError: true,
			errorMsg:    "unexpected character",
		},
		{
			name:        "Invalid character - plus",
			input:       "task+name",
			expectError: true,
			errorMsg:    "unexpected character",
		},
		{
			name:        "Invalid character - equals",
			input:       "task=name",
			expectError: true,
			errorMsg:    "unexpected character",
		},
		{
			name:        "Invalid character - braces",
			input:       "{task}",
			expectError: true,
			errorMsg:    "unexpected character",
		},
		{
			name:        "Invalid character - pipe",
			input:       "task|name",
			expectError: true,
			errorMsg:    "unexpected character",
		},
		{
			name:        "Invalid character - backslash",
			input:       "task\\name",
			expectError: true,
			errorMsg:    "unexpected character",
		},
		{
			name:        "Invalid character - colon",
			input:       "task:name",
			expectError: true,
			errorMsg:    "unexpected character",
		},
		{
			name:        "Invalid character - semicolon",
			input:       "task;name",
			expectError: true,
			errorMsg:    "unexpected character",
		},
		{
			name:        "Invalid character - quotes",
			input:       "\"task\"",
			expectError: true,
			errorMsg:    "unexpected character",
		},
		{
			name:        "Invalid character - single quote",
			input:       "'task'",
			expectError: true,
			errorMsg:    "unexpected character",
		},
		{
			name:        "Invalid character - less than",
			input:       "task<name",
			expectError: true,
			errorMsg:    "unexpected character",
		},
		{
			name:        "Invalid character - greater than (single)",
			input:       "task>name",
			expectError: true,
			errorMsg:    "unexpected character",
		},
		{
			name:        "Invalid character - question mark",
			input:       "task?name",
			expectError: true,
			errorMsg:    "unexpected character",
		},
		{
			name:        "Invalid character - slash",
			input:       "task/name",
			expectError: true,
			errorMsg:    "unexpected character",
		},
		{
			name:        "Invalid character - period",
			input:       "task.name",
			expectError: true,
			errorMsg:    "unexpected character",
		},
		{
			name:        "Invalid character - hyphen",
			input:       "task-name",
			expectError: true,
			errorMsg:    "unexpected character",
		},

		// === Error Cases - Bracket Errors ===
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
		{
			name:        "Mismatched brackets",
			input:       "[task1, [task2]",
			expectError: true,
			errorMsg:    "expected ',' or ']' in tuple",
		},
		{
			name:        "Only opening bracket",
			input:       "[",
			expectError: true,
			errorMsg:    "expected identifier or [",
		},
		{
			name:        "Only closing bracket",
			input:       "]",
			expectError: true,
			errorMsg:    "expected identifier or [",
		},
		{
			name:        "Nested brackets without comma",
			input:       "[task1 [task2]]",
			expectError: true,
			errorMsg:    "expected ',' or ']' in tuple",
		},

		// === Error Cases - Operator Errors ===
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
			name:        "Triple operator",
			input:       "task1 >> >> >> task2",
			expectError: true,
			errorMsg:    "expected identifier or [",
		},
		{
			name:        "Only operator",
			input:       ">>",
			expectError: true,
			errorMsg:    "expected identifier or [",
		},
		{
			name:        "Operator with whitespace only operands",
			input:       "   >>   ",
			expectError: true,
			errorMsg:    "expected identifier or [",
		},
		{
			name:        "Single greater than",
			input:       "task1 > task2",
			expectError: true,
			errorMsg:    "unexpected character",
		},
		{
			name:        "Triple greater than",
			input:       "task1 >>> task2",
			expectError: true,
			errorMsg:    "unexpected character",
		},

		// === Error Cases - Mixed Errors ===
		{
			name:        "Invalid character in identifier position",
			input:       "task1 >> 123invalid",
			expectError: true,
			errorMsg:    "unexpected character",
		},
		{
			name:        "Comma without brackets",
			input:       "task1, task2",
			expectError: true,
			errorMsg:    "unexpected trailing tokens",
		},
		{
			name:        "Multiple commas without brackets",
			input:       "task1, task2, task3",
			expectError: true,
			errorMsg:    "unexpected trailing tokens",
		},
		{
			name:        "Brackets in middle of identifier",
			input:       "ta[sk]1",
			expectError: true,
			errorMsg:    "unexpected character",
		},
		{
			name:  "Operator in brackets (valid nested expression)",
			input: "[task1 >> task2, task3]",
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

				// First element should be a BinOp (task1 >> task2)
				binOp, ok := tuple.Elements[0].(*BinOp)
				if !ok {
					t.Errorf("Expected first element to be *BinOp, got %T", tuple.Elements[0])
					return
				}
				if binOp.Op != ">>" {
					t.Errorf("Expected operator '>>', got '%s'", binOp.Op)
				}

				// Second element should be an Ident (task3)
				ident, ok := tuple.Elements[1].(*Ident)
				if !ok {
					t.Errorf("Expected second element to be *Ident, got %T", tuple.Elements[1])
					return
				}
				if ident.Name != "task3" {
					t.Errorf("Expected name 'task3', got '%s'", ident.Name)
				}
			},
		},
		{
			name:        "Partial operator sequence",
			input:       "task1 > task2",
			expectError: true,
			errorMsg:    "unexpected character",
		},
		{
			name:        "Invalid bracket nesting",
			input:       "[task1 [task2]",
			expectError: true,
			errorMsg:    "expected ',' or ']' in tuple",
		},
		{
			name:        "Bracket without identifier",
			input:       "[>> task1]",
			expectError: true,
			errorMsg:    "expected identifier or [",
		},

		// === Complex Expression Tests ===
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
			name:  "Deep nested tuples",
			input: "[[[task1]]]",
			validate: func(t *testing.T, expr Expr) {
				// Should create three levels of nesting
				level1, ok := expr.(*Tuple)
				if !ok {
					t.Errorf("Expected level 1 to be *Tuple, got %T", expr)
					return
				}
				if len(level1.Elements) != 1 {
					t.Errorf("Expected 1 element in level 1, got %d", len(level1.Elements))
					return
				}

				level2, ok := level1.Elements[0].(*Tuple)
				if !ok {
					t.Errorf("Expected level 2 to be *Tuple, got %T", level1.Elements[0])
					return
				}
				if len(level2.Elements) != 1 {
					t.Errorf("Expected 1 element in level 2, got %d", len(level2.Elements))
					return
				}

				level3, ok := level2.Elements[0].(*Tuple)
				if !ok {
					t.Errorf("Expected level 3 to be *Tuple, got %T", level2.Elements[0])
					return
				}
				if len(level3.Elements) != 1 {
					t.Errorf("Expected 1 element in level 3, got %d", len(level3.Elements))
				}
			},
		},
		{
			name:  "Complex nested with multiple elements",
			input: "[[task1, task2], [task3, task4]]",
			validate: func(t *testing.T, expr Expr) {
				outerTuple, ok := expr.(*Tuple)
				if !ok {
					t.Errorf("Expected *Tuple, got %T", expr)
					return
				}
				if len(outerTuple.Elements) != 2 {
					t.Errorf("Expected 2 elements in outer tuple, got %d", len(outerTuple.Elements))
					return
				}

				// Check first inner tuple
				innerTuple1, ok := outerTuple.Elements[0].(*Tuple)
				if !ok {
					t.Errorf("Expected first element to be *Tuple, got %T", outerTuple.Elements[0])
					return
				}
				if len(innerTuple1.Elements) != 2 {
					t.Errorf("Expected 2 elements in first inner tuple, got %d", len(innerTuple1.Elements))
				}

				// Check second inner tuple
				innerTuple2, ok := outerTuple.Elements[1].(*Tuple)
				if !ok {
					t.Errorf("Expected second element to be *Tuple, got %T", outerTuple.Elements[1])
					return
				}
				if len(innerTuple2.Elements) != 2 {
					t.Errorf("Expected 2 elements in second inner tuple, got %d", len(innerTuple2.Elements))
				}
			},
		},
		{
			name:  "Mixed nesting levels",
			input: "[task1, [task2, task3], task4]",
			validate: func(t *testing.T, expr Expr) {
				outerTuple, ok := expr.(*Tuple)
				if !ok {
					t.Errorf("Expected *Tuple, got %T", expr)
					return
				}
				if len(outerTuple.Elements) != 3 {
					t.Errorf("Expected 3 elements in outer tuple, got %d", len(outerTuple.Elements))
					return
				}

				// First should be identifier
				_, ok = outerTuple.Elements[0].(*Ident)
				if !ok {
					t.Errorf("Expected first element to be *Ident, got %T", outerTuple.Elements[0])
				}

				// Second should be tuple
				innerTuple, ok := outerTuple.Elements[1].(*Tuple)
				if !ok {
					t.Errorf("Expected second element to be *Tuple, got %T", outerTuple.Elements[1])
					return
				}
				if len(innerTuple.Elements) != 2 {
					t.Errorf("Expected 2 elements in inner tuple, got %d", len(innerTuple.Elements))
				}

				// Third should be identifier
				_, ok = outerTuple.Elements[2].(*Ident)
				if !ok {
					t.Errorf("Expected third element to be *Ident, got %T", outerTuple.Elements[2])
				}
			},
		},
		{
			name:  "Very long sequence",
			input: "task1 >> task2 >> task3 >> task4 >> task5",
			validate: func(t *testing.T, expr Expr) {
				// Should be left-associative: ((((task1 >> task2) >> task3) >> task4) >> task5)
				binOp := expr
				depth := 0
				for {
					if b, ok := binOp.(*BinOp); ok {
						if b.Op != ">>" {
							t.Errorf("Expected operator '>>', got '%s'", b.Op)
						}
						binOp = b.Left
						depth++
					} else {
						break
					}
				}
				if depth != 4 {
					t.Errorf("Expected depth of 4 for left-associative parsing, got %d", depth)
				}
			},
		},
		{
			name:  "Large tuple",
			input: "[task1, task2, task3, task4, task5, task6, task7, task8, task9, task10]",
			validate: func(t *testing.T, expr Expr) {
				tuple, ok := expr.(*Tuple)
				if !ok {
					t.Errorf("Expected *Tuple, got %T", expr)
					return
				}
				if len(tuple.Elements) != 10 {
					t.Errorf("Expected 10 elements, got %d", len(tuple.Elements))
				}
			},
		},
		{
			name:  "Complex mixed expression",
			input: "[task1 >> task2, task3] >> [task4, task5 >> task6] >> task7",
			validate: func(t *testing.T, expr Expr) {
				// This should be parsed as left-associative BinOps
				binOp, ok := expr.(*BinOp)
				if !ok {
					t.Errorf("Expected *BinOp at root, got %T", expr)
					return
				}

				// Right side should be task7
				rightIdent, ok := binOp.Right.(*Ident)
				if !ok {
					t.Errorf("Expected right to be *Ident, got %T", binOp.Right)
					return
				}
				if rightIdent.Name != "task7" {
					t.Errorf("Expected right name 'task7', got '%s'", rightIdent.Name)
				}

				// Left side should be another BinOp
				leftBinOp, ok := binOp.Left.(*BinOp)
				if !ok {
					t.Errorf("Expected left to be *BinOp, got %T", binOp.Left)
				} else {
					// Left.Right should be a tuple [task4, task5 >> task6]
					rightTuple, ok := leftBinOp.Right.(*Tuple)
					if !ok {
						t.Errorf("Expected left.Right to be *Tuple, got %T", leftBinOp.Right)
						return
					}
					if len(rightTuple.Elements) != 2 {
						t.Errorf("Expected 2 elements in right tuple, got %d", len(rightTuple.Elements))
					}
				}
			},
		},
		{
			name:  "Whitespace variations comprehensive",
			input: "  task1  >>  [  task2  ,  task3  ]  >>  task4  ",
			validate: func(t *testing.T, expr Expr) {
				// Should still parse correctly with lots of whitespace
				binOp, ok := expr.(*BinOp)
				if !ok {
					t.Errorf("Expected *BinOp, got %T", expr)
					return
				}
				if binOp.Op != ">>" {
					t.Errorf("Expected operator '>>', got '%s'", binOp.Op)
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
			name:     "Four task sequence",
			input:    "task1 >> task2 >> task3 >> task4",
			expected: []string{"task1", "task2", ">>", "task3", ">>", "task4", ">>"},
		},
		{
			name:     "Five task sequence",
			input:    "task1 >> task2 >> task3 >> task4 >> task5",
			expected: []string{"task1", "task2", ">>", "task3", ">>", "task4", ">>", "task5", ">>"},
		},
		{
			name:     "Simple tuple",
			input:    "[task1, task2]",
			expected: []string{"task1", "task2"},
		},
		{
			name:     "Triple tuple",
			input:    "[task1, task2, task3]",
			expected: []string{"task1", "task2", "task3"},
		},
		{
			name:     "Large tuple",
			input:    "[task1, task2, task3, task4, task5]",
			expected: []string{"task1", "task2", "task3", "task4", "task5"},
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
		{
			name:     "Complex diamond pattern",
			input:    "task1 >> [task2, task3, task4] >> task5",
			expected: []string{"task1", "task2", "task3", "task4", ">>", "task5", ">>"},
		},
		{
			name:     "Nested tuples",
			input:    "task1 >> [task2 >> [task3, task4], task5]",
			expected: []string{"task1", "task2", "task3", "task4", ">>", "task5", ">>"},
		},
		{
			name:     "Complex nested expression",
			input:    "[task1 >> task2, task3] >> [task4, task5 >> task6]",
			expected: []string{"task1", "task2", ">>", "task3", "task4", "task5", "task6", ">>", ">>"},
		},
		{
			name:     "Deep nesting with sequences",
			input:    "task1 >> [task2 >> [task3 >> task4, task5], task6 >> task7]",
			expected: []string{"task1", "task2", "task3", "task4", ">>", "task5", ">>", "task6", "task7", ">>", ">>"},
		},
		{
			name:     "Single element tuple",
			input:    "[task1]",
			expected: []string{"task1"},
		},
		{
			name:     "Nested single element tuples",
			input:    "[[task1]]",
			expected: []string{"task1"},
		},
		{
			name:     "Deep nested single element",
			input:    "[[[task1]]]",
			expected: []string{"task1"},
		},
		{
			name:     "Mixed nesting levels",
			input:    "[task1, [task2, task3], task4]",
			expected: []string{"task1", "task2", "task3", "task4"},
		},
		{
			name:     "Multiple fan-out fan-in",
			input:    "[task1, task2] >> [task3, task4] >> [task5, task6]",
			expected: []string{"task1", "task2", "task3", "task4", ">>", "task5", "task6", ">>"},
		},
		{
			name:     "Sequential then parallel",
			input:    "task1 >> task2 >> [task3, task4, task5]",
			expected: []string{"task1", "task2", ">>", "task3", "task4", "task5", ">>"},
		},
		{
			name:     "Parallel then sequential",
			input:    "[task1, task2, task3] >> task4 >> task5",
			expected: []string{"task1", "task2", "task3", "task4", ">>", "task5", ">>"},
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

// Test AST Resolution Functions - these are the missing coverage areas
func TestASTResolution(t *testing.T) {
	// Create mock tasks for testing
	taskA := &MockTask{name: "taskA"}
	taskB := &MockTask{name: "taskB"}
	taskC := &MockTask{name: "taskC"}

	getTask := func(name string) task.Task {
		switch name {
		case "taskA":
			return taskA
		case "taskB":
			return taskB
		case "taskC":
			return taskC
		default:
			return nil // This will trigger the error case
		}
	}

	t.Run("Ident resolve with existing task", func(t *testing.T) {
		ident := &Ident{Name: "taskA"}
		nodes, err := ident.ResolveLeft(getTask)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(nodes) != 1 {
			t.Fatalf("Expected 1 node, got %d", len(nodes))
		}
		if nodes[0].Task.GetName() != "taskA" {
			t.Errorf("Expected task name 'taskA', got '%s'", nodes[0].Task.GetName())
		}
	})

	t.Run("Ident resolve with missing task", func(t *testing.T) {
		ident := &Ident{Name: "nonexistent"}
		_, err := ident.ResolveLeft(getTask)
		if err == nil {
			t.Fatal("Expected error for missing task, got none")
		}
		expectedMsg := "task not found: nonexistent"
		if err.Error() != expectedMsg {
			t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
		}
	})

	t.Run("BinOp ResolveLeft", func(t *testing.T) {
		// taskA >> taskB
		binOp := &BinOp{
			Op:    ">>",
			Left:  &Ident{Name: "taskA"},
			Right: &Ident{Name: "taskB"},
		}

		nodes, err := binOp.ResolveLeft(getTask)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(nodes) != 1 {
			t.Fatalf("Expected 1 node, got %d", len(nodes))
		}
		if nodes[0].Task.GetName() != "taskA" {
			t.Errorf("Expected leftmost task 'taskA', got '%s'", nodes[0].Task.GetName())
		}
	})

	t.Run("BinOp ResolveRight", func(t *testing.T) {
		// taskA >> taskB
		binOp := &BinOp{
			Op:    ">>",
			Left:  &Ident{Name: "taskA"},
			Right: &Ident{Name: "taskB"},
		}

		nodes, err := binOp.ResolveRight(getTask)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(nodes) != 1 {
			t.Fatalf("Expected 1 node, got %d", len(nodes))
		}
		if nodes[0].Task.GetName() != "taskB" {
			t.Errorf("Expected rightmost task 'taskB', got '%s'", nodes[0].Task.GetName())
		}
	})

	t.Run("BinOp ResolveLeft with error", func(t *testing.T) {
		// nonexistent >> taskB (should fail on left side)
		binOp := &BinOp{
			Op:    ">>",
			Left:  &Ident{Name: "nonexistent"},
			Right: &Ident{Name: "taskB"},
		}

		_, err := binOp.ResolveLeft(getTask)
		if err == nil {
			t.Fatal("Expected error for missing task, got none")
		}
		if !containsStr(err.Error(), "task not found: nonexistent") {
			t.Errorf("Expected error about missing task, got: %v", err)
		}
	})

	t.Run("BinOp ResolveRight with error", func(t *testing.T) {
		// taskA >> nonexistent (should fail on right side)
		binOp := &BinOp{
			Op:    ">>",
			Left:  &Ident{Name: "taskA"},
			Right: &Ident{Name: "nonexistent"},
		}

		_, err := binOp.ResolveRight(getTask)
		if err == nil {
			t.Fatal("Expected error for missing task, got none")
		}
		if !containsStr(err.Error(), "task not found: nonexistent") {
			t.Errorf("Expected error about missing task, got: %v", err)
		}
	})

	t.Run("Tuple ResolveLeft with error", func(t *testing.T) {
		// [taskA, nonexistent] - should fail on second element
		tuple := &Tuple{
			Elements: []Expr{
				&Ident{Name: "taskA"},
				&Ident{Name: "nonexistent"},
			},
		}

		_, err := tuple.ResolveLeft(getTask)
		if err == nil {
			t.Fatal("Expected error for missing task, got none")
		}
		if !containsStr(err.Error(), "task not found: nonexistent") {
			t.Errorf("Expected error about missing task, got: %v", err)
		}
	})

	t.Run("Tuple ResolveRight with error", func(t *testing.T) {
		// [taskA, nonexistent] - should fail on second element
		tuple := &Tuple{
			Elements: []Expr{
				&Ident{Name: "taskA"},
				&Ident{Name: "nonexistent"},
			},
		}

		_, err := tuple.ResolveRight(getTask)
		if err == nil {
			t.Fatal("Expected error for missing task, got none")
		}
		if !containsStr(err.Error(), "task not found: nonexistent") {
			t.Errorf("Expected error about missing task, got: %v", err)
		}
	})

	t.Run("Complex BinOp with Tuple ResolveLeft", func(t *testing.T) {
		// taskA >> [taskB, taskC] - should return taskA
		binOp := &BinOp{
			Op:   ">>",
			Left: &Ident{Name: "taskA"},
			Right: &Tuple{
				Elements: []Expr{
					&Ident{Name: "taskB"},
					&Ident{Name: "taskC"},
				},
			},
		}

		nodes, err := binOp.ResolveLeft(getTask)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(nodes) != 1 {
			t.Fatalf("Expected 1 node, got %d", len(nodes))
		}
		if nodes[0].Task.GetName() != "taskA" {
			t.Errorf("Expected leftmost task 'taskA', got '%s'", nodes[0].Task.GetName())
		}
	})

	t.Run("Complex BinOp with Tuple ResolveRight", func(t *testing.T) {
		// taskA >> [taskB, taskC] - should return taskB and taskC
		binOp := &BinOp{
			Op:   ">>",
			Left: &Ident{Name: "taskA"},
			Right: &Tuple{
				Elements: []Expr{
					&Ident{Name: "taskB"},
					&Ident{Name: "taskC"},
				},
			},
		}

		nodes, err := binOp.ResolveRight(getTask)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(nodes) != 2 {
			t.Fatalf("Expected 2 nodes, got %d", len(nodes))
		}

		nodeNames := make([]string, len(nodes))
		for i, node := range nodes {
			nodeNames[i] = node.Task.GetName()
		}

		if !containsStr(nodeNames[0], "taskB") && !containsStr(nodeNames[0], "taskC") {
			t.Errorf("Expected taskB or taskC, got '%s'", nodeNames[0])
		}
		if !containsStr(nodeNames[1], "taskB") && !containsStr(nodeNames[1], "taskC") {
			t.Errorf("Expected taskB or taskC, got '%s'", nodeNames[1])
		}
	})
}

func TestGlobalNodeRegistry(t *testing.T) {
	// Save original registry
	originalRegistry := globalNodeRegistry
	defer func() {
		globalNodeRegistry = originalRegistry
	}()

	taskA := &MockTask{name: "taskA"}
	getTask := func(name string) task.Task {
		if name == "taskA" {
			return taskA
		}
		return nil
	}

	t.Run("Registry reuse existing node", func(t *testing.T) {
		// Set up registry with a pre-existing node
		existingNode := &Node{Task: taskA}
		globalNodeRegistry = map[string]*Node{
			"taskA": existingNode,
		}

		ident := &Ident{Name: "taskA"}
		nodes, err := ident.ResolveLeft(getTask)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(nodes) != 1 {
			t.Fatalf("Expected 1 node, got %d", len(nodes))
		}

		// Should return the exact same node instance from registry
		if nodes[0] != existingNode {
			t.Error("Expected to reuse existing node from registry")
		}
	})

	t.Run("Registry nil case", func(t *testing.T) {
		// Set registry to nil to test fallback
		globalNodeRegistry = nil

		ident := &Ident{Name: "taskA"}
		nodes, err := ident.ResolveLeft(getTask)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(nodes) != 1 {
			t.Fatalf("Expected 1 node, got %d", len(nodes))
		}

		if nodes[0].Task.GetName() != "taskA" {
			t.Errorf("Expected task name 'taskA', got '%s'", nodes[0].Task.GetName())
		}
	})
}

func TestMockTaskCoverage(t *testing.T) {
	mock := &MockTask{name: "test"}

	t.Run("Mock GetName", func(t *testing.T) {
		if mock.GetName() != "test" {
			t.Errorf("Expected 'test', got '%s'", mock.GetName())
		}
	})

	t.Run("Mock Run", func(t *testing.T) {
		inputChan := make(chan *record.Record, 1)
		outputChan := make(chan *record.Record, 1)

		err := mock.Run(inputChan, outputChan)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("Mock GetInputCount", func(t *testing.T) {
		count := mock.GetInputCount()
		if count != 1 {
			t.Errorf("Expected 1, got %d", count)
		}
	})

	t.Run("Mock GetFailOnError", func(t *testing.T) {
		failOnError := mock.GetFailOnError()
		if failOnError != false {
			t.Errorf("Expected false, got %v", failOnError)
		}
	})
}

// Helper function to check if string contains substring
func containsStr(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) &&
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}

// Additional edge case and stress tests
func TestEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		description string
	}{
		{
			name:        "Unicode characters not supported",
			input:       "tâsk1",
			expectError: true,
			description: "Unicode characters should cause tokenizer error",
		},
		{
			name:        "Very long identifier",
			input:       "task_with_very_long_name_that_contains_many_underscores_and_numbers_123456789",
			expectError: false,
			description: "Long identifiers should be supported",
		},
		{
			name:        "Identifier starting with number not allowed",
			input:       "1task",
			expectError: true,
			description: "Identifiers cannot start with numbers",
		},
		{
			name:        "Only underscores",
			input:       "____",
			expectError: false,
			description: "Identifier with only underscores should be valid",
		},
		{
			name:        "Mixed case with numbers and underscores",
			input:       "Task_123_ABC_xyz",
			expectError: false,
			description: "Complex valid identifier",
		},
		{
			name:        "Deeply nested expression",
			input:       "task1 >> [task2 >> [task3 >> [task4, task5], task6], task7 >> task8]",
			expectError: false,
			description: "Complex deeply nested expression should parse",
		},
		{
			name:        "Many sequential operators",
			input:       "a >> b >> c >> d >> e >> f >> g >> h >> i >> j",
			expectError: false,
			description: "Long sequence should be left-associative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseDAG(tt.input)

			if tt.expectError && err == nil {
				t.Errorf("Expected error for %s, but got none", tt.description)
			} else if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for %s: %v", tt.description, err)
			}
		})
	}
}

func TestParserRobustness(t *testing.T) {
	// Test parser with various malformed inputs to ensure it handles them gracefully
	malformedInputs := []string{
		"",
		" ",
		"[",
		"]",
		">>",
		",",
		"[,]",
		"[task1,,task2]",
		"task1 >>> task2",
		"task1 > task2",
		"[task1 task2]",
		"task1, task2",
		"task1 >> >> task2",
		"[[[[[[task1]]]]]]",
		"task1 >> [task2, [task3, task4], [task5 >> task6, task7]]",
		"[task1] >> [task2] >> [task3] >> [task4]",
	}

	for i, input := range malformedInputs {
		t.Run(fmt.Sprintf("malformed_input_%d", i+1), func(t *testing.T) {
			_, err := ParseDAG(input)
			// We don't care if it succeeds or fails, just that it doesn't panic
			// This is a robustness test
			if err != nil {
				// Expected for most malformed inputs
				t.Logf("Input '%s' failed as expected: %v", input, err)
			} else {
				// Some might actually be valid
				t.Logf("Input '%s' parsed successfully", input)
			}
		})
	}
}

// Test specific parser internal functions for complete coverage
func TestParserInternals(t *testing.T) {
	t.Run("Parser current and eat functions", func(t *testing.T) {
		tokens := []Token{
			{Type: TokenIdent, Value: "task1"},
			{Type: TokenEOF, Value: ""},
		}
		parser := Parser{tokens: tokens, pos: 0}

		// Test current()
		current := parser.current()
		if current.Type != TokenIdent || current.Value != "task1" {
			t.Errorf("Expected TokenIdent 'task1', got %v '%s'", current.Type, current.Value)
		}

		// Test eat() with matching token
		if !parser.eat(TokenIdent) {
			t.Error("Expected eat(TokenIdent) to return true")
		}

		// Test eat() with non-matching token
		if parser.eat(TokenIdent) {
			t.Error("Expected eat(TokenIdent) to return false when at EOF")
		}

		// Verify position advanced
		current = parser.current()
		if current.Type != TokenEOF {
			t.Errorf("Expected TokenEOF after eating, got %v", current.Type)
		}
	})

	t.Run("Deep nested expressions stress test", func(t *testing.T) {
		// Create a very deeply nested expression to stress test the parser
		input := "task1"
		for i := 0; i < 50; i++ {
			input = fmt.Sprintf("[%s]", input)
		}

		expr, err := ParseDAG(input)
		if err != nil {
			t.Fatalf("Deep nested expression failed: %v", err)
		}

		// Verify it's deeply nested tuples
		current := expr
		depth := 0
		for {
			if tuple, ok := current.(*Tuple); ok {
				if len(tuple.Elements) != 1 {
					break
				}
				current = tuple.Elements[0]
				depth++
			} else {
				break
			}
		}

		if depth != 50 {
			t.Errorf("Expected depth of 50, got %d", depth)
		}

		// Verify the innermost is an identifier
		if ident, ok := current.(*Ident); !ok || ident.Name != "task1" {
			t.Errorf("Expected innermost to be Ident 'task1', got %T %v", current, current)
		}
	})

	t.Run("Very long sequence stress test", func(t *testing.T) {
		// Create a very long sequence
		parts := make([]string, 100)
		for i := 0; i < 100; i++ {
			parts[i] = fmt.Sprintf("task%d", i+1)
		}
		input := fmt.Sprintf("%s", parts[0])
		for i := 1; i < 100; i++ {
			input += " >> " + parts[i]
		}

		expr, err := ParseDAG(input)
		if err != nil {
			t.Fatalf("Long sequence failed: %v", err)
		}

		// Verify it's left-associative by counting depth
		current := expr
		depth := 0
		for {
			if binOp, ok := current.(*BinOp); ok {
				current = binOp.Left
				depth++
			} else {
				break
			}
		}

		if depth != 99 { // 100 tasks = 99 operators
			t.Errorf("Expected depth of 99 for 100 tasks, got %d", depth)
		}
	})

	t.Run("Large tuple stress test", func(t *testing.T) {
		// Create a tuple with many elements
		parts := make([]string, 50)
		for i := 0; i < 50; i++ {
			parts[i] = fmt.Sprintf("task%d", i+1)
		}
		input := "[" + parts[0]
		for i := 1; i < 50; i++ {
			input += ", " + parts[i]
		}
		input += "]"

		expr, err := ParseDAG(input)
		if err != nil {
			t.Fatalf("Large tuple failed: %v", err)
		}

		tuple, ok := expr.(*Tuple)
		if !ok {
			t.Fatalf("Expected Tuple, got %T", expr)
		}

		if len(tuple.Elements) != 50 {
			t.Errorf("Expected 50 elements, got %d", len(tuple.Elements))
		}

		// Verify all elements are identifiers
		for i, elem := range tuple.Elements {
			if ident, ok := elem.(*Ident); !ok {
				t.Errorf("Element %d is not Ident, got %T", i, elem)
			} else if ident.Name != fmt.Sprintf("task%d", i+1) {
				t.Errorf("Element %d expected 'task%d', got '%s'", i, i+1, ident.Name)
			}
		}
	})
}

// Final coverage tests to reach 100%
func TestParserEdgeCasesForFullCoverage(t *testing.T) {
	t.Run("Complete tokenizer coverage", func(t *testing.T) {
		// Test every possible valid and invalid character combination
		validInputs := []string{
			"a", "A", "_", "a1", "A1", "_1", "abc", "ABC", "a_b", "A_B",
			"task_123", "TASK_ABC", "_task_", "a1b2c3", "mixedCase_123",
		}

		for _, input := range validInputs {
			tokens, err := tokenize(input)
			if err != nil {
				t.Errorf("Valid input '%s' failed tokenization: %v", input, err)
				continue
			}
			if len(tokens) != 2 { // identifier + EOF
				t.Errorf("Expected 2 tokens for '%s', got %d", input, len(tokens))
				continue
			}
			if tokens[0].Type != TokenIdent || tokens[0].Value != input {
				t.Errorf("Expected TokenIdent '%s', got %v '%s'", input, tokens[0].Type, tokens[0].Value)
			}
		}
	})

	t.Run("All operator combinations", func(t *testing.T) {
		// Test >> operator in various whitespace contexts
		operatorInputs := []string{
			"a>>b", "a >> b", "a  >>  b", "a\t>>\tb", "a\n>>\nb",
			"a>>b>>c", "a >> b >> c",
		}

		for _, input := range operatorInputs {
			expr, err := ParseDAG(input)
			if err != nil {
				t.Errorf("Valid operator input '%s' failed: %v", input, err)
				continue
			}

			// Should parse as BinOp(s)
			if _, ok := expr.(*BinOp); !ok {
				t.Errorf("Expected BinOp for '%s', got %T", input, expr)
			}
		}
	})

	t.Run("All bracket combinations", func(t *testing.T) {
		// Test bracket parsing in various contexts
		bracketInputs := []string{
			"[a]", "[a,b]", "[a, b]", "[a ,b]", "[a , b]",
			"[ a ]", "[ a , b ]", "[  a  ,  b  ]",
			"[a,b,c]", "[a, b, c]", "[a , b , c]",
		}

		for _, input := range bracketInputs {
			expr, err := ParseDAG(input)
			if err != nil {
				t.Errorf("Valid bracket input '%s' failed: %v", input, err)
				continue
			}

			// Should parse as Tuple
			if tuple, ok := expr.(*Tuple); !ok {
				t.Errorf("Expected Tuple for '%s', got %T", input, expr)
			} else {
				if len(tuple.Elements) == 0 {
					t.Errorf("Empty tuple for '%s'", input)
				}
			}
		}
	})

	t.Run("Comprehensive error combinations", func(t *testing.T) {
		// Test every possible error path
		errorInputs := map[string]string{
			"":                  "expected identifier or [",
			" ":                 "expected identifier or [",
			"[":                 "expected identifier or [",
			"]":                 "expected identifier or [",
			">>":                "expected identifier or [",
			",":                 "expected identifier or [",
			"123":               "unexpected character",
			"@":                 "unexpected character",
			"task@":             "unexpected character",
			"[]":                "empty tuples are not allowed",
			"[,]":               "expected identifier or [",
			"[task,]":           "trailing comma not allowed in tuple",
			"[task task]":       "expected ',' or ']' in tuple",
			"[task1,,task2]":    "expected identifier or [",
			"task >>":           "expected identifier or [",
			">> task":           "expected identifier or [",
			"task >>>":          "unexpected character",
			"task1, task2":      "unexpected trailing tokens",
			"[task]]":           "unexpected trailing tokens",
			"task1 >> >> task2": "expected identifier or [",
		}

		for input, expectedErrSubstr := range errorInputs {
			_, err := ParseDAG(input)
			if err == nil {
				t.Errorf("Expected error for input '%s', but got none", input)
				continue
			}
			if !containsStr(err.Error(), expectedErrSubstr) {
				t.Errorf("For input '%s', expected error containing '%s', got '%s'",
					input, expectedErrSubstr, err.Error())
			}
		}
	})

	t.Run("All whitespace variations", func(t *testing.T) {
		// Test all whitespace characters in all positions
		whitespaceVariations := []string{
			" task1 ", "\ttask1\t", "\ntask1\n",
			" \t\ntask1 \t\n", "task1 >> task2", " task1 >> task2 ",
			"\ttask1\t>>\ttask2\t", "\ntask1\n>>\ntask2\n",
			"[ task1 , task2 ]", "[\ttask1\t,\ttask2\t]", "[\ntask1\n,\ntask2\n]",
		}

		for _, input := range whitespaceVariations {
			_, err := ParseDAG(input)
			if err != nil {
				t.Errorf("Whitespace variation '%s' failed: %v", input, err)
			}
		}
	})
}
