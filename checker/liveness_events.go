package checker

import "time"

// startTemporalProgress counts Java's disk-graph table entries, not product
// nodes. TableauNodePtrTable counts fingerprints, including setDone entries
// without a consistent tableau node. Completed pure, unreduced exploration
// visits every such fingerprint; replay of a done state cannot introduce more.
func (s *search) startTemporalProgress() *time.Time {
	m := s.model
	if m.searchEvent == nil || !m.safeLivenessDiagnostic() || m.tlcRuntime.stopped {
		return nil
	}
	if len(s.nodes) == 0 && s.hasNoNextActions() {
		return nil
	}
	branches, tableau := m.progressBranches()
	if !m.validProgressBranches(branches, tableau) {
		return nil
	}
	started := time.Now()
	m.emitSearchEvent(SearchEvent{Kind: TemporalCheckStart, Branches: branches, Nodes: branches * len(s.nodes), GeneratedStates: s.result.GeneratedStates, DistinctStates: s.result.DistinctStates, Level: max(s.result.Depth, 1)})
	return &started
}

func (m *model) validProgressBranches(branches int, tableau bool) bool {
	return branches > 0 && (!tableau || m.safeTableauProgress())
}

func (m *model) safeTableauProgress() bool {
	if m.hasReduction() || m.environmentIsImpure() {
		return false
	}
	// A late tableau node makes Java regenerate successors and recheck both
	// constraints. Reject unstable definitions, including constant helpers.
	for _, def := range m.defs {
		if m.unsafeIncrementalExpression(def.Body) {
			return false
		}
	}
	return true
}

func (m *model) progressBranches() (int, bool) {
	disjuncts, err := m.liveDisjunctsBounded(1024)
	if err != nil || len(disjuncts) == 0 {
		return 0, false
	}
	formulas, _, err := groupLiveFormulas(disjuncts)
	if err != nil {
		return 0, false
	}
	for _, formula := range formulas {
		if formula != nil {
			return len(formulas), true
		}
	}
	return len(formulas), false
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
