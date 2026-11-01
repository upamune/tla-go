package checker

import (
	"errors"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

// TLCStateMut evaluates VIEW through FastTool even when the action is replayed
// with CallStackTool. Precomputed constants also retain FastTool provenance.
// An unsourced value reports the action's call stack instead of a value stack.
func (s *search) replayedKeyFailure(candidate *candidateViewError, index int) (Result, bool, error) {
	failure, ok := asRuntimeError(candidate.cause)
	if !ok || failure.Unexpected == nil {
		return Result{}, false, nil
	}
	if failure := s.initialKeyRenderFailure(candidate, index); failure != nil {
		return Result{Status: EvaluationFailure, Failure: failure}, true, nil
	}
	message := keyFailureMessage(failure, index)
	s.result.Status = EvaluationFailure
	s.result.Failure = &Failure{Message: message, ExitCode: 75, SuppressDepth: true}
	if failure.Unexpected.ToolCode != 0 {
		s.result.Failure.NestedToolCode = failure.Unexpected.ToolCode
		s.result.Failure.NestedMessage = failure.Message
	}
	if index < 0 {
		s.result.Failure.ToolCode = 2102
		s.result.Failure.InitialState = candidate.state
	}
	s.publishKeyFailureHeader()
	if err := s.replayKeyFailure(index); err != nil {
		return Result{}, true, err
	}
	return s.result, true, nil
}

func (s *search) initialKeyRenderFailure(candidate *candidateViewError, index int) *Failure {
	if index < 0 {
		return s.model.initialStateRenderFailure(candidate.state)
	}
	return nil
}

func keyFailureMessage(failure *evaluator.RuntimeError, index int) string {
	if index < 0 {
		return failure.Message
	}
	message := failure.Message
	if failure.Unexpected.TLCAssert {
		message = "util.Assert$TLCRuntimeException: " + message
	} else if failure.Unexpected.WrongInvocation {
		message = "util.WrongInvocationException: " + message
	}
	return unexpectedExceptionMessage(runtimeExceptionClass, message)
}

func (s *search) publishKeyFailureHeader() {
	if s.model.onFailureHeader != nil {
		s.model.presentTraces(&s.result)
		s.model.onFailureHeader(*s.result.Failure)
		s.result.Failure.HeaderReported = true
	}
}

func replayKeyCandidate(err error) (*candidateViewError, bool) {
	if candidate, ok := errors.AsType[*candidateViewError](err); ok {
		return candidate, true
	}
	return sourceLessUnionCandidate(err)
}

func (s *search) tryReplayKeyFailure(err error, index int) (Result, bool, error) {
	if candidate, ok := replayKeyCandidate(err); ok {
		return s.replayedKeyFailure(candidate, index)
	}
	return Result{}, false, nil
}

func (s *search) visitLevel(parent int) int {
	if s.replayingKeyFailure {
		return 0
	}
	return s.candidateLevel(parent)
}

// Precomputed constants may also reach state fingerprinting without a source.
func sourceLessUnionCandidate(err error) (*candidateViewError, bool) {
	candidate, ok := errors.AsType[*candidateFingerprintError](err)
	if !ok {
		return nil, false
	}
	if _, ok := errors.AsType[*value.FingerprintError](err); ok {
		return nil, false
	}
	if _, ok := errors.AsType[*value.UnionFirstElementError](err); !ok {
		return nil, false
	}
	return &candidateViewError{cause: candidate.cause, state: candidate.state, parent: candidate.parent}, true
}

func (s *search) replayKeyFailure(index int) error {
	previousFailure := s.result.Failure
	restore := s.beginKeyReplay(index)
	defer restore()
	err := s.runKeyReplay(index)
	return finishKeyReplay(previousFailure, err, index)
}

func (s *search) beginKeyReplay(index int) func() {
	m := s.model
	previousLevel := m.tlcRuntime.level
	previousFailure, previousStatus := s.result.Failure, s.result.Status
	previousGenerated := s.result.GeneratedStates
	s.result.Failure, s.result.Status = nil, Success
	s.replayingKeyFailure = true
	if index < 0 {
		s.result.GeneratedStates = 0
		m.tlcRuntime.initial = 0
	}
	m.tlcRuntime.level = 0
	return func() {
		m.tlcRuntime.level = previousLevel
		s.replayingKeyFailure = false
		s.result.Failure, s.result.Status = previousFailure, previousStatus
		if index >= 0 {
			s.result.GeneratedStates = previousGenerated
		}
	}
}

func (s *search) runKeyReplay(index int) error {
	m := s.model
	name := m.cfg.Init
	if index >= 0 {
		name = m.cfg.Next
	}
	expr, err := m.definition(name)
	if err != nil {
		return err
	}
	gen := generator{model: m, initial: index < 0, emit: s.replayKeyCandidate(index)}
	if index >= 0 {
		gen.current = s.nodes[index].state
		_, err = gen.solveNext(expr, m.partialState())
	} else {
		_, err = gen.solve(&workList{item: work{expr: expr}}, m.partialState(), "")
	}
	return err
}

func finishKeyReplay(previousFailure *Failure, err error, index int) error {
	if err == nil {
		previousFailure.ExitCode = 0
		if index >= 0 {
			previousFailure.ExitCode = 255
		}
		return nil
	}
	if replay, ok := asRuntimeError(err); ok {
		if replay.Fingerprint != nil {
			previousFailure.FingerprintDetail = fingerprintDetail(replay)
			setFingerprintCause(previousFailure, replay)
			return nil
		}
		detail := evaluationDetail(replay)
		if detail == "" {
			detail = emptyCallStackDetail
		}
		previousFailure.EvaluationDetail = detail
		return nil
	}
	return err
}

func (s *search) replayKeyCandidate(index int) func(evaluator.State, string, *parser.Expression) (bool, error) {
	return func(state evaluator.State, action string, source *parser.Expression) (bool, error) {
		more, err := s.visit(state, index, action, source)
		_, viewError := errors.AsType[*viewKeyError](err)
		_, unionError := errors.AsType[*unionKeyError](err)
		if !viewError && !unionError {
			return more, err
		}
		if _, ok := errors.AsType[*value.FingerprintError](err); ok {
			return false, err
		}
		if cause, ok := errors.AsType[*value.UnexpectedError](err); ok {
			copy := *cause
			copy.Sources = nil
			return false, &evaluator.RuntimeError{Message: copy.Message, Unexpected: &copy}
		}
		return false, err
	}
}
