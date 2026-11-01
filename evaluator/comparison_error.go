package evaluator

import (
	"fmt"

	"github.com/upamune/tla-go/value"
)

func comparisonCheckedBinary(name string, left, right value.Value, naturalOnly bool) (value.Value, error) {
	if name == "\\div" && naturalOnly {
		a, err := integer(left)
		if err != nil {
			return value.Value{}, err
		}
		b, err := integer(right)
		if err != nil {
			return value.Value{}, err
		}
		return naturalDivide(a, b)
	}
	if isComparisonOperator(name) {
		if err := checkComparisonArguments(name, left, right, naturalOnly); err != nil {
			return value.Value{}, err
		}
	}
	return binary(name, left, right)
}

func isComparisonOperator(name string) bool {
	return name == "<" || name == "<=" || name == ">" || name == ">="
}

func checkComparisonArguments(name string, left, right value.Value, naturalOnly bool) error {
	if name == ">=" && naturalOnly {
		name = ">"
	}
	if _, ok := left.Int(); !ok {
		return comparisonArgumentError(name, "first", left)
	}
	if _, ok := right.Int(); !ok {
		return comparisonArgumentError(name, "second", right)
	}
	return nil
}

func comparisonArgumentError(operator, position string, v value.Value) error {
	return &RuntimeError{
		Message:  fmt.Sprintf("The %s argument of %s should be an integer, but instead it is:\n%s", position, operator, v),
		Legacy:   fmt.Sprintf("expected an integer, found %s", v),
		ToolCode: 2266, Standard: true, TrackSource: true,
	}
}
