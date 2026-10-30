package checker

import (
	"errors"
	"fmt"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

func (m *model) constraintHolds(expr *parser.Expression, current, next evaluator.State) (bool, error) {
	v, err := m.eval(expr, current, next)
	if err != nil {
		return false, err
	}
	result, ok := v.Bool()
	if !ok {
		r := expr.Location.Range
		message := fmt.Sprintf("TLC expected a boolean value, but did not find one. line %d, col %d to line %d, col %d of module %s", r.Start.Line, r.Start.Column, r.End.Line, r.End.Column, r.Module)
		return false, &evaluator.RuntimeError{Message: message, ToolCode: 2215, TrackSource: true}
	}
	return result, nil
}

func (s *search) nextConstraintFailure(candidate *candidateConstraintError) (Result, error) {
	failure, ok := errors.AsType[*evaluator.RuntimeError](candidate.cause)
	if !ok {
		return Result{}, candidate.cause
	}
	heading := "TLC threw an unexpected exception.\nThis was probably caused by an error in the spec or model.\nSee the User Output or TLC Console for clues to what happened.\nThe exception was a java.lang.RuntimeException\n: util.Assert$TLCRuntimeException: "
	result, err := s.runtimeFailure(&evaluator.RuntimeError{
		Message: heading + failure.Message, ToolCode: 1000,
		NestedToolCode: failure.ToolCode, NestedMessage: failure.Message,
	}, nil, 75)
	if err == nil {
		result.Failure.EvaluationDetail = "The error occurred when TLC was evaluating the nested\nexpressions at the following positions:\n    The error call stack is empty.\n"
		result.Failure.SuppressDepth = true
	}
	return result, err
}
