package evaluator

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func (e *evaluator) applyStandardBuiltin(expr *parser.Expression) (value.Value, error) {
	switch expr.BuiltinStandard {
	case "FiniteSets":
		return e.finiteSetsOperator(expr)
	case "Sequences":
		return e.sequenceOperator(expr)
	case "Bags":
		return e.bagOperator(expr)
	case "Json":
		return e.jsonOperator(expr)
	case "Randomization":
		return e.randomOperator(expr)
	case "TLCExt":
		return e.callTLCBuiltin(expr)
	default:
		return value.Value{}, fmt.Errorf("unknown standard module %s", expr.BuiltinStandard)
	}
}
