package evaluator

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

var randomArities = map[string]int{"RandomSubset": 2, "RandomSetOfSubsets": 3, "TestRandomSetOfSubsets": 3}

func isRandomOperator(name string) bool { _, ok := randomArities[name]; return ok }

func (e *evaluator) randomOperator(expr *parser.Expression) (value.Value, error) {
	if len(expr.Args) != randomArities[expr.Text] {
		return value.Value{}, fmt.Errorf("%s expects %d arguments", expr.Text, randomArities[expr.Text])
	}
	args, err := e.evalArgs(expr.Args)
	if err != nil {
		return value.Value{}, err
	}
	return e.randomValue(expr.Text, args)
}

func (e *evaluator) randomValue(name string, args []value.Value) (value.Value, error) {
	if name == "RandomSubset" {
		return e.randomSubset(args)
	}
	return e.randomSetsOfSubsets(name, args)
}

func (e *evaluator) randomSubset(args []value.Value) (value.Value, error) {
	count, ok := args[0].Int()
	if !ok {
		return value.Value{}, randomArgumentError("first", "RandomSubset", "nonnegative integer", args[0])
	}
	if count < 0 {
		return value.Value{}, &RuntimeError{Message: fmt.Sprintf("Attempted to apply the operator overridden by the Java method\npublic static tlc2.value.impl.Value tlc2.module.Randomization.RandomSubset(tlc2.value.impl.Value,tlc2.value.impl.Value),\nbut it produced the following error:\n%d", count), Standard: true}
	}
	set := args[1]
	if !set.IsSet() || !set.IsEnumerable() {
		return value.Value{}, randomArgumentError("second", "RandomSubset", "a finite set", set)
	}
	random, err := e.randomGenerator()
	if err != nil {
		return value.Value{}, err
	}
	items, err := e.subsetElements(int(count), set, random)
	if err != nil {
		return value.Value{}, err
	}
	return e.orderedSet(items)
}

func randomArgumentError(position, name, expected string, actual value.Value) error {
	return &RuntimeError{Message: fmt.Sprintf("The %s argument of %s should be a %s, but instead it is:\n%s", position, name, expected, actual), Standard: true}
}
