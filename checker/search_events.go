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
	// SimulationLiveness reports the number of solutions of TLC's liveness
	// normal form before simulation computes the initial states.
	SimulationLiveness SearchEventKind = "simulation-liveness"
	// ModelCheckingLiveness reports the number of liveness solutions before
	// breadth-first model checking computes the initial states.
	ModelCheckingLiveness SearchEventKind = "model-checking-liveness"
	// TemporalCheckStart reports a final, complete-space liveness check.
	TemporalCheckStart SearchEventKind = "temporal-check-start"
	// TemporalCheckUncertain invalidates the preceding start observation when
	// Go's deferred liveness evaluation fails after reporting it.
	TemporalCheckUncertain SearchEventKind = "temporal-check-uncertain"
)

// SearchEvent describes initial-state generation and depth-first iterations
// for optional progress output.
type SearchEvent struct {
	Kind                            SearchEventKind
	GeneratedStates, DistinctStates int
	// Level is the depth of a DepthFirstLevel iteration.
	Level int
	// Branches is the solution count of a liveness event.
	Branches int
	// Nodes is the total number of nodes across the liveness graphs.
	Nodes int
}

func (m *model) emitSearchEvent(event SearchEvent) {
	if m.searchEvent != nil {
		m.searchEvent(event)
	}
}
