package evaluator

import (
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func (e *evaluator) setConstructor(expr *parser.Expression) (value.Value, error) {
	switch expr.Kind {
	case "FunctionSet":
		return e.functionSet(expr)
	case "RecordSet":
		return e.recordSet(expr.Fields)
	default:
		factors, err := e.evalArgs(expr.Args)
		if err != nil {
			return value.Value{}, err
		}
		return value.ProductSetWithSource(factors, e.valueSource(expr))
	}
}

func (e *evaluator) recordSet(fields []parser.Field) (value.Value, error) {
	pairs := make([]value.Pair, 0, len(fields))
	for _, field := range fields {
		set, err := e.eval(field.Value)
		if err != nil {
			return value.Value{}, err
		}
		pairs = append(pairs, value.Pair{Key: e.program.literal(field.Name), Value: set})
	}
	return value.RecordSet(pairs)
}
