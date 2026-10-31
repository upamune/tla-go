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
	s.result.Failure = &Failure{Message: failure.Message, Trace: trace, ExitCode: code, NestedToolCode: failure.NestedToolCode, NestedMessage: failure.NestedMessage, FingerprintDetail: fingerprintDetail(failure), EvaluationDetail: evaluationDetail(failure), SuppressDepth: failure.Unexpected != nil || len(failure.Sources) > 0}
	setFingerprintCause(s.result.Failure, failure)
	if failure.ToolCode == 2261 && failure.NestedToolCode == 0 {
		s.result.Failure.NestedMessage = failure.Message
		s.result.Failure.NestedToolCode = failure.ToolCode
	}
	if len(trace) > 0 {
		s.result.Failure.ToolCode = failure.ToolCode
	} else {
		setInitialNestedFailure(s.result.Failure, failure)
	}
	setInitialCandidateFailure(s.result.Failure, failure, err)
	if failure.WrongInvocation {
		s.result.Failure.EvaluationDetail, s.result.Failure.SuppressDepth = emptyCallStackDetail, true
	}
	return s.result, nil
}

// emptyCallStackDetail is Java TLC's 2103 message when its call stack recorded
// no expression for the failure.
const emptyCallStackDetail = "The error occurred when TLC was evaluating the nested\nexpressions at the following positions:\n    The error call stack is empty.\n"

func setInitialCandidateFailure(result *Failure, failure *evaluator.RuntimeError, err error) {
	if candidate, ok := errors.AsType[*candidateFingerprintError](err); ok && candidate.parent < 0 {
		result.InitialState = candidate.state
	}
	if candidate, ok := errors.AsType[*candidateConstraintError](err); ok && candidate.parent < 0 {
		result.InitialState = candidate.state
		result.ToolCode = 2102
		result.NestedToolCode = failure.ToolCode
		result.NestedMessage = failure.Message
	}
}

func setFingerprintCause(result *Failure, failure *evaluator.RuntimeError) {
	if failure.Fingerprint != nil && failure.Unexpected != nil {
		result.FingerprintCause = failure.Unexpected.Message
		result.FingerprintCauseCode = failure.Unexpected.ToolCode
	}
}

func setInitialNestedFailure(result *Failure, failure *evaluator.RuntimeError) {
	if slices.Contains([]int{2132, 2154, 2215, 2247, 2266}, failure.ToolCode) {
		result.NestedToolCode = failure.ToolCode
		result.NestedMessage = failure.Message
		result.evaluationSources = slices.Clone(failure.Sources)
	}
}

func (s *search) nextRuntimeFailure(err error, index int) (Result, error) {
	if incomplete, ok := errors.AsType[*incompleteState](err); ok {
		return s.incompleteSuccessorFailure(incomplete, index)
	}
	if candidate, ok := errors.AsType[*candidateConstraintError](err); ok {
		return s.nextConstraintFailure(candidate)
	}
	failure, ok := asRuntimeError(err)
	if !ok {
		return Result{}, err
	}
	if missing, ok := errors.AsType[*evaluator.UnboundVariable](err); ok {
		failure.Unexpected = &value.UnexpectedError{Message: failure.Message, Sources: missing.Sources}
	}
	failure = normalizeNextFailure(failure)
	if _, ok := errors.AsType[*candidateFingerprintError](err); ok || failure.WrongInvocation {
		return s.runtimeFailure(failure, nil, actionFailureCode(failure))
	}
	trace, recoveryErr := s.recoverFailureTrace(index)
	if recoveryErr != nil {
		return Result{}, recoveryErr
	}
	return s.runtimeFailure(failure, trace, actionFailureCode(failure))
}

func normalizeNextFailure(failure *evaluator.RuntimeError) *evaluator.RuntimeError {
	if failure.Unexpected != nil {
		failure = nextUnexpectedFailure(failure)
	}
	if failure.ToolCode == 2247 || failure.ToolCode == 2154 {
		failure = nextNestedGenerationFailure(failure)
	}
	if failure.WrongInvocation {
		result := *failure
		result.Message = wrongInvocationMessage(failure.Message)
		failure = &result
	}
	return failure
}

// nextNestedGenerationFailure reports a TLC assertion raised while generating
// successors, such as a non-boolean action (2247) or a Java override failure
// (2154), inside Java TLC's unexpected-exception message.
func nextNestedGenerationFailure(failure *evaluator.RuntimeError) *evaluator.RuntimeError {
	result := *failure
	result.Message = unexpectedExceptionMessage(runtimeExceptionClass, failure.Message)
	result.NestedMessage, result.NestedToolCode, result.ToolCode = failure.Message, failure.ToolCode, 1000
	return &result
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
	result.Message = unexpectedExceptionMessage(runtimeExceptionClass, cause)
	if failure.ToolCode == 2261 {
		result.NestedMessage = cause
		result.NestedToolCode, result.ToolCode = failure.ToolCode, 1000
	} else if failure.Fingerprint != nil && failure.Unexpected.ToolCode != 0 {
		result.NestedMessage = failure.Unexpected.Message
		result.NestedToolCode = failure.Unexpected.ToolCode
	}
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
	if failure, ok := errors.AsType[*evaluator.RuntimeError](err); ok && (failure.ToolCode == 2215 || failure.ToolCode == 2154) {
		return &candidateConstraintError{cause: err, state: state, parent: parent}
	}
	return err
}

type candidateConstraintError struct {
	cause  error
	state  evaluator.State
	parent int
}

func (e *candidateConstraintError) Error() string { return e.cause.Error() }
func (e *candidateConstraintError) Unwrap() error { return e.cause }

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
		setInitialInvariantFailure(result, failure, state)
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

func setInitialInvariantFailure(result *Failure, failure *evaluator.RuntimeError, state evaluator.State) {
	setInitialNestedFailure(result, failure)
	if failure.BooleanOperator || failure.ToolCode == 2215 || failure.ToolCode == 2154 || failure.WrongInvocation {
		result.ToolCode = 2102
	}
	if len(failure.Sources) > 0 {
		result.evaluationSources = slices.Clone(failure.Sources)
		result.SuppressDepth = true
	}
	if failure.WrongInvocation {
		result.EvaluationDetail, result.SuppressDepth = emptyCallStackDetail, true
	}
	if !failure.Standard {
		result.InitialState = state
	}
}
