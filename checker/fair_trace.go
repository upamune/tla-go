package checker

import (
	"fmt"
	"slices"

	"github.com/upamune/tla-go/syntax/parser"
)

func (f *fairSearch) recordCycle(component []int) error {
	s := f.search
	members := make([]bool, len(s.nodes))
	for _, node := range component {
		members[node] = true
	}
	start := component[0]
	trace := f.prefix(start)
	loop := len(trace) - 1
	witnesses, err := f.cycleWitnesses(component, members)
	if err != nil {
		return err
	}
	f.debugCycle(start, witnesses)
	current, err := s.appendWitnesses(&trace, start, witnesses, members)
	if err != nil {
		return err
	}
	if len(trace) == loop+1 && current == start {
		trace = append(trace, TraceStep{Action: "Stuttering", State: s.nodes[start].state})
	}
	if err := s.appendPath(&trace, current, start, members); err != nil {
		return err
	}
	closing := trace[len(trace)-1].Action
	closingRange := trace[len(trace)-1].ActionRange
	trace = trace[:len(trace)-1]
	s.result.Status = LivenessViolation
	s.result.Violation = &Violation{Name: f.property.name, Trace: trace, LoopStart: &loop, LoopAction: closing, LoopActionRange: closingRange}
	return nil
}

func (s *search) appendWitnesses(trace *[]TraceStep, current int, witnesses []cycleWitness, members []bool) (int, error) {
	for _, witness := range witnesses {
		if err := s.appendPath(trace, current, witness.node, members); err != nil {
			return 0, err
		}
		current = witness.node
		if witness.edge != nil {
			current = witness.edge.to
			*trace = append(*trace, TraceStep{Action: witness.edge.action, State: s.nodes[current].state, ActionRange: actionSourceRange(witness.edge.source)})
		}
	}
	return current, nil
}

func (f *fairSearch) prefix(node int) []TraceStep {
	root, length := f.prefixRootAndLength(node)
	path := make([]TraceStep, length, length+1)
	for i := length - 1; i >= 0; i-- {
		path[i] = TraceStep{Action: f.via[node], State: f.search.nodes[node].state, ActionRange: actionSourceRange(f.viaSource[node])}
		node = f.parent[node]
	}
	if f.property.mode == guardedAlways {
		return append(f.guarded.trace(f.search, root), path[1:]...)
	}
	if f.property.mode == leadsTo || f.property.mode == leadsToStaying || f.property.mode == actionOften {
		return append(f.search.trace(root), path[1:]...)
	}
	return path
}

func (f *fairSearch) prefixRootAndLength(node int) (int, int) {
	root, length := node, 0
	for node >= 0 {
		root, node = node, f.parent[node]
		length++
	}
	return root, length
}

type traceParent struct {
	index  int
	action string
	source *parser.Expression
	seen   bool
}

func (s *search) appendPath(trace *[]TraceStep, start, target int, members []bool) error {
	if start == target {
		return nil
	}
	parent := s.pathParents(start, target, members)
	if !parent[target].seen {
		return fmt.Errorf("no path inside strongly connected component")
	}
	var path []TraceStep
	for node := target; node != start; node = parent[node].index {
		path = append(path, TraceStep{Action: parent[node].action, State: s.nodes[node].state, ActionRange: actionSourceRange(parent[node].source)})
	}
	for _, p := range slices.Backward(path) {
		*trace = append(*trace, p)
	}
	return nil
}

func (s *search) pathParents(start, target int, members []bool) []traceParent {
	parents := make([]traceParent, len(s.nodes))
	parents[start].seen = true
	queue := []int{start}
	for head := 0; head < len(queue) && !parents[target].seen; head++ {
		current := queue[head]
		for _, edge := range s.edges[current] {
			if !members[edge.to] || parents[edge.to].seen {
				continue
			}
			parents[edge.to] = traceParent{index: current, action: edge.action, source: edge.source, seen: true}
			queue = append(queue, edge.to)
		}
	}
	return parents
}
