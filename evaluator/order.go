package evaluator

import (
	"slices"

	"github.com/upamune/tla-go/value"
)

func (e *evaluator) orderedSet(elements []value.Value) (value.Value, error) {
	v, err := value.Set(elements)
	if err != nil || e.program.ordering == nil || !needsValueOrdering(elements) {
		return v, err
	}
	return v.OrderSet(e.program.ordering.Compare)
}

func needsValueOrdering(elements []value.Value) bool {
	for _, item := range elements {
		if item.IsSet() {
			return true
		}
		switch item.Kind() {
		case value.StringKind, value.ModelKind, value.FunctionKind, value.LazyFunctionKind:
			return true
		}
	}
	return false
}

func (e *evaluator) orderSetResult(op string, v value.Value) (value.Value, error) {
	if e.program.ordering == nil || v.Kind() != value.SetKind || (op != "\\cup" && op != "\\cap" && op != "\\") {
		return v, nil
	}
	return v.OrderSet(e.program.ordering.Compare)
}

func (e *evaluator) orderedRecord(pairs []value.Pair) (value.Value, error) {
	if e.program.ordering == nil {
		return value.Function(pairs)
	}
	var failure error
	slices.SortStableFunc(pairs, func(a, b value.Pair) int {
		if failure != nil {
			return 0
		}
		var c int
		c, failure = e.program.ordering.Compare(a.Key, b.Key)
		return c
	})
	if failure != nil {
		return value.Value{}, failure
	}
	names := make([]string, len(pairs))
	for i, pair := range pairs {
		names[i], _ = pair.Key.Text()
	}
	return value.FunctionWithFieldOrder(pairs, names)
}

func (e *evaluator) orderedPairs(v value.Value) ([]value.Pair, error) {
	pairs, err := v.Pairs()
	if err != nil || e.program.ordering == nil {
		return pairs, err
	}
	var failure error
	slices.SortStableFunc(pairs, func(a, b value.Pair) int {
		if failure != nil {
			return 0
		}
		var cmp int
		cmp, failure = e.program.ordering.Compare(a.Key, b.Key)
		return cmp
	})
	return pairs, failure
}
