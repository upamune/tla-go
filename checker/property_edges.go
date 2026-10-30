package checker

import (
	"fmt"
	"maps"

	"github.com/upamune/tla-go/evaluator"
)

func (s *search) withoutPropertyEdges(p temporalProperty) (*search, error) {
	graph := *s
	graph.edges = make([][]graphEdge, len(s.edges))
	remap := s.hasCachedFairActions()
	if remap {
		graph.ltlValues = maps.Clone(s.ltlValues)
	}
	for node, edges := range s.edges {
		if err := s.model.ctx.Err(); err != nil {
			return nil, err
		}
		filtered, choices, err := s.filterPropertyEdges(p, node, edges, remap)
		if err != nil {
			return nil, err
		}
		graph.edges[node] = filtered
		if remap {
			graph.remapFairActions(s, node, len(edges), choices)
		}
	}
	return &graph, nil
}

func (s *search) filterPropertyEdges(p temporalProperty, node int, edges []graphEdge, keepChoices bool) ([]graphEdge, []int, error) {
	var filtered []graphEdge
	var choices []int
	filter := viewEdgeFilter{enabled: s.model.hasReduction(), node: node, allow: true}
	for choice, edge := range edges {
		ok, err := s.actionPropertyForEdge(p, node, edge)
		if err != nil {
			return nil, nil, err
		}
		if filter.keep(edge, ok) {
			filtered = append(filtered, edge)
			if keepChoices {
				choices = append(choices, choice)
			}
		}
	}
	return filtered, choices, nil
}

func (s *search) actionProperty(p temporalProperty, from, to int) (bool, error) {
	return s.actionPropertyForEdge(p, from, graphEdge{to: to})
}

func (s *search) actionPropertyForEdge(p temporalProperty, from int, edge graphEdge) (bool, error) {
	context := evaluator.Context{Current: s.nodes[from].state, Next: s.edgeNext(edge), Constants: s.model.constants, Locals: p.locals, Enabled: s.model.enabledCallback}
	result, err := s.model.program.Evaluate(p.predicate, context)
	if err != nil {
		return false, err
	}
	ok, valid := result.Bool()
	if !valid {
		return false, fmt.Errorf("temporal action must be boolean, got %s", result)
	}
	return ok, nil
}

type viewEdgeFilter struct {
	enabled, seen, allow bool
	node                 int
}

func (f *viewEdgeFilter) keep(edge graphEdge, propertyHolds bool) bool {
	if f.enabled && edge.to == f.node {
		if !f.seen {
			f.seen, f.allow = true, !propertyHolds
		}
		if !f.allow {
			return false
		}
	}
	return !propertyHolds
}
