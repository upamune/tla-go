package checker

import (
	"errors"
	"fmt"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func (m *model) nonBooleanPredicateError(expr *parser.Expression, v value.Value) error {
	if expr.Kind == "Identifier" && expr.Location.Range.Start.Line == 0 {
		if definition, ok := m.defs[expr.Text]; ok {
			expr = definition.Body
		}
	}
	detail := v.String()
	if m.readsState(expr, map[string]bool{}) || m.hasAction(expr, map[string]bool{}) {
		r := expr.Location.Range
		if r.Start.Line > 0 {
			detail = fmt.Sprintf("line %d, col %d to line %d, col %d of module %s", r.Start.Line, r.Start.Column, r.End.Line, r.End.Column, r.Module)
		}
	}
	return &evaluator.RuntimeError{Message: "TLC expected a boolean value, but did not find one. " + detail, ToolCode: 2215, TrackSource: true, Sources: []*parser.Expression{expr}}
}

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
	heading := unexpectedExceptionHeading("java.lang.RuntimeException") + " util.Assert$TLCRuntimeException: "
	result, err := s.runtimeFailure(&evaluator.RuntimeError{
		Message: heading + failure.Message, ToolCode: 1000,
		NestedToolCode: failure.ToolCode, NestedMessage: failure.Message,
	}, nil, 75)
	if err == nil {
		result.Failure.EvaluationDetail = emptyCallStackDetail
		result.Failure.SuppressDepth = true
	}
	return result, err
}
