package parser

import (
	"reflect"
	"testing"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

func TestIdentResolve(t *testing.T) {
	tests := []struct {
		name        string
		taskName    string
		taskExists  bool
		expectError bool
		errorMsg    string
	}{
		{
			name:       "Valid task",
			taskName:   "task1",
			taskExists: true,
		},
		{
			name:        "Missing task",
			taskName:    "nonexistent",
			taskExists:  false,
			expectError: true,
			errorMsg:    "task not found: nonexistent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ident := &Ident{Name: tt.taskName}

			taskRegistry := func(name string) task.Task {
				if name == tt.taskName && tt.taskExists {
					return &MockTask{name: name}
				}
				return nil
			}

			// Test ResolveLeft
			nodes, err := ident.ResolveLeft(taskRegistry)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for ResolveLeft, but got none")
				} else if err.Error() != tt.errorMsg {
					t.Errorf("Expected error message '%s', but got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for ResolveLeft: %v", err)
				} else if len(nodes) != 1 {
					t.Errorf("Expected 1 node, got %d", len(nodes))
				} else if nodes[0].Task.GetName() != tt.taskName {
					t.Errorf("Expected task name '%s', got '%s'", tt.taskName, nodes[0].Task.GetName())
				}
			}

			// Test ResolveRight
			nodes, err = ident.ResolveRight(taskRegistry)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for ResolveRight, but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for ResolveRight: %v", err)
				} else if len(nodes) != 1 {
					t.Errorf("Expected 1 node, got %d", len(nodes))
				}
			}
		})
	}
}

func TestIdentPostOrder(t *testing.T) {
	ident := &Ident{Name: "test_task"}
	result := ident.PostOrder()
	expected := []string{"test_task"}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected PostOrder %v, got %v", expected, result)
	}
}

func TestBinOpPostOrder(t *testing.T) {
	tests := []struct {
		name     string
		left     Expr
		right    Expr
		op       string
		expected []string
	}{
		{
			name:     "Simple sequence",
			left:     &Ident{Name: "task1"},
			right:    &Ident{Name: "task2"},
			op:       ">>",
			expected: []string{"task1", "task2", ">>"},
		},
		{
			name: "Nested BinOp left",
			left: &BinOp{
				Left:  &Ident{Name: "task1"},
				Right: &Ident{Name: "task2"},
				Op:    ">>",
			},
			right:    &Ident{Name: "task3"},
			op:       ">>",
			expected: []string{"task1", "task2", ">>", "task3", ">>"},
		},
		{
			name:     "Tuple left",
			left:     &Tuple{Elements: []Expr{&Ident{Name: "task1"}, &Ident{Name: "task2"}}},
			right:    &Ident{Name: "task3"},
			op:       ">>",
			expected: []string{"task1", "task2", "task3", ">>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			binOp := &BinOp{
				Left:  tt.left,
				Right: tt.right,
				Op:    tt.op,
			}

			result := binOp.PostOrder()
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Expected PostOrder %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestTuplePostOrder(t *testing.T) {
	tests := []struct {
		name     string
		elements []Expr
		expected []string
	}{
		{
			name:     "Two identifiers",
			elements: []Expr{&Ident{Name: "task1"}, &Ident{Name: "task2"}},
			expected: []string{"task1", "task2"},
		},
		{
			name:     "Three identifiers",
			elements: []Expr{&Ident{Name: "task1"}, &Ident{Name: "task2"}, &Ident{Name: "task3"}},
			expected: []string{"task1", "task2", "task3"},
		},
		{
			name: "Mixed expressions",
			elements: []Expr{
				&Ident{Name: "task1"},
				&BinOp{
					Left:  &Ident{Name: "task2"},
					Right: &Ident{Name: "task3"},
					Op:    ">>",
				},
			},
			expected: []string{"task1", "task2", "task3", ">>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tuple := &Tuple{Elements: tt.elements}
			result := tuple.PostOrder()

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Expected PostOrder %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestBinOpResolve(t *testing.T) {
	taskRegistry := func(name string) task.Task {
		return &MockTask{name: name}
	}

	tests := []struct {
		name           string
		left           Expr
		right          Expr
		validateResult func(*testing.T, []*Node, []*Node)
	}{
		{
			name:  "Simple sequence",
			left:  &Ident{Name: "task1"},
			right: &Ident{Name: "task2"},
			validateResult: func(t *testing.T, leftNodes, rightNodes []*Node) {
				if len(leftNodes) != 1 || len(rightNodes) != 1 {
					t.Fatalf("Expected 1 left and 1 right node, got %d left, %d right", len(leftNodes), len(rightNodes))
				}

				// Check that task1 has task2 as downstream
				if len(leftNodes[0].downstream) != 1 {
					t.Errorf("Expected 1 downstream connection, got %d", len(leftNodes[0].downstream))
				} else if leftNodes[0].downstream[0].Task.GetName() != "task2" {
					t.Errorf("Expected downstream task2, got %s", leftNodes[0].downstream[0].Task.GetName())
				}

				// Check that task2 has task1 as upstream
				if len(rightNodes[0].upstream) != 1 {
					t.Errorf("Expected 1 upstream connection, got %d", len(rightNodes[0].upstream))
				} else if rightNodes[0].upstream[0].Task.GetName() != "task1" {
					t.Errorf("Expected upstream task1, got %s", rightNodes[0].upstream[0].Task.GetName())
				}
			},
		},
		{
			name:  "Fan-out pattern",
			left:  &Ident{Name: "task1"},
			right: &Tuple{Elements: []Expr{&Ident{Name: "task2"}, &Ident{Name: "task3"}}},
			validateResult: func(t *testing.T, leftNodes, rightNodes []*Node) {
				if len(leftNodes) != 1 || len(rightNodes) != 2 {
					t.Fatalf("Expected 1 left and 2 right nodes, got %d left, %d right", len(leftNodes), len(rightNodes))
				}

				// Check that task1 has 2 downstream connections
				if len(leftNodes[0].downstream) != 2 {
					t.Errorf("Expected 2 downstream connections, got %d", len(leftNodes[0].downstream))
				}

				// Check that both right nodes have task1 as upstream
				for _, rightNode := range rightNodes {
					if len(rightNode.upstream) != 1 {
						t.Errorf("Expected 1 upstream connection for %s, got %d", rightNode.Task.GetName(), len(rightNode.upstream))
					} else if rightNode.upstream[0].Task.GetName() != "task1" {
						t.Errorf("Expected upstream task1 for %s, got %s", rightNode.Task.GetName(), rightNode.upstream[0].Task.GetName())
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset global registry for each test
			globalNodeRegistry = make(map[string]*Node)

			binOp := &BinOp{
				Left:  tt.left,
				Right: tt.right,
				Op:    ">>",
			}

			leftNodes, rightNodes, err := binOp.resolve(taskRegistry)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			tt.validateResult(t, leftNodes, rightNodes)
		})
	}
}

func TestTupleResolve(t *testing.T) {
	taskRegistry := func(name string) task.Task {
		return &MockTask{name: name}
	}

	tests := []struct {
		name          string
		elements      []Expr
		expectedCount int
		expectedNames []string
	}{
		{
			name:          "Two identifiers",
			elements:      []Expr{&Ident{Name: "task1"}, &Ident{Name: "task2"}},
			expectedCount: 2,
			expectedNames: []string{"task1", "task2"},
		},
		{
			name:          "Three identifiers",
			elements:      []Expr{&Ident{Name: "task1"}, &Ident{Name: "task2"}, &Ident{Name: "task3"}},
			expectedCount: 3,
			expectedNames: []string{"task1", "task2", "task3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset global registry for each test
			globalNodeRegistry = make(map[string]*Node)

			tuple := &Tuple{Elements: tt.elements}

			// Test ResolveLeft
			leftNodes, err := tuple.ResolveLeft(taskRegistry)
			if err != nil {
				t.Fatalf("Unexpected error in ResolveLeft: %v", err)
			}
			if len(leftNodes) != tt.expectedCount {
				t.Errorf("Expected %d nodes from ResolveLeft, got %d", tt.expectedCount, len(leftNodes))
			}

			// Test ResolveRight
			rightNodes, err := tuple.ResolveRight(taskRegistry)
			if err != nil {
				t.Fatalf("Unexpected error in ResolveRight: %v", err)
			}
			if len(rightNodes) != tt.expectedCount {
				t.Errorf("Expected %d nodes from ResolveRight, got %d", tt.expectedCount, len(rightNodes))
			}

			// Verify node names
			for i, expectedName := range tt.expectedNames {
				if i < len(leftNodes) && leftNodes[i].Task.GetName() != expectedName {
					t.Errorf("Expected left node %d to be %s, got %s", i, expectedName, leftNodes[i].Task.GetName())
				}
				if i < len(rightNodes) && rightNodes[i].Task.GetName() != expectedName {
					t.Errorf("Expected right node %d to be %s, got %s", i, expectedName, rightNodes[i].Task.GetName())
				}
			}
		})
	}
}

func TestNodeMethods(t *testing.T) {
	task1 := &MockTask{name: "task1"}
	task2 := &MockTask{name: "task2"}
	task3 := &MockTask{name: "task3"}

	node1 := &Node{Task: task1}
	node2 := &Node{Task: task2}
	node3 := &Node{Task: task3}

	// Test initial state
	if !node1.IsRoot() {
		t.Error("Node1 should be a root initially")
	}
	if !node1.IsLeaf() {
		t.Error("Node1 should be a leaf initially")
	}

	// Add connections
	node1.downstream = append(node1.downstream, node2)
	node2.upstream = append(node2.upstream, node1)

	node2.downstream = append(node2.downstream, node3)
	node3.upstream = append(node3.upstream, node2)

	// Test root/leaf status
	if !node1.IsRoot() {
		t.Error("Node1 should still be a root")
	}
	if node1.IsLeaf() {
		t.Error("Node1 should no longer be a leaf")
	}

	if node2.IsRoot() {
		t.Error("Node2 should not be a root")
	}
	if node2.IsLeaf() {
		t.Error("Node2 should not be a leaf")
	}

	if node3.IsRoot() {
		t.Error("Node3 should not be a root")
	}
	if !node3.IsLeaf() {
		t.Error("Node3 should be a leaf")
	}

	// Test String method
	if node1.String() != "task1" {
		t.Errorf("Expected node1.String() to be 'task1', got '%s'", node1.String())
	}

	// Test accessor methods
	if len(node1.Upstream()) != 0 {
		t.Errorf("Expected node1 to have 0 upstream nodes, got %d", len(node1.Upstream()))
	}
	if len(node1.Downstream()) != 1 {
		t.Errorf("Expected node1 to have 1 downstream node, got %d", len(node1.Downstream()))
	}
	if node1.Downstream()[0].Task.GetName() != "task2" {
		t.Errorf("Expected node1 downstream to be task2, got %s", node1.Downstream()[0].Task.GetName())
	}
}
