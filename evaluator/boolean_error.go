package evaluator

import (
	"errors"
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

type nonBooleanError struct {
	value  value.Value
	origin *parser.Expression
}

func (e *nonBooleanError) Error() string {
	return fmt.Sprintf("expected a boolean, found %s", e.value)
}

func booleanOperatorError(err error, expr, operand *parser.Expression, operator, position string, end *parser.Expression) error {
	failure, ok := errors.AsType[*nonBooleanError](err)
	if !ok || failure.origin != operand {
		return err
	}
	message := fmt.Sprintf("Attempted to evaluate an expression of form P %s Q when %s was\n%s.", operator, position, failure.value.TLCKindDescription())
	return booleanRuntimeError(message, expr, end)
}

func booleanJunctionError(err error, expr *parser.Expression, index int) error {
	operator, position := "/\\", "Q"
	if expr.Kind == "Or" {
		operator = "\\/"
	}
	if index == 0 {
		position = "P"
	}
	end := expr.Args[min(max(1, index), len(expr.Args)-1)]
	return booleanOperatorError(err, expr, expr.Args[index], operator, position, end)
}

func booleanRuntimeError(message string, expr, end *parser.Expression) error {
	r := expr.Location.Range
	if end != nil {
		r.End = end.Location.Range.End
	}
	if r.Start.Line > 0 {
		message += fmt.Sprintf("\nline %d, col %d to line %d, col %d of module %s", r.Start.Line, r.Start.Column, r.End.Line, r.End.Column, r.Module)
	}
	return &RuntimeError{Message: message, BooleanOperator: true, TrackSource: true}
}

func booleanCaseError(err error, condition *parser.Expression) error {
	failure, ok := errors.AsType[*nonBooleanError](err)
	if !ok || failure.origin != condition {
		return err
	}
	detail := failure.value.String()
	r := condition.Location.Range
	if condition.Kind != "Number" && r.Start.Line > 0 {
		detail = fmt.Sprintf("line %d, col %d to line %d, col %d of module %s", r.Start.Line, r.Start.Column, r.End.Line, r.End.Column, r.Module)
	}
	message := fmt.Sprintf("A non-boolean expression (%s) was used as a condition of a CASE. %s", failure.value.TLCKindDescription(), detail)
	return &RuntimeError{Message: message, BooleanOperator: true, TrackSource: true}
}

func booleanQuantifierError(err error, expr, operand *parser.Expression) error {
	failure, ok := errors.AsType[*nonBooleanError](err)
	if !ok || failure.origin != operand {
		return err
	}
	r := expr.Location.Range
	message := fmt.Sprintf("TLC expected a boolean value, but did not find one. line %d, col %d to line %d, col %d of module %s", r.Start.Line, r.Start.Column, r.End.Line, r.End.Column, r.Module)
	return &RuntimeError{Message: message, ToolCode: 2215, TrackSource: true}
}
