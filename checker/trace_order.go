package checker

import (
	"slices"

	"github.com/upamune/tla-go/evaluator"
)

// TLC prints state variables in the order produced by its symbol table.
// Preserve any order already supplied by an ALIAS.
func (m *model) applyTraceOrder(result *Result) {
	if len(m.names) < 2 || result.Violation == nil && result.Failure == nil {
		return
	}
	order := m.symmetryVariableOrder()
	if result.Violation != nil {
		applyTraceStepOrder(result.Violation.Trace, order)
	}
	if result.Failure != nil {
		applyTraceStepOrder(result.Failure.Trace, order)
		applyTraceStepOrder(result.Failure.TemporalTrace, order)
		if hasStateOrder(result.Failure.InitialState, order) {
			result.Failure.InitialStateOrder = slices.Clone(order)
		}
	}
}

func applyTraceStepOrder(trace []TraceStep, order []string) {
	for i := range trace {
		if len(trace[i].StateOrder) == 0 && hasStateOrder(trace[i].State, order) {
			trace[i].StateOrder = slices.Clone(order)
		}
	}
}

func hasStateOrder(state evaluator.State, order []string) bool {
	if state == nil || len(state) != len(order) {
		return false
	}
	for _, name := range order {
		if _, ok := state[name]; !ok {
			return false
		}
	}
	return true
}
