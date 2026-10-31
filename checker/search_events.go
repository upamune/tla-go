package checker

// SearchEventKind identifies a stage of model exploration.
type SearchEventKind string

const (
	InitialStart              SearchEventKind = "initial-start"
	InitialComplete           SearchEventKind = "initial-complete"
	SimulationInitialComplete SearchEventKind = "simulation-initial-complete"
)

// SearchEvent describes initial-state generation for optional progress output.
type SearchEvent struct {
	Kind                            SearchEventKind
	GeneratedStates, DistinctStates int
}

func (m *model) emitSearchEvent(event SearchEvent) {
	if m.searchEvent != nil {
		m.searchEvent(event)
	}
}
