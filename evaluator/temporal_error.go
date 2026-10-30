package evaluator

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func temporalEvaluationError(expr *parser.Expression) (value.Value, error) {
	name := temporalFormulaName(expr)
	r := expr.Location.Range
	message := fmt.Sprintf("TLC encountered a temporal formula (%s) when evaluating a predicate or action.\nline %d, col %d to line %d, col %d of module %s", name, r.Start.Line, r.Start.Column, r.End.Line, r.End.Column, r.Module)
	return value.Value{}, &RuntimeError{Message: message, ToolCode: 2261, Unexpected: &value.UnexpectedError{Message: message}}
}

func temporalFormulaName(expr *parser.Expression) string {
	if expr.Text == "-+->" {
		return "a -+-> formula"
	}
	switch expr.Kind {
	case "Always":
		return "[]A"
	case "Eventually":
		return "<>A"
	case "WeakFairness":
		return "WF"
	case "StrongFairness":
		return "SF"
	default:
		return "a ~> b"
	}
}
