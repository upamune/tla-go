package value

import "fmt"

func isOverriddenSet(v Value) bool { return v.kind == InfiniteSetKind || v.kind == SequenceSetKind }

func isOverriddenComparison(a, b Value) bool {
	if !isSet(a) || !isSet(b) {
		return false
	}
	if a.kind == SequenceSetKind && b.kind == SequenceSetKind {
		return false
	}
	return isOverriddenSet(a) || isOverriddenSet(b)
}

func compareOverriddenSets(a, b Value, verb string) (int, error) {
	if !isOverriddenSet(a) {
		return overriddenRightError(a, b, verb)
	}
	if !isOverriddenSet(b) {
		return overriddenLeftError(a, b)
	}
	if a.kind == InfiniteSetKind && b.kind == InfiniteSetKind {
		if a.number == b.number {
			return 0, nil
		}
		if a.number != stringSet && b.number != stringSet {
			return compareInt(a.number, b.number), nil
		}
	}
	return 0, &TLCError{Message: fmt.Sprintf("Attempted to compare %s with the value\n%s", a, b)}
}

func overriddenLeftError(a, b Value) (int, error) {
	return 0, &TLCError{Message: fmt.Sprintf("Attempted to compare overridden value %s with non-overridden value:\n%s", a, b)}
}

func overriddenRightError(a, b Value, verb string) (int, error) {
	text, err := setForComparison(a)
	if err != nil {
		return 0, err
	}
	return 0, &TLCError{Message: fmt.Sprintf("Attempted to %s the set %s with the value:\n%s", verb, text, b)}
}

func setForComparison(v Value) (string, error) {
	if !v.IsEnumerable() {
		return v.StrictString()
	}
	items, err := v.Elements()
	if err != nil {
		return "", err
	}
	set, err := SetWithIterationOrder(items)
	if err != nil {
		return "", err
	}
	return set.StrictString()
}
