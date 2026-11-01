package checker

func (s *search) emitResultLivenessGraph(result Result) error {
	if result.Status != Success && result.Status != LivenessViolation {
		return nil
	}
	return s.emitCompletedLivenessGraph()
}

// emitCompletedLivenessGraph reconstructs the non-tableau DiskGraph after a
// completed pure search. Java builds this graph while exploring, so reductions,
// side effects and incomplete searches must not use this reconstruction.
func (s *search) emitCompletedLivenessGraph() error {
	if !s.canReconstructLivenessGraph() {
		return nil
	}
	graph := *s
	graph.ltlFingerprints = make(map[int]uint64)
	graph.ltlVariableOrder = s.model.symmetryVariableOrder()
	graph.ltlSetMultiplier = 1
	for _, index := range s.initial {
		if err := graph.emitLivenessInitial(index); err != nil {
			return err
		}
	}
	for index := range s.nodes {
		if err := s.model.ctx.Err(); err != nil {
			return err
		}
		if err := graph.emitLivenessSuccessors(index); err != nil {
			return err
		}
	}
	return nil
}

func (s *search) canReconstructLivenessGraph() bool {
	m := s.model
	if len(m.livenessOrder) == 0 || !s.completedLivenessGraphEligible() {
		return false
	}
	if !m.pureLivenessDumpModel() {
		return false
	}
	return m.hasSinglePlainLivenessSolution()
}

func (m *model) hasSinglePlainLivenessSolution() bool {
	disjuncts, err := m.liveDisjunctsBounded(1024)
	if err != nil || len(disjuncts) == 0 {
		return false
	}
	temporal, _, err := groupLiveFormulas(disjuncts)
	return err == nil && len(temporal) == 1 && temporal[0] == nil
}

func (s *search) completedLivenessGraphEligible() bool {
	m := s.model
	return m.livenessGraph != nil && !m.hasReduction() && !m.tlcRuntime.stopped && s.head == len(s.nodes) && len(s.initial) != 0 && !m.continueChecking && s.safeLivenessGraphBindings()
}

func (s *search) safeLivenessGraphBindings() bool {
	return !hasOperatorConstants(s.model.module.Constants) && len(s.model.sourceConfig.Substitutions) == 0 && len(s.model.cfg.Substitutions) == 0 && len(s.model.module.Instances) == 0
}

func (s *search) emitLivenessInitial(index int) error {
	node := s.nodes[index]
	fingerprint, err := s.model.javaFingerprint(node.state, s.ltlVariableOrder)
	if err != nil {
		return err
	}
	return s.model.livenessGraph(GraphEvent{State: node.state, Order: s.ltlVariableOrder, Fingerprint: fingerprint, RankFingerprint: fingerprint, Depth: node.depth, Initial: true, New: true})
}

func (s *search) emitLivenessSuccessors(index int) error {
	choices, err := s.incrementalSuccessors(index)
	if err != nil {
		return err
	}
	source := s.nodes[index]
	parent, err := s.model.javaFingerprint(source.state, s.ltlVariableOrder)
	if err != nil {
		return err
	}
	for _, choice := range choices {
		edge := s.edges[index][choice]
		state := s.edgeNext(edge)
		fingerprint, err := s.model.javaFingerprint(state, s.ltlVariableOrder)
		if err != nil {
			return err
		}
		event := GraphEvent{State: state, Order: s.ltlVariableOrder, Fingerprint: fingerprint, Parent: parent, RankFingerprint: parent, Depth: source.depth + 1, New: edge.to >= index}
		if err := s.model.livenessGraph(event); err != nil {
			return err
		}
	}
	return nil
}
