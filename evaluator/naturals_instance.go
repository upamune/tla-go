package evaluator

import (
	"strings"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func (e *evaluator) applyNaturalBuiltin(expr *parser.Expression) (value.Value, error) {
	name := strings.TrimPrefix(expr.Text, "!naturals!")
	if name == "Nat" || name == "Int" || len(expr.Args) != 2 && !(name == "-." && len(expr.Args) == 1) {
		builtin := *expr
		builtin.Text = name
		return e.naturalArgumentMismatch(&builtin)
	}
	left, err := e.eval(expr.Args[0])
	if err != nil {
		return value.Value{}, err
	}
	if len(expr.Args) == 1 {
		return unary(name, left)
	}
	return e.naturalBinary(name, left, expr.Args[1])
}

func (e *evaluator) naturalBinary(name string, left value.Value, argument *parser.Expression) (value.Value, error) {
	right, err := e.eval(argument)
	if err != nil {
		return value.Value{}, err
	}
	result, err := comparisonCheckedBinary(name, left, right, true)
	if err != nil {
		return result, err
	}
	return e.orderSetResult(name, result)
}

func (e *evaluator) naturalArgumentMismatch(expr *parser.Expression) (value.Value, error) {
	if _, err := e.evalArgs(expr.Args); err != nil {
		return value.Value{}, err
	}
	message := "array is not of length 2"
	if expr.Text == "Nat" {
		message = "cannot convert MethodHandle()Value to (Value[])Value"
	}
	return value.Value{}, &RuntimeError{Message: message, Standard: true}
}
