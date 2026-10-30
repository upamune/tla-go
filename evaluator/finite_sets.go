package evaluator

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func (e *evaluator) finiteSetsOperator(expr *parser.Expression) (value.Value, error) {
	if len(expr.Args) != 1 {
		return value.Value{}, fmt.Errorf("%s expects one argument", expr.Text)
	}
	v, err := e.eval(expr.Args[0])
	if err != nil {
		return value.Value{}, err
	}
	return finiteSetsValue(expr.Text, v)
}

func finiteSetsValue(name string, v value.Value) (value.Value, error) {
	if name == "IsFiniteSet" {
		finite, err := v.IsFinite()
		return value.Bool(finite), err
	}
	size, err := v.Cardinality()
	if err != nil {
		return value.Value{}, err
	}
	return value.Integer(size)
}
