package checker

import (
	"cmp"
	"fmt"
	"slices"
)

// Simulation supports selected pure state formulas without fairness or state
// reduction. Other temporal forms still fail explicitly.
func (m *model) supportsSimulationLiveness() bool {
	if len(m.guarded) > 0 || len(m.fairness) > 0 {
		return false
	}
	if len(m.livenessOrder) > 0 && m.hasReduction() {
		return false
	}
	for _, property := range m.livenessOrder {
		if !m.supportsSimulationProperty(property) {
			return false
		}
	}
	return true
}

func (m *model) supportsSimulationProperty(property temporalProperty) bool {
	if property.mode == leadsTo && len(m.livenessOrder) == 1 {
		return m.supportsSimulationLeadsTo(property)
	}
	return (property.mode == eventually || property.mode == often || property.mode == eventuallyAlways) && m.isStateExpression(property.predicate) &&
		!m.propertyUsesTLC(property) && !m.unsafeIncrementalExpression(property.predicate)
}

func (m *model) supportsSimulationLeadsTo(property temporalProperty) bool {
	return m.isStateExpression(property.trigger) && m.isStateExpression(property.predicate) &&
		!m.propertyUsesTLC(property) && !m.unsafeIncrementalExpression(property.trigger) && !m.unsafeIncrementalExpression(property.predicate)
}

func (r *simulationRun) checkTraceLiveness(visited int) (*Result, error) {
	if len(r.model.livenessOrder) == 0 {
		return nil, nil
	}
	if visited == 0 {
		return nil, fmt.Errorf("zero-depth liveness simulation is not yet supported")
	}
	if len(r.model.livenessOrder) == 1 && r.model.livenessOrder[0].mode == leadsTo {
		return r.checkSimulationLeadsTo(visited, r.model.livenessOrder[0])
	}
	graph, err := r.traceGraph(visited)
	if err != nil {
		return nil, err
	}
	previous := r.model.tlcRuntime.search
	r.model.tlcRuntime.search = &graph
	defer func() { r.model.tlcRuntime.search = previous }()
	result, err := graph.checkSimulationLivenessGraph()
	if err != nil || result.Status != Success {
		return &result, err
	}
	return nil, nil
}

// LiveCheck1 groups recurrence and stability into one no-tableau solution.
// Its DNF normalization moves stability to the end; each eventuality has a
// separate tableau solution in its original position.
func (s *search) checkSimulationLivenessGraph() (Result, error) {
	if len(s.model.livenessOrder) <= 1 {
		return s.checkLivenessGraph()
	}
	checkedGroup := false
	for _, property := range s.model.livenessOrder {
		found, checked, err := s.checkSimulationSolution(property, checkedGroup)
		if err != nil {
			return s.result, err
		}
		checkedGroup = checkedGroup || checked
		if found {
			return s.reportLassoProperties()
		}
	}
	if checkedGroup {
		return s.result, nil
	}
	return s.finishSimulationNoTableau()
}

func (s *search) finishSimulationNoTableau() (Result, error) {
	found, err := s.checkSimulationPropertyGroup(s.simulationNoTableauProperties())
	if err != nil {
		return s.result, err
	}
	if found {
		return s.reportLassoProperties()
	}
	return s.result, nil
}

func (s *search) checkSimulationSolution(property temporalProperty, checkedGroup bool) (bool, bool, error) {
	if property.mode == eventually {
		found, err := s.checkLivenessProperty(property)
		return found, false, err
	}
	if property.mode == often && !checkedGroup {
		found, err := s.checkSimulationPropertyGroup(s.simulationNoTableauProperties())
		return found, true, err
	}
	return false, false, nil
}

func (s *search) simulationNoTableauProperties() []temporalProperty {
	var result []temporalProperty
	for _, mode := range []temporalMode{often, eventuallyAlways} {
		for _, property := range s.model.livenessOrder {
			if property.mode == mode {
				result = append(result, property)
			}
		}
	}
	return result
}

func (s *search) checkSimulationPropertyGroup(properties []temporalProperty) (bool, error) {
	if len(properties) == 0 {
		return false, nil
	}
	ranks := s.simulationComponentRanks()
	base := s.result
	best := len(s.nodes)
	var selected Result
	for _, property := range properties {
		s.result = base
		rank, err := s.simulationPropertyRank(property, ranks, best)
		if err != nil {
			return false, err
		}
		if rank < best {
			selected, best = s.result, rank
		}
	}
	if best == len(s.nodes) {
		s.result = base
		return false, nil
	}
	s.result = selected
	return true, nil
}

func (s *search) simulationPropertyRank(property temporalProperty, ranks []int, fallback int) (int, error) {
	found, err := s.checkLivenessProperty(property)
	if err != nil || !found {
		return fallback, err
	}
	return s.simulationViolationRank(ranks)
}

func (s *search) simulationComponentRanks() []int {
	all := make([]bool, len(s.nodes))
	for i := range all {
		all[i] = true
	}
	ranks := make([]int, len(s.nodes))
	for rank, component := range slices.Backward(s.components(all)) {
		for _, node := range component {
			ranks[node] = len(s.nodes) - 1 - rank
		}
	}
	return ranks
}

func (s *search) simulationViolationRank(ranks []int) (int, error) {
	v := s.result.Violation
	if !validLasso(v) {
		return 0, fmt.Errorf("invalid simulation liveness counterexample")
	}
	key, err := s.model.unreducedStateKey(v.Trace[*v.LoopStart].State)
	if err != nil {
		return 0, err
	}
	index, ok := s.indices[key]
	if !ok {
		return 0, fmt.Errorf("simulation counterexample state is missing from graph")
	}
	return ranks[index], nil
}

// LiveCheck1 merges equal observed states and adds a stuttering transition to
// every state, even when the sampled trace itself does not stutter.
func (r *simulationRun) traceGraph(visited int) (search, error) {
	graph := newSearch(r.model)
	graph.result = r.search.result
	graph.indices = map[string]int{}
	previous := -1
	for _, node := range r.search.nodes[:visited] {
		key, err := r.model.unreducedStateKey(node.state)
		if err != nil {
			return graph, err
		}
		index, exists := graph.indices[key]
		if !exists {
			index = graph.addSimulationNode(node, previous)
			graph.indices[key] = index
		}
		if previous < 0 {
			graph.initial = append(graph.initial, index)
		} else if !simulationEdgeExists(graph.edges[previous], index) {
			graph.edges[previous] = append(graph.edges[previous], graphEdge{to: index, action: node.action, source: node.source})
		}
		previous = index
	}
	return graph, nil
}

func (s *search) addSimulationNode(node stateNode, previous int) int {
	index := len(s.nodes)
	node.parent = previous
	s.nodes = append(s.nodes, node)
	checkState := node.state
	if previous >= 0 {
		checkState = s.nodes[previous].state
	}
	s.simulationCheckStates = append(s.simulationCheckStates, checkState)
	s.edges = append(s.edges, []graphEdge{{to: index, action: "Stuttering"}})
	return index
}

func simulationEdgeExists(edges []graphEdge, target int) bool {
	for _, edge := range edges {
		if edge.to == target {
			return true
		}
	}
	return false
}

// LiveCheck1 processes the strongly connected components of the full sampled
// graph before pruning states that satisfy a recurrence predicate.
func (s *search) checkSimulationOften(property temporalProperty) (bool, error) {
	f := fairSearch{search: s, property: property, bad: make([]bool, len(s.nodes)), trigger: make([]bool, len(s.nodes))}
	if err := f.evaluateStates(); err != nil {
		return false, err
	}
	bad := f.reachableStates()
	ranks := s.simulationComponentRanks()
	components := s.components(bad)
	// checkComponents visits in reverse, so sort later full components first.
	slices.SortStableFunc(components, func(a, b []int) int {
		return cmp.Compare(ranks[b[0]], ranks[a[0]])
	})
	found, _, err := f.checkComponents(components)
	return found, err
}
