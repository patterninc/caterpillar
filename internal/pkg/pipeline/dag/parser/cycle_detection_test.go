package parser

import (
	"strings"
	"testing"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

// MockTask is a simple task implementation for testing
type MockTask struct {
	name string
}

func (m *MockTask) GetName() string {
	return m.name
}

func (m *MockTask) Run(<-chan *record.Record, chan<- *record.Record) error {
	return nil
}

func (m *MockTask) GetInputCount() int {
	return 1
}

func (m *MockTask) GetFailOnError() bool {
	return false
}

func createMockTaskRegistry(names ...string) func(string) task.Task {
	taskMap := make(map[string]task.Task)
	for _, name := range names {
		taskMap[name] = &MockTask{name: name}
	}

	return func(name string) task.Task {
		return taskMap[name]
	}
}

func TestCircularDependencyDetection(t *testing.T) {
	tests := []struct {
		name          string
		dagExpr       string
		expectError   bool
		errorContains string
	}{
		{
			name:          "Simple cycle - direct self reference",
			dagExpr:       "task1 >> task1",
			expectError:   true,
			errorContains: "circular dependency detected",
		},
		{
			name:          "Two task cycle",
			dagExpr:       "task1 >> task2 >> task1",
			expectError:   true,
			errorContains: "circular dependency detected",
		},
		{
			name:          "Three task cycle",
			dagExpr:       "task1 >> task2 >> task3 >> task1",
			expectError:   true,
			errorContains: "circular dependency detected",
		},
		{
			name:        "Valid linear DAG",
			dagExpr:     "task1 >> task2 >> task3",
			expectError: false,
		},
		{
			name:        "Valid diamond DAG",
			dagExpr:     "task1 >> [task2, task3] >> task4",
			expectError: false,
		},
		{
			name:        "Valid fan-out DAG",
			dagExpr:     "task1 >> [task2, task3, task4]",
			expectError: false,
		},
		{
			name:        "Valid fan-in DAG",
			dagExpr:     "[task1, task2, task3] >> task4",
			expectError: false,
		},
		{
			name:          "Complex cycle through branches",
			dagExpr:       "task1 >> [task2, task3] >> task4 >> task2",
			expectError:   true,
			errorContains: "circular dependency detected",
		},
		{
			name:          "Cycle in one branch of fan-out",
			dagExpr:       "task1 >> [task2 >> task3 >> task2, task4]",
			expectError:   true,
			errorContains: "circular dependency detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create task registry with all needed tasks
			taskRegistry := createMockTaskRegistry("task1", "task2", "task3", "task4")

			// Parse the DAG expression
			expr, err := ParseDAG(tt.dagExpr)
			if err != nil {
				t.Fatalf("Failed to parse DAG expression: %v", err)
			}

			// Build the DAG (this should detect cycles)
			_, err = BuildDag(expr, taskRegistry)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for DAG expression '%s', but got none", tt.dagExpr)
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', but got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for DAG expression '%s', but got: %v", tt.dagExpr, err)
				}
			}
		})
	}
}

func TestCycleDetectionWithComplexDAG(t *testing.T) {
	// Test a more complex scenario with multiple potential cycles
	dagExpr := "source >> [transform1, transform2] >> [process1 >> validate1, process2 >> validate2] >> sink >> source"
	taskRegistry := createMockTaskRegistry("source", "transform1", "transform2", "process1", "process2", "validate1", "validate2", "sink")

	expr, err := ParseDAG(dagExpr)
	if err != nil {
		t.Fatalf("Failed to parse complex DAG expression: %v", err)
	}

	_, err = BuildDag(expr, taskRegistry)
	if err == nil {
		t.Error("Expected circular dependency error for complex DAG with cycle, but got none")
	} else if !strings.Contains(err.Error(), "circular dependency detected") {
		t.Errorf("Expected circular dependency error, but got: %v", err)
	}
}

func TestNoCycleInComplexValidDAG(t *testing.T) {
	// Test a complex but valid DAG
	dagExpr := "source >> [transform1, transform2] >> [process1 >> validate1, process2 >> validate2] >> sink"
	taskRegistry := createMockTaskRegistry("source", "transform1", "transform2", "process1", "process2", "validate1", "validate2", "sink")

	expr, err := ParseDAG(dagExpr)
	if err != nil {
		t.Fatalf("Failed to parse complex valid DAG expression: %v", err)
	}

	nodes, err := BuildDag(expr, taskRegistry)
	if err != nil {
		t.Errorf("Expected no error for complex valid DAG, but got: %v", err)
	}

	if len(nodes) == 0 {
		t.Error("Expected nodes to be returned for valid DAG")
	}
}
