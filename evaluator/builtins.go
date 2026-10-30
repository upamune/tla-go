package evaluator

import (
	"fmt"

	"github.com/upamune/tla-go/value"
)

func unary(name string, v value.Value) (value.Value, error) {
	switch name {
	case "~":
		b, err := boolean(v)
		return value.Bool(!b), err
	case "-.":
		n, err := integer(v)
		if err != nil {
			return value.Value{}, err
		}
		return value.Integer(-n)
	case "SUBSET":
		return value.Subsets(v)
	case "UNION":
		return value.Union(v)
	case "DOMAIN":
		return domain(v)
	}
	return value.Value{}, fmt.Errorf("unsupported unary operator %s", name)
}

func domain(v value.Value) (value.Value, error) {
	return v.FunctionDomain()
}

func binary(name string, left, right value.Value) (value.Value, error) {
	switch name {
	case "=", "#":
		same, err := value.Equal(left, right)
		if name == "#" {
			same = !same
		}
		return value.Bool(same), err
	case "\\in", "\\notin":
		found, err := right.Contains(left)
		if name == "\\notin" {
			found = !found
		}
		return value.Bool(found), err
	case "\\subseteq":
		return subset(left, right)
	case "\\cup", "\\cap", "\\":
		return setOperation(name, left, right)
	case "<=>":
		return equivalence(left, right)
	}
	return arithmetic(name, left, right)
}

func integer(v value.Value) (int64, error) {
	n, ok := v.Int()
	if !ok {
		return 0, fmt.Errorf("expected an integer, found %s", v)
	}
	return int64(n), nil
}

func equivalence(left, right value.Value) (value.Value, error) {
	a, err := boolean(left)
	if err != nil {
		return value.Value{}, err
	}
	b, err := boolean(right)
	return value.Bool(a == b), err
}

func subset(left, right value.Value) (value.Value, error) {
	found, err := left.SubsetOf(right)
	return value.Bool(found), err
}

func setOperation(name string, left, right value.Value) (value.Value, error) {
	return value.SetOperation(name, left, right)
}
