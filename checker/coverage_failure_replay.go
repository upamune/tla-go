package checker

import (
	"errors"
	"fmt"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

// Successor-expression errors use a different CallStackTool replay from
// invariant evaluation errors. Do not publish its unmodeled coverage costs.
func (s *search) coverageAwareNextFailure(cause error, index int) (Result, error) {
	result, err := s.nextRuntimeFailure(cause, index)
	if err == nil && s.coverageCandidateEvalFailure(cause) {
		return result, s.replayCoverageCandidateFailure(cause, index)
	}
	if err == nil && s.coverageUnmodeledNextFailure(cause) {
		return result, fmt.Errorf("-coverage cannot report successor expression error replay")
	}
	return result, err
}

func (s *search) coverageCandidateEvalFailure(cause error) bool {
	if s.model.coverage == nil {
		return false
	}
	_, candidate := errors.AsType[*candidateEvalError](cause)
	_, constraint := errors.AsType[*constraintEvaluationError](cause)
	return candidate && constraint
}

func (s *search) coverageUnmodeledNextFailure(cause error) bool {
	if s.model.coverage == nil || s.model.coverage.hasExistential || coverageCandidateFailure(cause) {
		return false
	}
	_, ok := asRuntimeError(cause)
	return ok
}

func coverageCandidateFailure(err error) bool {
	_, fingerprint := errors.AsType[*candidateFingerprintError](err)
	_, constraint := errors.AsType[*candidateConstraintError](err)
	_, view := errors.AsType[*candidateViewError](err)
	_, incomplete := errors.AsType[*incompleteState](err)
	return fingerprint || constraint || view || incomplete
}

// Java reconstructs a successor-invariant error trace before it repeats the
// failing state expansion with CallStackTool. Both runs charge coverage.
func (s *search) replayCoverageFailureTrace() error {
	if !s.coverageSafetyFailureReplay() || len(s.result.Failure.Trace) < 2 {
		return nil
	}
	if s.model.lassoReplayHasEffects() || s.model.coverageFailureConstraintEffects() {
		return fmt.Errorf("-coverage cannot report an effectful safety error recovery")
	}
	return s.coverageSafetyTrace(Result{Status: InvariantViolation, Violation: &Violation{Trace: s.result.Failure.Trace}})
}

// The CLI emits the invariant error after checking finishes. Replaying an
// effectful constraint here would put its output before Java's error message.
func (m *model) coverageFailureConstraintEffects() bool {
	for _, name := range m.cfg.Constraints {
		expr, err := m.definition(name)
		if err != nil || m.expressionUsesTLC(expr, map[string]bool{}) || m.expressionHasEffects(expr, map[string]bool{}) || m.readsRandom(expr, map[string]bool{}) {
			return true
		}
	}
	return false
}

type coverageFailureCandidate struct {
	state  evaluator.State
	action string
	source *parser.Expression
}

// CallStackTool generates an action's complete StateVec before checking any
// candidate. Buffer one action at a time so counters and the stopping point
// follow that order, including errors during candidate checking.
func (s *search) replayCoveredSafetyFailure(index int, next *parser.Expression, seen map[string]bool) error {
	var pending []coverageFailureCandidate
	flush := func() (bool, error) {
		return s.flushCoverageFailureCandidates(index, seen, &pending)
	}
	gen := generator{model: s.model, current: s.nodes[index].state, actionStart: flush, emit: func(state evaluator.State, action string, source *parser.Expression) (bool, error) {
		pending = append(pending, coverageFailureCandidate{state: state, action: action, source: source})
		return true, nil
	}}
	more, err := gen.solveNext(next, s.model.partialState())
	if err == nil && more {
		_, err = flush()
	}
	return s.finishCoverageFailureReplay(err)
}

func (s *search) flushCoverageFailureCandidates(index int, seen map[string]bool, pending *[]coverageFailureCandidate) (bool, error) {
	batch := *pending
	*pending = nil
	for _, candidate := range batch {
		action := candidate.action
		if action == "" {
			action = s.model.cfg.Next
		}
		s.model.coverage.generated(action, false)
	}
	for _, candidate := range batch {
		more, err := s.replaySafetyCandidate(index, candidate.state, candidate.action, candidate.source, seen)
		if err != nil || !more {
			return more, err
		}
	}
	return true, nil
}

func (s *search) finishCoverageFailureReplay(err error) error {
	if err == nil {
		return nil
	}
	if replay, ok := asRuntimeError(err); ok {
		failure := s.result.Failure
		failure.ExitCode = actionFailureCode(replay)
		failure.EvaluationDetail = evaluationDetail(replay)
		failure.SuppressDepth = true
		return nil
	}
	return err
}
