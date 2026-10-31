package checker

import "fmt"

// The first supported simulation liveness form is a pure eventuality without
// fairness or state reduction. Other temporal forms still fail explicitly.
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
	return property.mode == eventually && m.isStateExpression(property.predicate) &&
		!m.propertyUsesTLC(property) && !m.unsafeIncrementalExpression(property.predicate)
}

func (r *simulationRun) checkTraceLiveness(visited int) (*Result, error) {
	if len(r.model.livenessOrder) == 0 {
		return nil, nil
	}
	if visited == 0 {
		return nil, fmt.Errorf("zero-depth liveness simulation is not yet supported")
	}
	graph, err := r.traceGraph(visited)
	if err != nil {
		return nil, err
	}
	previous := r.model.tlcRuntime.search
	r.model.tlcRuntime.search = &graph
	defer func() { r.model.tlcRuntime.search = previous }()
	result, err := graph.checkLivenessGraph()
	if err != nil || result.Status != Success {
		return &result, err
	}
	return nil, nil
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
			index = len(graph.nodes)
			node.parent = previous
			graph.nodes = append(graph.nodes, node)
			graph.edges = append(graph.edges, []graphEdge{{to: index, action: "Stuttering"}})
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

func simulationEdgeExists(edges []graphEdge, target int) bool {
	for _, edge := range edges {
		if edge.to == target {
			return true
		}
	}
	return false
}
