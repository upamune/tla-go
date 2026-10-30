package checker

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
)

func validActionOperator(operator *parser.Expression, arity int) error {
	switch operator.Kind {
	case "Identifier", "OperatorReference":
		return nil
	case "Lambda":
		if len(operator.Names) == arity {
			return nil
		}
	}
	return fmt.Errorf("invalid operator argument for action")
}

// Lambda parameter names are locally bound, so only its other references can
// be captured by binders in the action that receives it.
func collectArgumentNames(expr *parser.Expression, names map[string]bool) {
	if expr.Kind != "Lambda" {
		collectNames(expr, names)
		return
	}
	free := map[string]bool{}
	collectNames(expr.Args[0], free)
	for _, name := range expr.Names {
		delete(free, name)
	}
	for name := range free {
		names[name] = true
	}
}

func validateOperatorCalls(expr *parser.Expression, operators, replacements map[string]*parser.Expression) error {
	if err := validateOperatorCall(expr, operators, replacements); err != nil {
		return err
	}
	var failure error
	walkChildren(expr, func(child *parser.Expression) {
		if failure == nil {
			failure = validateOperatorCalls(child, operators, replacements)
		}
	})
	return failure
}

func validateOperatorCall(expr *parser.Expression, operators, replacements map[string]*parser.Expression) error {
	if expr.Kind != "Apply" {
		return nil
	}
	op, ok := operators[expr.Text]
	if !ok || op.Kind != "Lambda" {
		return nil
	}
	if len(expr.Args) != len(op.Names) {
		return fmt.Errorf("lambda argument arity mismatch")
	}
	names := map[string]bool{}
	for _, arg := range expr.Args {
		collectNames(substitute(arg, replacements), names)
	}
	for _, name := range op.Names {
		names[name] = true
	}
	if captures(op.Args[0], names) {
		return fmt.Errorf("action lambda requires capture-avoiding substitution")
	}
	return nil
}
