package value

import (
	"errors"
	"fmt"
	"math"
)

// ErrCardinalityOverflow indicates a set size outside TLC's integer range.
var ErrCardinalityOverflow = errors.New("set cardinality exceeds the maximum 32-bit integer")

// CardinalityOverflowError identifies the function set whose size overflowed.
type CardinalityOverflowError struct{ Set Value }

func (e *CardinalityOverflowError) Error() string { return ErrCardinalityOverflow.Error() }
func (e *CardinalityOverflowError) Unwrap() error { return ErrCardinalityOverflow }

// Cardinality computes the size of supported finite sets without enumerating
// products, powersets or function sets. Results must fit TLC's 32-bit integers.
func (v Value) Cardinality() (int64, error) {
	switch v.kind {
	case SetKind:
		return int64(len(v.data.elements)), nil
	case InfiniteSetKind:
		return v.infiniteCardinality()
	case IntervalKind:
		return checkedSize(max(int64(v.high)-int64(v.number)+1, 0))
	case SubsetKind:
		return v.subsetCardinality()
	case FunctionSetKind:
		return v.functionSetCardinality()
	case RecordSetKind, ProductSetKind:
		return v.productCardinality()
	default:
		return v.otherCardinality()
	}
}

func (v Value) otherCardinality() (int64, error) {
	switch v.kind {
	case UnionKind, SetOperationKind, FilteredSetKind:
		set, err := v.finiteSet()
		if err != nil {
			return 0, err
		}
		return set.Cardinality()
	default:
		return 0, fmt.Errorf("cardinality requires a finite set")
	}
}

func (v Value) infiniteCardinality() (int64, error) {
	if isAny(v) {
		return 0, &TLCError{Message: "Attempted to compute cardinality of the value\nANY"}
	}
	return 0, fmt.Errorf("cardinality requires a finite set")
}

func checkedSize(n int64) (int64, error) {
	if n > math.MaxInt32 {
		return 0, ErrCardinalityOverflow
	}
	return n, nil
}

func (v Value) subsetCardinality() (int64, error) {
	size, err := v.data.domain.Cardinality()
	if err != nil {
		return 0, err
	}
	if size >= 31 {
		return 0, fmt.Errorf("powerset cardinality exceeds the maximum 32-bit integer")
	}
	return int64(1) << uint(size), nil
}

func (v Value) functionSetCardinality() (int64, error) {
	empty, err := v.data.domain.isEmpty()
	if err != nil {
		return 0, err
	}
	if empty {
		return 1, nil
	}
	empty, err = v.data.codomain.isEmpty()
	if err != nil {
		return 0, err
	}
	if empty {
		return 0, nil
	}
	base, err := v.data.codomain.Cardinality()
	if err != nil || base == 1 {
		return base, err
	}
	exponent, err := v.data.domain.Cardinality()
	if err != nil {
		return 0, err
	}
	return v.functionPowerCardinality(base, exponent)
}

func (v Value) functionPowerCardinality(base, exponent int64) (int64, error) {
	size, err := checkedPower(base, exponent)
	if err != nil {
		return 0, &CardinalityOverflowError{Set: v}
	}
	return size, nil
}

func checkedPower(base, exponent int64) (int64, error) {
	result := int64(1)
	for range exponent {
		var err error
		result, err = checkedSize(result * base)
		if err != nil {
			return 0, err
		}
	}
	return result, nil
}

func (v Value) productCardinality() (int64, error) {
	empty, err := v.productIsEmpty()
	if err != nil || empty {
		return 0, err
	}
	result := int64(1)
	for _, pair := range v.data.pairs {
		size, err := pair.Value.Cardinality()
		if err != nil {
			return 0, err
		}
		result, err = checkedSize(result * size)
		if err != nil {
			return 0, err
		}
	}
	return result, nil
}

func (v Value) isEmpty() (bool, error) {
	switch v.kind {
	case SetKind:
		return len(v.data.elements) == 0, nil
	case IntervalKind:
		return v.high < v.number, nil
	case SubsetKind, SequenceSetKind, InfiniteSetKind:
		return false, nil
	case FunctionSetKind:
		return v.functionSetIsEmpty()
	case RecordSetKind, ProductSetKind:
		return v.productIsEmpty()
	default:
		return v.isEmptySpecial()
	}
}

func (v Value) isEmptySpecial() (bool, error) {
	if v.kind == UnionKind {
		return v.unionIsEmpty()
	}
	if v.kind != SetOperationKind && v.kind != FilteredSetKind {
		return false, fmt.Errorf("expected a set")
	}
	set, err := v.finiteSet()
	if err != nil {
		return false, err
	}
	return set.isEmpty()
}

func (v Value) functionSetIsEmpty() (bool, error) {
	empty, err := v.data.domain.isEmpty()
	if err != nil || empty {
		return false, err
	}
	return v.data.codomain.isEmpty()
}

func (v Value) productIsEmpty() (bool, error) {
	for _, pair := range v.data.pairs {
		empty, err := pair.Value.isEmpty()
		if err != nil || empty {
			return empty, err
		}
	}
	return false, nil
}

func (v Value) unionIsEmpty() (bool, error) {
	empty := true
	err := v.data.domain.Each(func(set Value) (bool, error) {
		var err error
		empty, err = set.isEmpty()
		return empty, err
	})
	return empty, err
}

// IsFinite reports finiteness for values currently represented by this package.
func (v Value) IsFinite() (bool, error) {
	switch v.kind {
	case FunctionKind, SetKind, IntervalKind:
		return true, nil
	case InfiniteSetKind:
		return false, nil
	case SubsetKind:
		return v.data.domain.IsFinite()
	case SequenceSetKind:
		return v.sequenceSetIsFinite()
	case FunctionSetKind:
		return v.functionSetIsFinite()
	case RecordSetKind, ProductSetKind:
		return v.productIsFinite()
	default:
		return v.isFiniteSpecial()
	}
}

func (v Value) isFiniteSpecial() (bool, error) {
	if v.kind == FilteredSetKind {
		return v.filteredIsFinite()
	}
	if v.kind == UnionKind {
		return v.unionIsFinite()
	}
	if v.kind == SetOperationKind {
		return v.operationIsFinite()
	}
	return false, fmt.Errorf("expected a set or finite function")
}

func (v Value) functionSetIsFinite() (bool, error) {
	domainFinite, err := v.data.domain.IsFinite()
	if err != nil {
		return false, err
	}
	if domainFinite {
		empty, err := v.data.domain.isEmpty()
		if err != nil || empty {
			return empty, err
		}
	}
	return v.functionRangeIsFinite(domainFinite)
}

func (v Value) functionRangeIsFinite(domainFinite bool) (bool, error) {
	rangeFinite, err := v.data.codomain.IsFinite()
	if err != nil || !rangeFinite {
		return rangeFinite, err
	}
	empty, err := v.data.codomain.isEmpty()
	if err != nil || empty {
		return empty, err
	}
	if domainFinite {
		return true, nil
	}
	size, err := v.data.codomain.Cardinality()
	return size == 1, err
}

func (v Value) productIsFinite() (bool, error) {
	allFinite := true
	for _, pair := range v.data.pairs {
		finite, err := pair.Value.IsFinite()
		if err != nil {
			return false, err
		}
		if !finite {
			allFinite = false
			continue
		}
		empty, err := pair.Value.isEmpty()
		if err != nil {
			return false, err
		}
		if empty {
			return true, nil
		}
	}
	return allFinite, nil
}

func (v Value) unionIsFinite() (bool, error) {
	finite := true
	err := v.data.domain.Each(func(set Value) (bool, error) {
		var err error
		finite, err = set.IsFinite()
		return finite, err
	})
	return finite, err
}
