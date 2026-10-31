package checker

// SearchEventKind identifies a stage of model exploration.
type SearchEventKind string

const (
	InitialStart SearchEventKind = "initial-start"
	// InitialProgress reports the initial states generated so far when the
	// count reaches a power of two greater than one, as Java TLC's
	// DoInitFunctor does before adding the next state.
	InitialProgress           SearchEventKind = "initial-progress"
	InitialComplete           SearchEventKind = "initial-complete"
	SimulationInitialComplete SearchEventKind = "simulation-initial-complete"
	// DepthFirstLevel starts an iteration of depth-first iterative deepening.
	DepthFirstLevel SearchEventKind = "depth-first-level"
)

// SearchEvent describes initial-state generation and depth-first iterations
// for optional progress output.
type SearchEvent struct {
	Kind                            SearchEventKind
	GeneratedStates, DistinctStates int
	// Level is the depth of a DepthFirstLevel iteration.
	Level int
}

func (m *model) emitSearchEvent(event SearchEvent) {
	if m.searchEvent != nil {
		m.searchEvent(event)
	}
}
