package value

import "fmt"

const (
	naturalSet = iota
	integerSet
	stringSet
	anySet
)

func NaturalNumbers() Value { return Value{kind: InfiniteSetKind, number: naturalSet} }
func Integers() Value       { return Value{kind: InfiniteSetKind, number: integerSet} }
func Strings() Value        { return Value{kind: InfiniteSetKind, number: stringSet} }
func Any() Value            { return Value{kind: InfiniteSetKind, number: anySet} }

func (v Value) infiniteName() string {
	switch v.number {
	case naturalSet:
		return "Nat"
	case integerSet:
		return "Int"
	case stringSet:
		return "STRING"
	case anySet:
		return "ANY"
	default:
		return "<invalid>"
	}
}

func (v Value) containsInfinite(element Value) (bool, error) {
	if v.number == anySet {
		return true, nil
	}
	if element.kind == ModelKind {
		return false, nil
	}
	if v.number == stringSet {
		if element.kind == StringKind {
			return true, nil
		}
		return false, invalidInfiniteMembership(element, v.infiniteName())
	}
	n, ok := element.Int()
	if !ok {
		return false, invalidInfiniteMembership(element, v.infiniteName())
	}
	return v.number == integerSet || n >= 0, nil
}

func invalidInfiniteMembership(element Value, set string) error {
	return &TLCError{Message: fmt.Sprintf("Attempted to check if the value:\n%s\nis an element of %s.", element, set), Standard: true}
}

func compareInfiniteSets(a, b Value) (int, error) {
	if a.kind != InfiniteSetKind || b.kind != InfiniteSetKind {
		return 0, fmt.Errorf("cannot compare an infinite set with a different kind of set")
	}
	if a.number == b.number {
		return 0, nil
	}
	if a.number != stringSet && b.number != stringSet {
		return compareInt(a.number, b.number), nil
	}
	return 0, fmt.Errorf("cannot compare %s with %s", a.infiniteName(), b.infiniteName())
}

// Equal function sets can be compared structurally even when they cannot be enumerated.
func equalFunctionSets(a, b Value) (bool, error) {
	equal, err := Equal(a.data.domain, b.data.domain)
	if err != nil || !equal {
		return equal, err
	}
	return Equal(a.data.codomain, b.data.codomain)
}
