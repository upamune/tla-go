package checker

// LiveCheck1's fair eventuality tableau only contains states where the goal
// is false. Once a sampled product node repeats, it no longer advances.
func (r *simulationRun) checkSimulationFairEventually(visited int, property temporalProperty) (*Result, error) {
	graph, err := r.simulationFairEventuallyGraph(visited, property)
	if err != nil || len(graph.initial) == 0 {
		return nil, err
	}
	old := r.model.tlcRuntime.search
	r.model.tlcRuntime.search = &graph
	defer func() { r.model.tlcRuntime.search = old }()
	found, err := graph.checkLivenessProperty(property)
	if err != nil || !found {
		return nil, err
	}
	result, err := graph.reportLassoProperties()
	return &result, err
}

func (r *simulationRun) simulationFairEventuallyGraph(visited int, property temporalProperty) (search, error) {
	graph := newSearch(r.model)
	graph.result = r.search.result
	graph.indices = map[string]int{}
	previous := -1
	for _, node := range r.search.nodes[:visited] {
		index, stop, err := graph.addSimulationFairEventuallyNode(node, previous, property)
		if stop || err != nil {
			return graph, err
		}
		previous = index
	}
	return graph, nil
}

func (s *search) addSimulationFairEventuallyNode(node stateNode, previous int, property temporalProperty) (int, bool, error) {
	key, err := s.model.unreducedStateKey(node.state)
	if err != nil {
		return 0, false, err
	}
	if existing, ok := s.indices[key]; ok {
		s.addSimulationFairEdge(previous, existing, node)
		return existing, true, nil
	}
	holds, err := s.model.holdsProperty(property.predicate, node.state, property.locals)
	if err != nil || holds {
		return 0, true, err
	}
	index := s.addSimulationNode(node, previous)
	s.indices[key] = index
	if previous < 0 {
		s.initial = append(s.initial, index)
	} else {
		s.addSimulationFairEdge(previous, index, node)
	}
	return index, false, nil
}

func (s *search) addSimulationFairEdge(from, to int, node stateNode) {
	if from >= 0 && !simulationEdgeExists(s.edges[from], to) {
		s.edges[from] = append(s.edges[from], graphEdge{to: to, action: node.action, source: node.source})
	}
}
