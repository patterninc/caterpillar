package parser

import (
	"fmt"

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
		// This shouldn't happen if wrappedGetTask is used properly
		node := &Node{Task: task}
		globalNodeRegistry[i.Name] = node
		return []*Node{node}, nil
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
			downstreamExists := false
			for _, existing := range leftNode.downstream {
				if existing == rightNode {
					downstreamExists = true
					break
				}
			}

			// Check if upstream connection already exists
			upstreamExists := false
			for _, existing := range rightNode.upstream {
				if existing == leftNode {
					upstreamExists = true
					break
				}
			}

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
	return expr.ResolveRight(wrappedGetTask)
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
