package checker

import (
	"errors"
	"fmt"
	"slices"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/value"
)

func (s *search) runtimeFailure(err error, trace []TraceStep, code int) (Result, error) {
	failure, ok := asRuntimeError(err)
	if !ok {
		return Result{}, err
	}
	s.result.Status = EvaluationFailure
	s.result.Failure = &Failure{Message: failure.Message, Trace: trace, ExitCode: code, FingerprintDetail: fingerprintDetail(failure), EvaluationDetail: evaluationDetail(failure), SuppressDepth: failure.Unexpected != nil || len(failure.Sources) > 0}
	if len(trace) > 0 {
		s.result.Failure.ToolCode = failure.ToolCode
	}
	if candidate, ok := errors.AsType[*candidateFingerprintError](err); ok && candidate.parent < 0 {
		s.result.Failure.InitialState = candidate.state
	}
	return s.result, nil
}

func (s *search) nextRuntimeFailure(err error, index int) (Result, error) {
	if incomplete, ok := errors.AsType[*incompleteState](err); ok {
		return s.incompleteSuccessorFailure(incomplete, index)
	}
	failure, ok := asRuntimeError(err)
	if !ok {
		return Result{}, err
	}
	if missing, ok := errors.AsType[*evaluator.UnboundVariable](err); ok {
		failure.Unexpected = &value.UnexpectedError{Message: failure.Message, Sources: missing.Sources}
	}
	if failure.Unexpected != nil {
		failure = nextUnexpectedFailure(failure)
	}
	if _, ok := errors.AsType[*candidateFingerprintError](err); ok {
		return s.runtimeFailure(failure, nil, actionFailureCode(failure))
	}
	trace, recoveryErr := s.recoverFailureTrace(index)
	if recoveryErr != nil {
		return Result{}, recoveryErr
	}
	return s.runtimeFailure(failure, trace, actionFailureCode(failure))
}

func asRuntimeError(err error) (*evaluator.RuntimeError, bool) {
	if failure, ok := errors.AsType[*evaluator.RuntimeError](err); ok {
		return failure, true
	}
	if missing, ok := errors.AsType[*evaluator.UnboundVariable](err); ok {
		return unboundVariableFailure(missing), true
	}
	return evaluator.ValueRuntimeError(err)
}

func unboundVariableFailure(missing *evaluator.UnboundVariable) *evaluator.RuntimeError {
	message := fmt.Sprintf("In evaluation, the identifier %s is either undefined or not an operator.", missing.Name)
	if len(missing.Sources) > 0 {
		r := missing.Sources[0].Location.Range
		message += fmt.Sprintf("\nline %d, col %d to line %d, col %d of module %s", r.Start.Line, r.Start.Column, r.End.Line, r.End.Column, r.Module)
	}
	return &evaluator.RuntimeError{Message: message, Sources: missing.Sources}
}

func nextUnexpectedFailure(failure *evaluator.RuntimeError) *evaluator.RuntimeError {
	result := *failure
	cause := failure.Message
	if failure.Unexpected.TLCAssert && failure.Fingerprint != nil {
		cause = "util.Assert$TLCRuntimeException: " + cause
	}
	result.Message = "TLC threw an unexpected exception.\nThis was probably caused by an error in the spec or model.\nSee the User Output or TLC Console for clues to what happened.\nThe exception was a java.lang.RuntimeException\n: " + cause
	return &result
}

func fingerprintDetail(failure *evaluator.RuntimeError) string {
	if failure.Fingerprint == nil {
		return ""
	}
	return fmt.Sprintf("TLC was unable to fingerprint.\n\nFingerprint Stack Trace:\n%s\nReason:\n%s", failure.Fingerprint.Trace(), failure.Fingerprint.Error())
}

type candidateFingerprintError struct {
	cause  error
	state  evaluator.State
	parent int
}

func candidateFailure(err error, state evaluator.State, parent int) error {
	if _, ok := errors.AsType[*value.FingerprintError](err); ok {
		return &candidateFingerprintError{cause: err, state: state, parent: parent}
	}
	return err
}

func (e *candidateFingerprintError) Error() string { return e.cause.Error() }
func (e *candidateFingerprintError) Unwrap() error { return e.cause }

func actionFailureCode(failure *evaluator.RuntimeError) int {
	if failure.ExitCode != 0 {
		return failure.ExitCode
	}
	if failure.Standard {
		return 255
	}
	return 75
}

func (s *search) propertyRuntimeFailure(err error, name string, state evaluator.State, parent int, action string) (bool, error) {
	if parent < 0 {
		return s.invariantRuntimeFailure(err, name, state, parent, action)
	}
	failure, ok := errors.AsType[*evaluator.RuntimeError](err)
	if !ok {
		return false, err
	}
	s.result.Status = EvaluationFailure
	s.result.Failure = &Failure{
		Message:        fmt.Sprintf("Evaluating action property %s failed.\n%s", name, failure.Message),
		Trace:          append(s.trace(parent), TraceStep{Action: action, State: state, ActionRange: actionSourceRange(s.actionSource)}),
		ExitCode:       actionFailureCode(failure),
		ToolCode:       2113,
		NestedToolCode: failure.ToolCode,
	}
	return false, nil
}

func (s *search) invariantRuntimeFailure(err error, name string, state evaluator.State, parent int, action string) (bool, error) {
	failure, ok := errors.AsType[*evaluator.RuntimeError](err)
	if !ok {
		return false, err
	}
	result := &Failure{Message: failure.Message, ExitCode: 75}
	if parent < 0 {
		if len(failure.Sources) > 0 {
			result.evaluationSources = slices.Clone(failure.Sources)
			result.SuppressDepth = true
		}
		if !failure.Standard {
			result.InitialState = state
		}
	} else {
		result.Message = fmt.Sprintf("Evaluating invariant %s failed.\n%s", name, failure.Message)
		result.ToolCode = 2111
		result.NestedToolCode = failure.ToolCode
		result.Trace = append(s.trace(parent), TraceStep{Action: action, State: state, ActionRange: actionSourceRange(s.actionSource)})
		result.ExitCode = 76
		s.result.HasOutDegrees = true
		if !s.candidateInModel {
			result.ExitCode = actionFailureCode(failure)
			result.EvaluationDetail = evaluationDetail(failure)
			result.SuppressDepth = true
		}
	}
	s.result.Status = EvaluationFailure
	s.result.Failure = result
	return false, nil
}
