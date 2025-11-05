package pipeline

import (
	"fmt"
	"sync"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/dag/parser"
	dagTask "github.com/patterninc/caterpillar/internal/pkg/pipeline/dag/task"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

type dag []*parser.Node

func (d dag) Run(wg *sync.WaitGroup, locker *sync.Mutex, channelSize int) map[string]error {
	fmt.Println("\n======================================================")
	fmt.Println("Final DAG Dependencies:")
	fmt.Println("======================================================")

	td := d
	i := 0
	nodeSet := make(map[string]bool)
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

		nodeSet[node.Task.GetName()] = true
		for _, node := range node.Upstream() {
			if !nodeSet[node.Task.GetName()] {
				td = append(td, node)
				nodeSet[node.Task.GetName()] = true
			}
		}
	}
	fmt.Println("======================================================")

	nodeSet = make(map[string]bool)
	errors := make(map[string]error)
	chanManager := &manager{
		chanSize: channelSize,
		inputs:   make(map[string]chan *record.Record),
		outputs:  make(map[string]chan *record.Record),
	}

	nodes := d
	i = 0
	for i < len(nodes) {
		var in, out chan *record.Record
		node := nodes[i]
		i++

		in = chanManager.getInputChannel(node, wg, locker, &errors)
		out = chanManager.getOutputChannel(node, wg, locker, &errors)

		wg.Add(1)
		go func(in <-chan *record.Record, out chan<- *record.Record, n *parser.Node) {
			defer wg.Done()
			if err := n.Task.Run(in, out); err != nil {
				locker.Lock()
				errors[n.Task.GetName()] = fmt.Errorf("error in task %s: %w", n.Task.GetName(), err)
				locker.Unlock()
			}
		}(in, out, node)

		nodeSet[node.Task.GetName()] = true
		for _, node := range node.Upstream() {
			if !nodeSet[node.Task.GetName()] {
				nodes = append(nodes, node)
				nodeSet[node.Task.GetName()] = true
			}
		}
	}
	return errors
}

type dagExpr struct {
	expr parser.Expr
}

func (dx *dagExpr) IsEmpty() bool {
	return dx.expr == nil
}

func (d *dagExpr) UnmarshalYAML(unmarshal func(any) error) error {
	var expr string

	if err := unmarshal(&expr); err != nil {
		return err
	}

	parsedExpr, err := parser.ParseDAG(expr)
	if err != nil {
		return err
	}
	d.expr = parsedExpr

	return nil
}

func buildDAG(expr dagExpr, taskMap map[string]task.Task) (dag, error) {
	dag, err := parser.BuildDag(expr.expr, func(name string) task.Task {
		return taskMap[name]
	})
	if err != nil {
		return nil, err
	}

	return dag, nil
}

type manager struct {
	chanSize int
	inputs   map[string]chan *record.Record
	outputs  map[string]chan *record.Record
}

// returns the input channel for the given node, from where the node should receive the data from, creating demuxes as necessary
func (m *manager) getInputChannel(n *parser.Node, wg *sync.WaitGroup, locker *sync.Mutex, errors *map[string]error) chan *record.Record {
	c := m.inputs[n.Task.GetName()]
	if c != nil {
		return c
	}

	if n.IsRoot() {
		return nil
	}
	if len(n.Upstream()) == 1 {
		m.getOutputChannel(n.Upstream()[0], wg, locker, errors)
		c = m.inputs[n.Task.GetName()]
		return c
	}

	// there are multiple upstreams, need to demux
	demuxOut := make(chan *record.Record, m.chanSize)
	d := dagTask.NewDemux(fmt.Sprintf("demux_%s", n.Task.GetName()), demuxOut)
	m.inputs[n.Task.GetName()] = demuxOut

	for _, upNode := range n.Upstream() {
		upOut := m.outputs[upNode.Task.GetName()]
		if upOut == nil {
			upOut = make(chan *record.Record, m.chanSize)
			m.outputs[upNode.Task.GetName()] = upOut
		}
		d.AddInputChannel(upOut)
	}

	wg.Add(1) // wait for demux
	go func(demux *dagTask.Demux) {
		defer wg.Done()
		if err := demux.Run(); err != nil {
			locker.Lock()
			(*errors)[n.Task.GetName()] = fmt.Errorf("error in task %s: %w", n.Task.GetName(), err)
			locker.Unlock()
		}
	}(d)

	return m.inputs[n.Task.GetName()]
}

// returns the output channel for the given node, where the node should send the data, creating muxes as necessary
func (m *manager) getOutputChannel(n *parser.Node, wg *sync.WaitGroup, locker *sync.Mutex, errors *map[string]error) chan *record.Record {
	c := m.outputs[n.Task.GetName()]
	if c != nil {
		return c
	}

	if n.IsLeaf() {
		return nil
	}

	if len(n.Downstream()) == 1 {
		c = make(chan *record.Record, m.chanSize)
		m.outputs[n.Task.GetName()] = c
		m.inputs[n.Downstream()[0].Task.GetName()] = c
		return c
	}

	// there are multiple downstreams, need to mux
	muxIn := make(chan *record.Record, m.chanSize)
	mx := dagTask.NewMux(fmt.Sprintf("mux_%s", n.Task.GetName()), muxIn)
	m.outputs[n.Task.GetName()] = muxIn

	for _, downNode := range n.Downstream() {
		downIn := m.inputs[downNode.Task.GetName()]
		if downIn == nil {
			downIn = make(chan *record.Record, m.chanSize)
			m.inputs[downNode.Task.GetName()] = downIn
		}
		mx.AddOutputChannel(downIn)
	}

	wg.Add(1) // wait for mux
	go func(mux *dagTask.Mux) {
		defer wg.Done()
		if err := mux.Run(); err != nil {
			locker.Lock()
			(*errors)[n.Task.GetName()] = fmt.Errorf("error in task %s: %w", n.Task.GetName(), err)
			locker.Unlock()
		}
	}(mx)

	return m.outputs[n.Task.GetName()]
}
