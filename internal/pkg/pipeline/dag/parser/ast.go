package parser

import (
	"fmt"
	"slices"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

type Expr interface {
	PostOrder() []string
	ResolveLeft(func(name string) task.Task) ([]*Node, error)
	ResolveRight(func(name string) task.Task) ([]*Node, error)
}

type Ident struct {
	Name string
}

func (i *Ident) PostOrder() []string {
	return []string{i.Name}
}

func (i *Ident) resolve(getTask func(name string) task.Task) ([]*Node, error) {
	task := getTask(i.Name)
	if task == nil {
		return nil, fmt.Errorf("task not found: %s", i.Name)
	}

	// Use existing node from registry if available
	if globalNodeRegistry != nil {
		if node, exists := globalNodeRegistry[i.Name]; exists {
			return []*Node{node}, nil
		}
	}

	// Fallback for when registry is not available
	return []*Node{{Task: task}}, nil
}

func (i *Ident) ResolveLeft(getTask func(name string) task.Task) ([]*Node, error) {
	return i.resolve(getTask)
}

func (i *Ident) ResolveRight(getTask func(name string) task.Task) ([]*Node, error) {
	return i.resolve(getTask)
}

type BinOp struct {
	Op    string
	Left  Expr
	Right Expr
}

func (b *BinOp) PostOrder() []string {
	order := make([]string, 0)
	order = append(order, b.Left.PostOrder()...)
	order = append(order, b.Right.PostOrder()...)
	order = append(order, b.Op)
	return order
}

func (b *BinOp) resolve(getTask func(name string) task.Task) ([]*Node, []*Node, error) {
	leftNodes, err := b.Left.ResolveRight(getTask)
	if err != nil {
		return nil, nil, err
	}

	rightNodes, err := b.Right.ResolveLeft(getTask)
	if err != nil {
		return nil, nil, err
	}

	for _, leftNode := range leftNodes {
		for _, rightNode := range rightNodes {
			// Check if downstream connection already exists
			downstreamExists := slices.Contains(leftNode.downstream, rightNode)

			// Check if upstream connection already exists
			upstreamExists := slices.Contains(rightNode.upstream, leftNode)

			// Only add connections if they don't already exist
			if !downstreamExists {
				leftNode.downstream = append(leftNode.downstream, rightNode)
			}

			if !upstreamExists {
				rightNode.upstream = append(rightNode.upstream, leftNode)
			}
		}
	}

	return leftNodes, rightNodes, nil
}

func (b *BinOp) ResolveLeft(getTask func(name string) task.Task) ([]*Node, error) {
	// Execute resolve to build the connections
	_, _, err := b.resolve(getTask)
	if err != nil {
		return nil, err
	}

	// Return the leftmost nodes of the entire expression
	return b.Left.ResolveLeft(getTask)
}

func (b *BinOp) ResolveRight(getTask func(name string) task.Task) ([]*Node, error) {
	// Execute resolve to build the connections
	_, _, err := b.resolve(getTask)
	if err != nil {
		return nil, err
	}

	// Return the rightmost nodes of the entire expression
	return b.Right.ResolveRight(getTask)
}

type Tuple struct {
	Elements []Expr
}

func (t *Tuple) PostOrder() []string {
	order := []string{}
	for _, elem := range t.Elements {
		order = append(order, elem.PostOrder()...)
	}
	return order
}

func (t *Tuple) ResolveLeft(getTask func(name string) task.Task) ([]*Node, error) {
	nodes := make([]*Node, 0)
	var resolved []*Node
	for _, elem := range t.Elements {
		var err error
		resolved, err = elem.ResolveLeft(getTask)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, resolved...)
	}

	return nodes, nil
}

func (t *Tuple) ResolveRight(getTask func(name string) task.Task) ([]*Node, error) {
	nodes := make([]*Node, 0)
	var resolved []*Node
	for _, elem := range t.Elements {
		var err error
		resolved, err = elem.ResolveRight(getTask)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, resolved...)
	}

	return nodes, nil
}

func BuildDag(expr Expr, getTask func(name string) task.Task) ([]*Node, error) {
	// Create a global node registry to track all created nodes
	nodeRegistry := make(map[string]*Node)

	// Wrap the getTask function to register nodes
	wrappedGetTask := func(name string) task.Task {
		task := getTask(name)
		if task != nil {
			// Check if node already exists
			if _, exists := nodeRegistry[name]; !exists {
				nodeRegistry[name] = &Node{Task: task}
			}
		}
		return task
	}

	// Set global registry for use in resolve methods
	globalNodeRegistry = nodeRegistry

	// Execute the resolution to build the DAG
	nodes, err := expr.ResolveRight(wrappedGetTask)
	if err != nil {
		return nil, err
	}

	// Check for circular dependencies
	if err := detectCycles(nodeRegistry); err != nil {
		return nil, err
	}

	return nodes, nil
}

// detectCycles performs a depth-first search to detect cycles in the DAG
func detectCycles(nodeRegistry map[string]*Node) error {
	// Track visit states: 0 = unvisited, 1 = visiting, 2 = visited
	visitState := make(map[string]int)

	// Initialize all nodes as unvisited
	for name := range nodeRegistry {
		visitState[name] = 0
	}

	// Check each unvisited node for cycles
	for name, node := range nodeRegistry {
		if visitState[name] == 0 {
			if err := dfsDetectCycle(node, visitState, make([]string, 0)); err != nil {
				return err
			}
		}
	}

	return nil
}

// dfsDetectCycle performs depth-first search to detect cycles
func dfsDetectCycle(node *Node, visitState map[string]int, path []string) error {
	nodeName := node.Task.GetName()

	// If we're currently visiting this node, we found a cycle
	if visitState[nodeName] == 1 {
		// Find where the cycle starts in the path
		cycleStart := -1
		for i, pathNode := range path {
			if pathNode == nodeName {
				cycleStart = i
				break
			}
		}

		var cyclePath []string
		if cycleStart >= 0 {
			cyclePath = append(path[cycleStart:], nodeName)
		} else {
			cyclePath = append(path, nodeName)
		}

		return fmt.Errorf("circular dependency detected: %v", cyclePath)
	}

	// If already fully visited, no cycle through this path
	if visitState[nodeName] == 2 {
		return nil
	}

	// Mark as currently visiting
	visitState[nodeName] = 1
	newPath := append(path, nodeName)

	// Visit all downstream nodes
	for _, downstream := range node.downstream {
		if err := dfsDetectCycle(downstream, visitState, newPath); err != nil {
			return err
		}
	}

	// Mark as fully visited
	visitState[nodeName] = 2
	return nil
}

// Global registry to track nodes during construction
var globalNodeRegistry map[string]*Node

type Node struct {
	upstream   []*Node
	downstream []*Node
	Task       task.Task
}

func (n *Node) String() string {
	return n.Task.GetName()
}

func (n *Node) IsLeaf() bool {
	return len(n.downstream) == 0
}

func (n *Node) IsRoot() bool {
	return len(n.upstream) == 0
}

func (n *Node) Upstream() []*Node {
	return n.upstream
}

func (n *Node) Downstream() []*Node {
	return n.downstream
}
