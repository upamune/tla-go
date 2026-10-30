package checker

import (
	"fmt"
	"slices"

	"github.com/upamune/tla-go/syntax/parser"
)

// A node may be reachable both before and after a trigger. The armed path is
// retained separately from the ordinary BFS parent so merges do not hide it.
type guardedNode struct {
	armed, expanded bool
	parent          int
	action          string
	source          *parser.Expression
}

type guardedMonitor struct {
	property temporalProperty
	nodes    []guardedNode
	ready    []int
}

func (s *search) checkGuardedProperties(index int) (bool, error) {
	if len(s.model.fairness) > 0 || s.model.tlcRuntime.needsLevelGraph {
		return true, nil
	}
	for i := range s.guarded {
		trace, err := s.guarded[i].advance(s, index)
		if err != nil {
			return false, err
		}
		if trace != nil {
			return s.recordGuardedViolation(trace)
		}
	}
	return true, nil
}

func (g *guardedMonitor) advance(s *search, index int) ([]TraceStep, error) {
	for len(g.nodes) < len(s.nodes) {
		g.nodes = append(g.nodes, guardedNode{})
	}
	g.nodes[index].expanded = true
	trigger, err := s.model.holdsProperty(g.property.trigger, s.nodes[index].state, g.property.locals)
	if err != nil {
		return nil, err
	}
	if trigger && !g.nodes[index].armed {
		g.nodes[index].armed = true
		g.nodes[index].parent = -1
	}
	if g.nodes[index].armed {
		g.ready = append(g.ready, index)
	}
	return g.propagate(s)
}

func (g *guardedMonitor) propagate(s *search) ([]TraceStep, error) {
	for head := 0; head < len(g.ready); head++ {
		if err := s.model.ctx.Err(); err != nil {
			return nil, err
		}
		index := g.ready[head]
		valid, err := s.model.holdsProperty(g.property.predicate, s.nodes[index].state, g.property.locals)
		if err != nil {
			return nil, err
		}
		if !valid {
			return g.trace(s, index), nil
		}
		g.propagateEdges(s, index)
	}
	g.ready = g.ready[:0]
	return nil, nil
}

func (g *guardedMonitor) propagateEdges(s *search, index int) {
	for _, edge := range s.edges[index] {
		if g.nodes[edge.to].armed {
			continue
		}
		g.nodes[edge.to].armed = true
		g.nodes[edge.to].parent = index
		g.nodes[edge.to].action = edge.action
		g.nodes[edge.to].source = edge.source
		if g.nodes[edge.to].expanded {
			g.ready = append(g.ready, edge.to)
		}
	}
}

func (g *guardedMonitor) trace(s *search, index int) []TraceStep {
	var tail []TraceStep
	for g.nodes[index].parent >= 0 {
		tail = append(tail, TraceStep{State: s.nodes[index].state, Action: g.nodes[index].action, ActionRange: actionSourceRange(g.nodes[index].source)})
		index = g.nodes[index].parent
	}
	trace := s.trace(index)
	for _, step := range slices.Backward(tail) {
		trace = append(trace, step)
	}
	return trace
}

func (s *search) recordGuardedViolation(trace []TraceStep) (bool, error) {
	names, err := s.guardedViolations(trace)
	if err != nil {
		return false, err
	}
	if len(names) == 0 {
		return false, fmt.Errorf("safety counterexample violates no property")
	}
	s.result.Status = PropertyViolation
	s.result.Violation = &Violation{Name: names[0], Properties: names, Trace: trace}
	return false, nil
}
