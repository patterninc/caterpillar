package parser

import (
	"reflect"
	"testing"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []Token
		hasError bool
	}{
		{
			name:  "Empty string",
			input: "",
			expected: []Token{
				{TokenEOF, ""},
			},
			hasError: false,
		},
		{
			name:  "Whitespace only",
			input: "   \t\n  ",
			expected: []Token{
				{TokenEOF, ""},
			},
			hasError: false,
		},
		{
			name:  "Single identifier",
			input: "task1",
			expected: []Token{
				{TokenIdent, "task1"},
				{TokenEOF, ""},
			},
			hasError: false,
		},
		{
			name:  "Identifier with numbers",
			input: "task123",
			expected: []Token{
				{TokenIdent, "task123"},
				{TokenEOF, ""},
			},
			hasError: false,
		},
		{
			name:  "Identifier with underscores",
			input: "my_task_name",
			expected: []Token{
				{TokenIdent, "my_task_name"},
				{TokenEOF, ""},
			},
			hasError: false,
		},
		{
			name:  "Identifier starting with underscore",
			input: "_private_task",
			expected: []Token{
				{TokenIdent, "_private_task"},
				{TokenEOF, ""},
			},
			hasError: false,
		},
		{
			name:  "Simple sequence",
			input: "task1 >> task2",
			expected: []Token{
				{TokenIdent, "task1"},
				{TokenRShift, ">>"},
				{TokenIdent, "task2"},
				{TokenEOF, ""},
			},
			hasError: false,
		},
		{
			name:  "Tuple with brackets",
			input: "[task1, task2]",
			expected: []Token{
				{TokenLBracket, "["},
				{TokenIdent, "task1"},
				{TokenComma, ","},
				{TokenIdent, "task2"},
				{TokenRBracket, "]"},
				{TokenEOF, ""},
			},
			hasError: false,
		},
		{
			name:  "Complex expression",
			input: "source >> [transform1, transform2] >> sink",
			expected: []Token{
				{TokenIdent, "source"},
				{TokenRShift, ">>"},
				{TokenLBracket, "["},
				{TokenIdent, "transform1"},
				{TokenComma, ","},
				{TokenIdent, "transform2"},
				{TokenRBracket, "]"},
				{TokenRShift, ">>"},
				{TokenIdent, "sink"},
				{TokenEOF, ""},
			},
			hasError: false,
		},
		{
			name:  "No spaces between tokens",
			input: "task1>>[task2,task3]>>task4",
			expected: []Token{
				{TokenIdent, "task1"},
				{TokenRShift, ">>"},
				{TokenLBracket, "["},
				{TokenIdent, "task2"},
				{TokenComma, ","},
				{TokenIdent, "task3"},
				{TokenRBracket, "]"},
				{TokenRShift, ">>"},
				{TokenIdent, "task4"},
				{TokenEOF, ""},
			},
			hasError: false,
		},
		{
			name:  "Mixed whitespace",
			input: "task1 \t>> \n [task2,\ttask3] \n>> task4",
			expected: []Token{
				{TokenIdent, "task1"},
				{TokenRShift, ">>"},
				{TokenLBracket, "["},
				{TokenIdent, "task2"},
				{TokenComma, ","},
				{TokenIdent, "task3"},
				{TokenRBracket, "]"},
				{TokenRShift, ">>"},
				{TokenIdent, "task4"},
				{TokenEOF, ""},
			},
			hasError: false,
		},
		{
			name:  "Nested brackets",
			input: "task1 >> [task2 >> [task3, task4], task5]",
			expected: []Token{
				{TokenIdent, "task1"},
				{TokenRShift, ">>"},
				{TokenLBracket, "["},
				{TokenIdent, "task2"},
				{TokenRShift, ">>"},
				{TokenLBracket, "["},
				{TokenIdent, "task3"},
				{TokenComma, ","},
				{TokenIdent, "task4"},
				{TokenRBracket, "]"},
				{TokenComma, ","},
				{TokenIdent, "task5"},
				{TokenRBracket, "]"},
				{TokenEOF, ""},
			},
			hasError: false,
		},
		{
			name:  "Multiple commas",
			input: "[task1, task2, task3, task4]",
			expected: []Token{
				{TokenLBracket, "["},
				{TokenIdent, "task1"},
				{TokenComma, ","},
				{TokenIdent, "task2"},
				{TokenComma, ","},
				{TokenIdent, "task3"},
				{TokenComma, ","},
				{TokenIdent, "task4"},
				{TokenRBracket, "]"},
				{TokenEOF, ""},
			},
			hasError: false,
		},
		{
			name:     "Invalid character - number at start",
			input:    "1task",
			expected: nil,
			hasError: true,
		},
		{
			name:     "Invalid character - special symbol",
			input:    "task1 @ task2",
			expected: nil,
			hasError: true,
		},
		{
			name:     "Invalid character - hash",
			input:    "task1 # comment",
			expected: nil,
			hasError: true,
		},
		{
			name:     "Invalid character - dollar sign",
			input:    "task1 >> $task2",
			expected: nil,
			hasError: true,
		},
		{
			name:     "Invalid character - parentheses",
			input:    "task1 >> (task2)",
			expected: nil,
			hasError: true,
		},
		{
			name:  "Single greater than (not shift)",
			input: "task1 > task2",
			expected: nil,
			hasError: true,
		},
		{
			name:  "CamelCase identifiers",
			input: "MyTaskName >> AnotherTask",
			expected: []Token{
				{TokenIdent, "MyTaskName"},
				{TokenRShift, ">>"},
				{TokenIdent, "AnotherTask"},
				{TokenEOF, ""},
			},
			hasError: false,
		},
		{
			name:  "Numbers in middle of identifier",
			input: "task1step2 >> final3task",
			expected: []Token{
				{TokenIdent, "task1step2"},
				{TokenRShift, ">>"},
				{TokenIdent, "final3task"},
				{TokenEOF, ""},
			},
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := tokenize(tt.input)
			
			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error for input '%s', but got none", tt.input)
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error for input '%s': %v", tt.input, err)
				return
			}
			
			if !reflect.DeepEqual(tokens, tt.expected) {
				t.Errorf("For input '%s':\nExpected: %+v\nGot:      %+v", tt.input, tt.expected, tokens)
			}
		})
	}
}

func TestTokenizeEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Single >",
			input:       "task1 > task2",
			expectError: true,
			errorMsg:    "unexpected character '>'",
		},
		{
			name:        "Triple >",
			input:       "task1 >>> task2",
			expectError: true,
			errorMsg:    "unexpected character '>'",
		},
		{
			name:        "Unicode character",
			input:       "task1 >> taské",
			expectError: true,
			errorMsg:    "", // Don't check exact error message as it varies by encoding
		},
		{
			name:        "Backslash",
			input:       "task1\\task2",
			expectError: true,
			errorMsg:    "unexpected character '\\'",
		},
		{
			name:        "Quote character",
			input:       "task1 >> \"task2\"",
			expectError: true,
			errorMsg:    "unexpected character '\"'",
		},
		{
			name:        "Semicolon",
			input:       "task1; task2",
			expectError: true,
			errorMsg:    "unexpected character ';'",
		},
		{
			name:        "Dot",
			input:       "task1.subtask",
			expectError: true,
			errorMsg:    "unexpected character '.'",
		},
		{
			name:        "Colon",
			input:       "task1: task2",
			expectError: true,
			errorMsg:    "unexpected character ':'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tokenize(tt.input)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for input '%s', but got none", tt.input)
				} else if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("Expected error message '%s', but got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for input '%s': %v", tt.input, err)
				}
			}
		})
	}
}
