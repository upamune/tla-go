package value

import (
	"fmt"
	"strings"
)

// SymbolicString renders a set used in the domain of a deferred function.
// Unlike StrictString, it can describe infinite set operations.
func (v Value) SymbolicString() (string, error) {
	switch v.kind {
	case SetOperationKind:
		return v.symbolicOperation()
	case ProductSetKind:
		return v.symbolicProduct()
	default:
		return v.StrictString()
	}
}

func (v Value) symbolicOperation() (string, error) {
	left, err := v.data.domain.SymbolicString()
	if err != nil {
		return "", err
	}
	right, err := v.data.codomain.SymbolicString()
	if err != nil {
		return "", err
	}
	return left + " " + v.data.text + " " + right, nil
}

func (v Value) symbolicProduct() (string, error) {
	parts := make([]string, len(v.data.pairs))
	for i, pair := range v.data.pairs {
		var err error
		parts[i], err = pair.Value.SymbolicString()
		if err != nil {
			return "", err
		}
	}
	return "(" + strings.Join(parts, " \\X ") + ")", nil
}

// ProductFactors returns the component sets of a Cartesian-product set.
func (v Value) ProductFactors() ([]Value, error) {
	if v.kind != ProductSetKind {
		return nil, fmt.Errorf("expected a Cartesian-product set")
	}
	factors := make([]Value, len(v.data.pairs))
	for i, pair := range v.data.pairs {
		factors[i] = pair.Value
	}
	return factors, nil
}
