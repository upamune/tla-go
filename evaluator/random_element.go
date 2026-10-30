package evaluator

import (
	"fmt"

	"github.com/upamune/tla-go/value"
)

func (e *evaluator) randomElement(set value.Value) (value.Value, error) {
	if !set.IsSet() || !set.IsEnumerable() {
		return value.Value{}, &RuntimeError{Message: fmt.Sprintf("RandomElement cannot enumerate the non-enumerable set %s", set), Standard: true}
	}
	random, err := e.randomGenerator()
	if err != nil {
		return value.Value{}, err
	}
	return e.drawRandomElement(set, random)
}

func (e *evaluator) randomGenerator() (*JavaRandom, error) {
	runtime, ok := e.program.tlcRuntime.(interface{ Random() (*JavaRandom, error) })
	if !ok {
		if e.program.defaultRandom != nil {
			return e.program.defaultRandom, nil
		}
		return nil, fmt.Errorf("random operators require a TLC runtime")
	}
	return runtime.Random()
}

func (e *evaluator) drawRandomElement(set value.Value, random *JavaRandom) (value.Value, error) {
	switch set.Kind() {
	case value.FunctionSetKind:
		return e.drawFunction(set, random)
	case value.RecordSetKind:
		return e.drawRecord(set, random)
	case value.ProductSetKind:
		return e.drawProduct(set, random)
	case value.IntervalKind:
		low, high, _ := set.Range()
		size := max(int64(high)-int64(low)+1, 0)
		if size == 0 {
			return value.Value{}, randomEmptyError()
		}
		return value.Integer(int64(low) + int64(random.Float64()*float64(size)))
	}
	items, err := e.randomElements(set)
	if err != nil {
		return value.Value{}, err
	}
	if len(items) == 0 {
		return value.Value{}, randomEmptyError()
	}
	return items[int(random.Float64()*float64(len(items)))], nil
}

func (e *evaluator) randomElements(set value.Value) ([]value.Value, error) {
	items, err := set.Elements()
	if err != nil {
		return nil, err
	}
	ordered, err := e.orderedSet(items)
	if err != nil {
		return nil, err
	}
	return ordered.Elements()
}

func randomEmptyError() error {
	return &RuntimeError{Message: "RandomElement of the empty set", Standard: true}
}
