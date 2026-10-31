package checker

import (
	"fmt"

	"github.com/upamune/tla-go/evaluator"
)

// dfidInvariant is an INVARIANT or a state-level []P property in TLC order.
type dfidInvariant struct {
	name  string
	holds func(evaluator.State) (bool, error)
}

func (m *model) dfidInvariants() []dfidInvariant {
	var invariants []dfidInvariant
	for _, name := range m.cfg.Invariants {
		if m.trueConfigured[name] {
			continue
		}
		expr, _ := m.definition(name)
		invariants = append(invariants, dfidInvariant{name: name, holds: func(state evaluator.State) (bool, error) {
			return m.holds(expr, state, nil)
		}})
	}
	for _, p := range m.safety {
		if p.mode == stateAlways {
			invariants = append(invariants, dfidInvariant{name: m.safetyViolationName(p), holds: func(state evaluator.State) (bool, error) {
				return m.holdsProperty(p.predicate, state, p.locals)
			}})
		}
	}
	return invariants
}

func invariantFailure(name string) *Failure {
	return &Failure{Message: fmt.Sprintf("Evaluating invariant %s failed.", name), ToolCode: 2111, ExitCode: 76}
}

func actionPropertyFailure(name string) *Failure {
	return &Failure{Message: fmt.Sprintf("Evaluating action property %s failed.", name), ToolCode: 2113, ExitCode: 13}
}

// violation reports a violated invariant or action property. With -continue,
// TLC reports it and skips the remaining checks of the successor.
func (e *dfidExpansion) violation(succ *dfidNode, result Result) (bool, bool, error) {
	result.Violation.Trace = e.traceTo(succ)
	if e.search.model.continueChecking && !e.search.replaying {
		e.search.continueViolation(result)
		return true, true, nil
	}
	e.report(result, false, nil)
	return false, false, nil
}

// propertyFailure reports an invariant or action property that failed to
// evaluate. TLC then replays the expansion to report a call stack.
func (e *dfidExpansion) propertyFailure(err error, succ *dfidNode, failure *Failure) (bool, bool, error) {
	if _, ok := asRuntimeError(err); !ok {
		return false, false, err
	}
	failure.Trace = e.traceTo(succ)
	e.report(Result{Status: EvaluationFailure, Failure: failure}, true, err)
	return false, false, nil
}

// evaluationFailure reports an exception while computing successors, after
// which TLC's worker stops. With -continue, DFIDWorker reports the exception
// it catches again.
func (e *dfidExpansion) evaluationFailure(err error, succ *dfidNode) (bool, error) {
	failure, ok := asRuntimeError(err)
	if !ok {
		return false, err
	}
	result := generalFailure("computing the set of next states", failure)
	result.ExitCode, result.Trace = 255, e.traceTo(succ)
	e.report(Result{Status: EvaluationFailure, Failure: result}, true, err)
	if s := e.search; s.model.continueChecking && !s.replaying {
		s.publish(Result{Status: EvaluationFailure, Failure: generalFailure("", failure)})
	}
	e.aborted = true
	return false, nil
}

// report is DFIDModelChecker.setErrState with printTrace. During TLC's replay,
// printing the trace fails, so the first error ends the replay instead.
func (e *dfidExpansion) report(result Result, keep bool, cause error) {
	s := e.search
	if s.replaying {
		s.replayEnded(cause)
		e.aborted = true
		return
	}
	if s.done && !s.model.continueChecking {
		return
	}
	s.done, s.stopped, s.replay = true, true, nil
	if keep {
		node := e.node
		s.replay = &node
	}
	s.stop = s.publish(result)
}

// publish orders trace variables, prints them through VIEW under -view, and
// passes an error to OnError when TLC prints it.
func (s *dfidSearch) publish(result Result) Result {
	s.presentTrace(&result)
	if s.options.OnError != nil {
		s.options.OnError(s.counts(result))
	}
	return result
}

func (s *dfidSearch) continueViolation(result Result) {
	s.continued++
	if s.model.onContinuedViolation != nil {
		s.presentTrace(&result)
		s.model.onContinuedViolation(s.counts(result))
	}
}

// presentTrace prepares a depth-first trace as presentTraces does, without
// ALIAS, which TLC's depth-first checker does not apply.
func (s *dfidSearch) presentTrace(result *Result) {
	s.model.applyTraceOrder(result)
	s.model.applyActionRanges(result)
	s.model.applyViewStates(result)
}

// replayExpansion follows TLC's replay of the failing doNext with a call
// stack. It counts generated states again, and an evaluation error during
// it adds TLC's nested-expression report.
func (s *dfidSearch) replayExpansion(node dfidNode) error {
	s.replaying = true
	defer func() { s.replaying = false }()
	_, err := newDFIDExpansion(s, node).run()
	return err
}

// replayEnded records the error that ends a replay. Replays stop at their
// first error, so a later cause cannot replace it.
func (s *dfidSearch) replayEnded(cause error) {
	if s.replayCause == nil {
		s.replayCause = cause
	}
}

// nextReplayDetail is replayDetail after TLC replays doNext. Its call stack
// is empty after a util.WrongInvocationException, as in breadth-first mode.
func (s *dfidSearch) nextReplayDetail() string {
	if failure, ok := asRuntimeError(s.replayCause); ok && failure.WrongInvocation {
		return emptyCallStackDetail
	}
	return s.replayDetail()
}

// replayDetail is TLC's nested-expression report after a replay throws. TLC
// prints its header even when it cannot locate the expressions.
func (s *dfidSearch) replayDetail() string {
	if s.replayCause == nil {
		return ""
	}
	failure, ok := asRuntimeError(s.replayCause)
	if !ok {
		return ""
	}
	if detail := evaluationDetail(failure); detail != "" {
		return detail
	}
	return "The error occurred when TLC was evaluating the nested\nexpressions at the following positions:\n"
}
