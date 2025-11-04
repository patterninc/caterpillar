package pipeline

import (
	"fmt"
	"sync"

	dagparser "github.com/patterninc/caterpillar/internal/pkg/pipeline/dag_parser"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/mux"
)

type dag []*dagparser.Node

func (d dag) Run(wg *sync.WaitGroup, locker *sync.Mutex, channelSize int) []error {
	// First, let's print all nodes with their dependencies
	fmt.Println("\n======================================================")
	fmt.Println("Final DAG Dependencies:")
	fmt.Println("======================================================")

	td := d
	i := 0
	for i < len(td) {
		node := td[i]
		i++
		upstreamNames := make([]string, 0)
		for _, upNode := range node.Upstream() {
			upstreamNames = append(upstreamNames, upNode.Task.GetName())
		}

		downstreamNames := make([]string, 0)
		for _, downNode := range node.Downstream() {
			downstreamNames = append(downstreamNames, downNode.Task.GetName())
		}

		fmt.Printf("Task: %s\n", node.Task.GetName())
		fmt.Printf("  Upstream: %v\n", upstreamNames)
		fmt.Printf("  Downstream: %v\n", downstreamNames)
		td = append(td, node.Upstream()...)
	}
	fmt.Println("======================================================")

	visited := make(map[string]bool)
	inputs := make(map[string]chan *record.Record)
	errors := make([]error, 0)

	nodes := d
	i = 0
	for i < len(nodes) {
		var in, out chan *record.Record
		node := nodes[i]
		i++

		if node.IsRoot() {
			inputs[node.Task.GetName()] = nil
		} else {
			in = inputs[node.Task.GetName()]
			if in == nil {
				in = make(chan *record.Record)
				inputs[node.Task.GetName()] = in
			}
		}

		if visited[node.Task.GetName()] {
			continue
		}

		if len(node.Downstream()) <= 1 {

			if node.IsLeaf() {
				out = nil
			} else {
				// there's one downstream
				out = inputs[node.Downstream()[0].Task.GetName()]
			}

			go func(in chan *record.Record, out chan *record.Record) {
				defer wg.Done()
				if err := node.Task.Run(in, out); err != nil {
					locker.Lock()
					errors = append(errors, fmt.Errorf("error in task %s: %w", node.Task.GetName(), err))
					locker.Unlock()
				}
			}(in, out)
		} else {
			// there are multiple downstreams, the output needs to be muxed
			muxIn := make(chan *record.Record, channelSize)
			m := mux.New(fmt.Sprintf("Mux after %s", node.Task.GetName()), muxIn)

			for _, downstreamNode := range node.Downstream() {
				out := inputs[downstreamNode.Task.GetName()]
				m.AddOutputChannel(out)
			}

			wg.Add(1) // wait for mux
			go func(mux *mux.Mux) {
				defer wg.Done()
				if err := mux.Run(); err != nil {
					locker.Lock()
					errors = append(errors, fmt.Errorf("error in task %s: %w", node.Task.GetName(), err))
					locker.Unlock()
				}
			}(m)

			go func(in <-chan *record.Record, out chan<- *record.Record) {
				defer wg.Done()
				if err := node.Task.Run(in, muxIn); err != nil {
					locker.Lock()
					errors = append(errors, fmt.Errorf("error in task %s: %w", node.Task.GetName(), err))
					locker.Unlock()
				}
			}(in, muxIn)
		}

		for _, node := range node.Upstream() {
			if !visited[node.Task.GetName()] {
				nodes = append(nodes, node)
			}
		}
		visited[node.Task.GetName()] = true
	}
	return errors
}

type dagExpr struct {
	expr dagparser.Expr
}

func (dx *dagExpr) IsEmpty() bool {
	return dx.expr == nil
}

func (d *dagExpr) UnmarshalYAML(unmarshal func(any) error) error {
	var expr string

	if err := unmarshal(&expr); err != nil {
		return err
	}

	parsedExpr, err := dagparser.ParseDAG(expr)
	if err != nil {
		return err
	}
	d.expr = parsedExpr

	return nil
}

func buildDAG(expr dagExpr, taskMap map[string]task.Task) dag {
	dag, err := dagparser.BuildDag(expr.expr, func(name string) task.Task {
		return taskMap[name]
	})
	if err != nil {
		fmt.Printf("error building dag: %v\n", err)
		return nil
	}

	return dag
}
