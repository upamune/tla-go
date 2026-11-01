package checker

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
