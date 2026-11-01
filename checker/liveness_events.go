package checker

import "time"

// startTemporalProgress only counts plain disk graphs: their nodes correspond
// to distinct explored states. Tableau graphs need a reachable product graph,
// which the breadth-first checker does not maintain in Java's representation.
func (s *search) startTemporalProgress() *time.Time {
	m := s.model
	if m.searchEvent == nil || !m.safeLivenessDiagnostic() || m.tlcRuntime.stopped {
		return nil
	}
	if len(s.nodes) == 0 && s.hasNoNextActions() {
		return nil
	}
	if !m.hasPlainProgressGraph() {
		return nil
	}
	started := time.Now()
	m.emitSearchEvent(SearchEvent{Kind: TemporalCheckStart, Branches: 1, Nodes: s.result.DistinctStates, GeneratedStates: s.result.GeneratedStates, DistinctStates: s.result.DistinctStates, Level: max(s.result.Depth, 1)})
	return &started
}

func (m *model) hasPlainProgressGraph() bool {
	disjuncts, err := m.liveDisjunctsBounded(1024)
	if err != nil || len(disjuncts) == 0 {
		return false
	}
	formulas, _, err := groupLiveFormulas(disjuncts)
	return err == nil && len(formulas) == 1 && formulas[0] == nil
}

// emitModelLiveness reports the number of normalized liveness solutions before
// initial-state generation, as TLC's LiveCheck does. Do not evaluate an
// effectful formula a second time solely to produce a diagnostic message.
func (m *model) emitModelLiveness() {
	if m.searchEvent == nil || !m.safeLivenessDiagnostic() {
		return
	}
	count, err := m.liveSolutionCount()
	if err == nil && count > 0 {
		m.emitSearchEvent(SearchEvent{Kind: ModelCheckingLiveness, Branches: count})
	}
}

func (m *model) safeLivenessDiagnostic() bool {
	if len(m.livenessOrder) == 0 {
		return false
	}
	for _, p := range m.livenessOrder {
		if m.unsafeIncrementalExpression(p.source) {
			return false
		}
	}
	for _, fair := range m.fairness {
		if m.unsafeIncrementalExpression(fair.action) || m.unsafeIncrementalExpression(fair.subscript) {
			return false
		}
	}
	return true
}
