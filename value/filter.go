package value

import "fmt"

// Filter returns a set whose predicate is evaluated only when needed. The
// caller must ensure the predicate captures immutable, concurrency-safe state.
func Filter(base Value, predicate func(Value) (bool, error)) (Value, error) {
	if !isSet(base) || predicate == nil {
		return Value{}, fmt.Errorf("set filter requires a set and predicate")
	}
	return Value{kind: FilteredSetKind, data: &payload{domain: base, test: predicate}}, nil
}

func (v Value) containsFiltered(element Value) (bool, error) {
	found, err := v.data.domain.Contains(element)
	if err != nil || !found {
		return found, err
	}
	return v.data.test(element)
}

func (v Value) eachFiltered(visit func(Value) (bool, error)) error {
	return v.data.domain.Each(func(item Value) (bool, error) {
		accepted, err := v.data.test(item)
		if err != nil {
			return false, err
		}
		if accepted {
			return visit(item)
		}
		return true, nil
	})
}

func (v Value) filteredIsFinite() (bool, error) {
	finite, err := v.data.domain.IsFinite()
	if err != nil {
		return false, err
	}
	if !finite {
		return false, fmt.Errorf("cannot determine finiteness of a filter over an infinite set")
	}
	return true, nil
}
