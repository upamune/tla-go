package checker

import (
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/value"
)

// Java TLC's simulation Trace omits adjacent identical states. A candidate is
// appended to the current behavior unless it is already the current state.
func (r *tlcRuntime) simulationBehavior(state evaluator.State) []evaluator.State {
	path := r.tracePath(r.traceCurrent)
	if r.traceCurrent < 0 || traceStateID(state) != traceStateID(r.search.nodes[r.traceCurrent].state) {
		path = append(path, state)
	}
	result := make([]evaluator.State, 0, len(path))
	for _, step := range path {
		if len(result) == 0 || !r.sameSimulationState(result[len(result)-1], step) {
			result = append(result, step)
		}
	}
	return result
}

func (r *tlcRuntime) sameSimulationState(a, b evaluator.State) bool {
	for _, name := range r.model.names {
		equal, err := value.Equal(a[name], b[name])
		if err != nil || !equal {
			return false
		}
	}
	return true
}

func (m *model) finishSimulationResult(result *Result) {
	if result.Failure != nil {
		result.Failure.Trace = m.compactSimulationTrace(result.Failure.Trace)
	}
	if result.Violation != nil && result.Violation.LoopStart == nil {
		result.Violation.AfterInitial = len(result.Violation.Trace) > 1
		result.Violation.Trace = m.compactSimulationTrace(result.Violation.Trace)
	}
	m.presentTraces(result)
}

func (m *model) compactSimulationTrace(trace []TraceStep) []TraceStep {
	if len(trace) < 2 {
		return trace
	}
	result := make([]TraceStep, 0, len(trace))
	for _, step := range trace {
		if len(result) == 0 || !m.tlcRuntime.sameSimulationState(result[len(result)-1].State, step.State) {
			result = append(result, step)
		}
	}
	return result
}
