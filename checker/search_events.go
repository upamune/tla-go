package checker

// SearchEventKind identifies a stage of model exploration.
type SearchEventKind string

const (
	InitialStart              SearchEventKind = "initial-start"
	InitialComplete           SearchEventKind = "initial-complete"
	SimulationInitialComplete SearchEventKind = "simulation-initial-complete"
	// SimulationLiveness reports the number of solutions of TLC's liveness
	// normal form before simulation computes the initial states.
	SimulationLiveness SearchEventKind = "simulation-liveness"
)

// SearchEvent describes initial-state generation for optional progress output.
type SearchEvent struct {
	Kind                            SearchEventKind
	GeneratedStates, DistinctStates int
	Branches                        int
}

func (m *model) emitSearchEvent(event SearchEvent) {
	if m.searchEvent != nil {
		m.searchEvent(event)
	}
}
