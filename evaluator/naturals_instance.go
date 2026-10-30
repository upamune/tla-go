package evaluator

import (
	"strings"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func (e *evaluator) applyNaturalBuiltin(expr *parser.Expression) (value.Value, error) {
	builtin := *expr
	builtin.Text = strings.TrimPrefix(expr.Text, "!naturals!")
	if builtin.Text == "Nat" || len(builtin.Args) != 2 {
		return e.naturalArgumentMismatch(&builtin)
	}
	return e.builtinByArity(&builtin)
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
