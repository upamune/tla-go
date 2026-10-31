package value

import (
	"fmt"
	"strings"
)

// TLC expands small lazy sets in strict output and prints larger ones as a
// formula. Checking size first also avoids allocating exponential collections.
func (v Value) strictExpand() (bool, error) {
	switch v.kind {
	case SubsetKind:
		size, err := v.data.domain.Cardinality()
		return size <= 6, err
	case FunctionSetKind:
		return v.strictFunctionSetExpand()
	case RecordSetKind, ProductSetKind:
		return v.strictProductExpand()
	default:
		return true, nil
	}
}

func (v Value) strictFunctionSetExpand() (bool, error) {
	if v.data.domain.enumerable() {
		empty, err := v.data.domain.isEmpty()
		if err != nil || empty {
			return empty, err
		}
	}
	rangeSize, err := v.data.codomain.Cardinality()
	if err != nil {
		return false, err
	}
	domainSize, err := v.data.domain.Cardinality()
	if err != nil {
		return false, err
	}
	return powerAtMost(rangeSize, domainSize, 1999), nil
}

func powerAtMost(base, exponent, limit int64) bool {
	if base == 0 || base == 1 || exponent == 0 {
		return true
	}
	result := int64(1)
	for range exponent {
		if result > limit/base {
			return false
		}
		result *= base
	}
	return true
}

func (v Value) strictProductExpand() (bool, error) {
	size := int64(1)
	for _, pair := range v.data.pairs {
		if size == 0 {
			return true, nil
		}
		factor, err := pair.Value.Cardinality()
		if err != nil {
			return false, err
		}
		if factor == 0 {
			size = 0
			continue
		}
		if size > 1999/factor {
			size = 2000
		} else {
			size *= factor
		}
	}
	return size <= 1999, nil
}

func (v Value) strictSymbolicSet(terse bool) (string, error) {
	switch v.kind {
	case SubsetKind:
		base, err := v.data.domain.render(terse)
		if err != nil {
			return "", err
		}
		if v.data.domain.kind == IntervalKind {
			base = "(" + base + ")"
		}
		return "SUBSET " + base, nil
	case FunctionSetKind:
		domain, err := v.data.domain.render(terse)
		if err != nil {
			return "", err
		}
		codomain, err := v.data.codomain.render(terse)
		if err != nil {
			return "", err
		}
		return "[" + domain + " -> " + codomain + "]", nil
	case RecordSetKind, ProductSetKind:
		return v.strictSymbolicProduct(terse)
	default:
		return "", fmt.Errorf("cannot render this set symbolically")
	}
}

func (v Value) strictSymbolicProduct(terse bool) (string, error) {
	parts := make([]string, len(v.data.pairs))
	for i, pair := range v.data.pairs {
		part, err := pair.Value.render(terse)
		if err != nil {
			return "", err
		}
		if v.kind == RecordSetKind {
			part = pair.Key.data.text + ": " + part
		}
		parts[i] = part
	}
	if v.kind == RecordSetKind {
		return "[" + strings.Join(parts, ", ") + "]", nil
	}
	return "(" + strings.Join(parts, " \\X ") + ")", nil
}
