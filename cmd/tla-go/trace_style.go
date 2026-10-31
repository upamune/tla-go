package main

import (
	"slices"
	"sort"

	"github.com/upamune/tla-go/checker"
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/value"
)

// traceStyle applies Java TLC's -difftrace and -terse presentation to the
// traces of a result before it is printed. The result itself is not
// modified.
type traceStyle struct {
	diff, terse, simulation bool
}

func newTraceStyle(opts options) traceStyle {
	return traceStyle{diff: opts.compat.diffTrace, terse: opts.compat.terse, simulation: opts.simulate}
}

func (s traceStyle) apply(result checker.Result) checker.Result {
	if !s.diff && !s.terse {
		return result
	}
	fullLast := s.fullLastState(result)
	if result.Violation != nil {
		violation := *result.Violation
		violation.Trace = s.trace(violation.Trace, fullLast)
		result.Violation = &violation
	}
	if result.Failure != nil {
		failure := *result.Failure
		failure.Trace = s.trace(failure.Trace, fullLast)
		failure.InitialState = s.state(failure.InitialState)
		result.Failure = &failure
	}
	return result
}

// fullLastState reports whether -difftrace prints the last trace state in
// full. Java TLC's TLCTrace prints the state whose successors were being
// generated in full, and a checked successor in full unless its predecessor
// is initial. Liveness and simulation traces print every state after the
// first as a difference from its predecessor.
func (s traceStyle) fullLastState(result checker.Result) bool {
	if s.simulation || result.Violation != nil && result.Violation.LoopStart != nil {
		return false
	}
	trace := resultTrace(result)
	return len(trace) > 2 || !endsWithSuccessor(result)
}

func resultTrace(result checker.Result) []checker.TraceStep {
	if result.Violation != nil {
		return result.Violation.Trace
	}
	if result.Failure != nil {
		return result.Failure.Trace
	}
	return nil
}

// endsWithSuccessor reports whether the trace ends with a successor state
// that was being checked, rather than with the state being expanded.
func endsWithSuccessor(result checker.Result) bool {
	if result.Failure != nil {
		code := result.Failure.ToolCode
		return code == 2109 || code == 2111 || code == 2113
	}
	return result.Status != checker.Deadlock
}

// trace styles each step. A difference is printed by TLCStateMut's
// toString(TLCState), which lists variables even under -view, so it replaces
// any VIEW text.
func (s traceStyle) trace(trace []checker.TraceStep, fullLast bool) []checker.TraceStep {
	styled := slices.Clone(trace)
	for i := range styled {
		if s.diff && i > 0 && (i < len(trace)-1 || !fullLast) {
			styled[i].State, styled[i].StateOrder = changedState(trace[i-1], trace[i])
			styled[i].View = ""
		}
		styled[i].State = s.state(styled[i].State)
	}
	return styled
}

// changedState returns the variables whose values differ from the previous
// step, together with the display order of all variables, which determines
// whether TLC prefixes each line with a conjunction.
func changedState(previous, current checker.TraceStep) (evaluator.State, []string) {
	order := current.StateOrder
	if len(order) != len(current.State) {
		order = sortedStateNames(current.State)
	}
	changed := evaluator.State{}
	for _, name := range order {
		if !sameStateValue(previous.State[name], current.State[name]) {
			changed[name] = current.State[name]
		}
	}
	return changed, order
}

func sameStateValue(a, b value.Value) bool {
	if a.Kind() == value.Invalid || b.Kind() == value.Invalid {
		return a.Kind() == b.Kind()
	}
	equal, err := value.Equal(a, b)
	return err == nil && equal
}

func sortedStateNames(state evaluator.State) []string {
	names := make([]string, 0, len(state))
	for name := range state {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// state renders values tersely in advance when -terse is set; printing a
// model value writes its text verbatim.
func (s traceStyle) state(state evaluator.State) evaluator.State {
	if !s.terse || state == nil {
		return state
	}
	styled := make(evaluator.State, len(state))
	for name, v := range state {
		styled[name] = v
		if v.Kind() == value.Invalid {
			continue
		}
		if text, err := v.TerseString(); err == nil {
			styled[name] = value.Model(text)
		}
	}
	return styled
}
