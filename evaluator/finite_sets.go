package evaluator

import (
	"errors"
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
	return e.finiteSetsValue(expr.Text, v)
}

func (e *evaluator) finiteSetsValue(name string, v value.Value) (value.Value, error) {
	if name == "IsFiniteSet" {
		finite, err := v.IsFinite()
		return value.Bool(finite), err
	}
	size, err := v.Cardinality()
	if err != nil {
		if overflow, ok := errors.AsType[*value.CardinalityOverflowError](err); ok {
			return value.Value{}, cardinalityOverflow(overflow.Set, err)
		}
		return value.Value{}, err
	}
	if err := e.program.checkDeferredCardinality(v, size); err != nil {
		return value.Value{}, err
	}
	return value.Integer(size)
}

func cardinalityOverflow(v value.Value, err error) error {
	text, renderErr := v.StrictString()
	if renderErr != nil {
		return err
	}
	message := fmt.Sprintf("Attempted to apply the operator overridden by the Java method\npublic static tlc2.value.impl.IntValue tlc2.module.FiniteSets.Cardinality(tlc2.value.impl.Value),\nbut it produced the following error:\nOverflow when computing the number of elements in:\n%s", text)
	return &RuntimeError{Message: message, Legacy: err.Error(), Standard: true}
}
