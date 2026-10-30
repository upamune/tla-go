package checker

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
)

// These built-ins accept constant, state, and action arguments, but not
// temporal formulas, as specified by SANY's BuiltInLevel table.
func (m *model) temporalValueArguments(expr *parser.Expression) error {
	name := temporalValueOperator(expr)
	if name == "" {
		return nil
	}
	for i, arg := range expr.Args {
		if m.hasTemporal(arg, map[string]bool{}) {
			return fmt.Errorf("Level error in applying operator %s: The level of argument %d exceeds the maximum level allowed by the operator", name, i+1)
		}
	}
	return nil
}

func temporalValueOperator(expr *parser.Expression) string {
	switch expr.Kind {
	case "Tuple":
		return "$Tuple"
	case "SetEnumeration":
		return "$SetEnumerate"
	case "FunctionSet":
		return "$SetOfFcns"
	case "Apply":
		return temporalValueInfix(expr.Text)
	}
	return ""
}

func temporalValueInfix(name string) string {
	switch name {
	case "#":
		return "/="
	case "=", "\\in", "\\notin", "\\subseteq", "\\cup", "\\cap", "\\", "DOMAIN", "SUBSET", "UNION":
		return name
	default:
		return ""
	}
}
