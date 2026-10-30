package evaluator

import (
	"fmt"
	"math"

	"github.com/upamune/tla-go/value"
)

func (e *evaluator) subsetElements(count int, set value.Value, random *JavaRandom) ([]value.Value, error) {
	if set.Kind() == value.IntervalKind {
		return subsetInterval(count, set, random)
	}
	if set.Kind() == value.FunctionSetKind || set.Kind() == value.RecordSetKind || set.Kind() == value.ProductSetKind {
		return e.subsetProduct(count, set, random)
	}
	items, err := e.randomElements(set)
	if err != nil {
		return nil, err
	}
	indices := subsetIndices(count, int64(len(items)), random)
	result := make([]value.Value, len(indices))
	for i, index := range indices {
		result[i] = items[index]
	}
	return result, nil
}

func subsetInterval(count int, set value.Value, random *JavaRandom) ([]value.Value, error) {
	low, high, _ := set.Range()
	size := max(int64(high)-int64(low)+1, 0)
	if size > math.MaxInt32 {
		return nil, &RuntimeError{Message: fmt.Sprintf("Attempted to apply the operator overridden by the Java method\npublic static tlc2.value.impl.Value tlc2.module.Randomization.RandomSubset(tlc2.value.impl.Value,tlc2.value.impl.Value),\nbut it produced the following error:\nSize of interval value exceeds the maximum representable size (32bits): %s.", set), Standard: true}
	}
	indices := subsetIndices(count, size, random)
	result := make([]value.Value, len(indices))
	for i, index := range indices {
		v, err := value.Integer(int64(low) + index)
		if err != nil {
			return nil, err
		}
		result[i] = v
	}
	return result, nil
}
