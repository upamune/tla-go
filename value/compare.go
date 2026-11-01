package value

import (
	"fmt"
	"math"
)

// Equal compares values using TLC's type-sensitive equality. Set and interval
// representations compare by their mathematical members.
func Equal(a, b Value) (bool, error) {
	if c, ok := compareScalars(a, b); ok {
		return c == 0, nil
	}
	if isAny(a) || isAny(b) {
		return equalAny(a, b)
	}
	if isOverriddenComparison(a, b) {
		c, err := compareOverriddenSets(a, b, "check equality of")
		return c == 0, err
	}
	return equalRegular(a, b)
}

func equalRegular(a, b Value) (bool, error) {
	if err := incompatibleSetEquality(a, b); err != nil {
		return false, err
	}
	if err := incompatibleEquality(a, b); err != nil {
		return false, err
	}
	if a.kind == b.kind {
		switch a.kind {
		case FunctionSetKind:
			return equalFunctionSets(a, b)
		case SubsetKind:
			return Equal(a.data.domain, b.data.domain)
		case SetKind:
			return equalElements(a.data.elements, b.data.elements)
		case FunctionKind:
			return equalFunctions(a.data.pairs, b.data.pairs)
		}
	}
	return equalOther(a, b)
}

func equalOther(a, b Value) (bool, error) {
	if a.kind == LazyFunctionKind || (b.kind == LazyFunctionKind && a.kind != ModelKind) {
		left, err := a.materializedFunction()
		if err != nil {
			return false, err
		}
		right, err := b.materializedFunction()
		if err != nil {
			return false, err
		}
		return Equal(left, right)
	}
	if isEnumerableEqualitySet(a) && isEnumerableEqualitySet(b) {
		return equalEnumerableSets(a, b)
	}
	c, err := Compare(a, b)
	return c == 0, err
}

func isEnumerableEqualitySet(v Value) bool {
	return isSet(v) && v.kind != InfiniteSetKind && v.kind != SequenceSetKind
}

func incompatibleSetEquality(a, b Value) error {
	if (a.kind == SetKind || a.kind == IntervalKind) && !isSet(b) && b.kind != ModelKind {
		set, err := setForComparison(a)
		if err != nil {
			return err
		}
		return &UnexpectedError{Message: fmt.Sprintf("Attempted to check equality of the set %s with the value:\n%s", set, b)}
	}
	return nil
}

func incompatibleEquality(a, b Value) error {
	if b.kind == ModelKind {
		return nil
	}
	if a.kind == FunctionKind && b.kind != FunctionKind && b.kind != LazyFunctionKind {
		return incompatibleFunctionEquality(a, b)
	}
	return incompatibleScalarEquality(a, b)
}

func incompatibleScalarEquality(a, b Value) error {
	if a.kind == IntegerKind && b.kind != IntegerKind {
		return &UnexpectedError{Message: fmt.Sprintf("Attempted to check equality of integer %s with non-integer:\n%s", a, b)}
	}
	if a.kind == BooleanKind && b.kind != BooleanKind {
		return &UnexpectedError{Message: fmt.Sprintf("Attempted to compare equality of boolean %s with non-boolean:\n%s", a, b)}
	}
	if a.kind == StringKind && b.kind != StringKind {
		return &UnexpectedError{Message: fmt.Sprintf("Attempted to check equality of string %s with non-string:\n%s", a, b)}
	}
	return nil
}

func incompatibleFunctionEquality(a, b Value) error {
	if a.IsRecord() {
		return &UnexpectedError{Message: fmt.Sprintf("Attempted to check equality of record:\n%s\nwith non-record\n%s", a, b)}
	}
	if a.IsTuple() && len(a.data.pairs) == 0 {
		return &UnexpectedError{Message: fmt.Sprintf("Attempted to check equality of the function %s with the value:\n%s", emptyFunctionRecordText, b)}
	}
	return &UnexpectedError{Message: fmt.Sprintf("Attempted to check equality of the function %s with the value:\n%s", a, b)}
}

// TLC uses equals, rather than compareTo, for corresponding collection values.
// Sorting and membership continue to use Compare.
func equalElements(a, b []Value) (bool, error) {
	if len(a) != len(b) {
		return false, nil
	}
	for i := range a {
		equal, err := Equal(a[i], b[i])
		if err != nil || !equal {
			return false, err
		}
	}
	return true, nil
}

func equalFunctions(a, b []Pair) (bool, error) {
	if recordMismatch(a, b) {
		return false, fmt.Errorf("cannot compare a record with a non-record function")
	}
	if len(a) != len(b) {
		return false, nil
	}
	for i := range a {
		equal, err := Equal(a[i].Key, b[i].Key)
		if err != nil || !equal {
			return false, err
		}
		equal, err = Equal(a[i].Value, b[i].Value)
		if err != nil || !equal {
			return false, err
		}
	}
	return true, nil
}

func equalEnumerableSets(a, b Value) (bool, error) {
	if isLazySet(a) {
		set, err := a.finiteSet()
		if err != nil {
			return false, err
		}
		return Equal(set, b)
	}
	if isLazySet(b) {
		return equalRightLazySet(a, b)
	}
	if a.kind == IntervalKind && b.kind == IntervalKind {
		return equalIntervals(a, b)
	}
	return equalIntervalSet(a, b)
}

func equalRightLazySet(a, b Value) (bool, error) {
	if a.kind == IntervalKind {
		if _, err := equalityIntervalSize(a); err != nil {
			return false, err
		}
	}
	set, err := b.finiteSet()
	if err != nil {
		return false, err
	}
	return Equal(a, set)
}

func equalIntervals(a, b Value) (bool, error) {
	size, err := equalityIntervalSize(a)
	if err != nil {
		return false, err
	}
	if size == 0 {
		other, err := equalityIntervalSize(b)
		return other == 0 && err == nil, err
	}
	return a.number == b.number && a.high == b.high, nil
}

func equalityIntervalSize(interval Value) (int64, error) {
	size := max(int64(interval.high)-int64(interval.number)+1, 0)
	if size > math.MaxInt32 {
		return 0, &UnexpectedError{Message: fmt.Sprintf("Size of interval value exceeds the maximum representable size (32bits): %s.", interval)}
	}
	return size, nil
}

func equalIntervalSet(a, b Value) (bool, error) {
	interval, set := a, b
	intervalFirst := true
	if b.kind == IntervalKind {
		interval, set, intervalFirst = b, a, false
	}
	count, err := equalityIntervalSize(interval)
	if err != nil {
		return false, err
	}
	if count != int64(len(set.data.elements)) {
		return false, nil
	}
	for i, element := range set.data.elements {
		n := Value{kind: IntegerKind, number: int32(int64(interval.number) + int64(i))}
		equal, err := equalIntervalElement(n, element, intervalFirst)
		if err != nil || !equal {
			return false, err
		}
	}
	return true, nil
}

func equalIntervalElement(n, element Value, intervalFirst bool) (bool, error) {
	if intervalFirst {
		return Equal(n, element)
	}
	return Equal(element, n)
}

// compareScalars orders two integers, booleans or strings, which no
// comparison override applies to.
func compareScalars(a, b Value) (int, bool) {
	if a.kind != b.kind {
		return 0, false
	}
	switch a.kind {
	case IntegerKind, BooleanKind:
		return compareInt(a.number, b.number), true
	case StringKind:
		return compareString(a.data.text, b.data.text), true
	}
	return 0, false
}

func Compare(a, b Value) (int, error) {
	if c, ok := compareScalars(a, b); ok {
		return c, nil
	}
	if isAny(a) || isAny(b) {
		return compareAnyValues(a, b)
	}
	if isOverriddenComparison(a, b) {
		return compareOverriddenSets(a, b, "compare")
	}
	return compareRegular(a, b)
}

func compareRegular(a, b Value) (int, error) {
	if isSet(a) && isSet(b) {
		return compareSets(a, b)
	}
	if a.kind == ModelKind || b.kind == ModelKind {
		return compareModel(a, b)
	}
	if a.kind == LazyFunctionKind || b.kind == LazyFunctionKind {
		return compareLazyFunctions(a, b)
	}
	return compareOrdinary(a, b)
}

func compareOrdinary(a, b Value) (int, error) {
	if a.kind != b.kind || a.kind == Invalid {
		if a.kind == IntegerKind {
			return 0, &UnexpectedError{Message: fmt.Sprintf("Attempted to compare integer %s with non-integer:\n%s", a, b), TLCAssert: true}
		}
		return 0, fmt.Errorf("cannot compare different TLA+ value kinds")
	}
	return compareSameKind(a, b)
}

func compareSameKind(a, b Value) (int, error) {
	switch a.kind {
	case IntegerKind, BooleanKind:
		return compareInt(a.number, b.number), nil
	case StringKind:
		return compareString(a.data.text, b.data.text), nil
	case FunctionKind:
		return compareFunctions(a.data.pairs, b.data.pairs)
	default:
		return 0, fmt.Errorf("unsupported value comparison")
	}
}

func compareInt(a, b int32) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func compareString(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func isSet(v Value) bool {
	switch v.kind {
	case SetKind, IntervalKind, SequenceSetKind, InfiniteSetKind:
		return true
	default:
		return isLazySet(v)
	}
}

func modelType(v Value) string {
	if v.kind == ModelKind && len(v.data.text) > 2 && v.data.text[1] == '_' {
		return v.data.text[:1]
	}
	return ""
}

func compareModel(a, b Value) (int, error) {
	ta, tb := modelType(a), modelType(b)
	if modelTypesConflict(a, b, ta, tb) {
		return 0, fmt.Errorf("cannot compare incompatible typed model values")
	}
	if a.kind != ModelKind {
		return 1, nil
	}
	if b.kind != ModelKind {
		return -1, nil
	}
	return compareString(a.data.text, b.data.text), nil
}

func modelTypesConflict(a, b Value, ta, tb string) bool {
	return ta != "" && b.kind != ModelKind || tb != "" && a.kind != ModelKind || ta != "" && tb != "" && ta != tb
}

func compareSets(a, b Value) (int, error) {
	if a.kind == InfiniteSetKind || b.kind == InfiniteSetKind {
		return compareInfiniteSets(a, b)
	}
	if a.kind == SequenceSetKind || b.kind == SequenceSetKind {
		return compareSequenceSets(a, b)
	}
	return compareEnumerableSets(a, b)
}

func compareEnumerableSets(a, b Value) (int, error) {
	if isLazySet(a) {
		set, err := a.finiteSet()
		if err != nil {
			return 0, err
		}
		return compareSets(set, b)
	}
	if isLazySet(b) {
		set, err := b.finiteSet()
		if err != nil {
			return 0, err
		}
		return compareSets(a, set)
	}
	return compareFiniteSets(a, b)
}

func compareFiniteSets(a, b Value) (int, error) {
	if a.kind == IntervalKind && b.kind == IntervalKind {
		return compareIntervals(a, b), nil
	}
	if a.kind == IntervalKind {
		return compareIntervalSet(a, b)
	}
	if b.kind == IntervalKind {
		c, err := compareIntervalSet(b, a)
		return -c, err
	}
	return compareSlices(a.data.elements, b.data.elements)
}

func compareIntervalSet(interval, set Value) (int, error) {
	count := max(int64(interval.high)-int64(interval.number)+1, 0)
	if count < int64(len(set.data.elements)) {
		return -1, nil
	}
	if count > int64(len(set.data.elements)) {
		return 1, nil
	}
	for i, element := range set.data.elements {
		c, err := Compare(Value{kind: IntegerKind, number: int32(int64(interval.number) + int64(i))}, element)
		if err != nil || c != 0 {
			return c, err
		}
	}
	return 0, nil
}

func compareIntervals(a, b Value) int {
	na := int64(a.high) - int64(a.number) + 1
	nb := int64(b.high) - int64(b.number) + 1
	if na < 0 {
		na = 0
	}
	if nb < 0 {
		nb = 0
	}
	if na < nb {
		return -1
	}
	if na > nb {
		return 1
	}
	if na == 0 {
		return 0
	}
	return compareInt(a.number, b.number)
}

func compareSlices(a, b []Value) (int, error) {
	if len(a) < len(b) {
		return -1, nil
	}
	if len(a) > len(b) {
		return 1, nil
	}
	for i := range a {
		c, err := Compare(a[i], b[i])
		if err != nil || c != 0 {
			return c, err
		}
	}
	return 0, nil
}

func compareFunctions(a, b []Pair) (int, error) {
	if recordMismatch(a, b) {
		return 0, fmt.Errorf("cannot compare a record with a non-record function")
	}
	if len(a) < len(b) {
		return -1, nil
	}
	if len(a) > len(b) {
		return 1, nil
	}
	if c, err := comparePairKeys(a, b); err != nil || c != 0 {
		return c, err
	}
	return comparePairValues(a, b)
}

func comparePairKeys(a, b []Pair) (int, error) {
	for i := range a {
		c, err := Compare(a[i].Key, b[i].Key)
		if err != nil || c != 0 {
			return c, err
		}
	}
	return 0, nil
}

func comparePairValues(a, b []Pair) (int, error) {
	for i := range a {
		c, err := Compare(a[i].Value, b[i].Value)
		if err != nil || c != 0 {
			return c, err
		}
	}
	return 0, nil
}

func recordDomain(pairs []Pair) bool {
	for _, pair := range pairs {
		if pair.Key.kind != StringKind {
			return false
		}
	}
	return true
}

func recordMismatch(a, b []Pair) bool {
	return len(a) != len(b) && len(a) > 0 && len(b) > 0 && recordDomain(a) != recordDomain(b)
}

func compareSequenceSets(a, b Value) (int, error) {
	if a.kind != SequenceSetKind || b.kind != SequenceSetKind {
		return 0, fmt.Errorf("cannot compare a sequence set with a non-sequence set")
	}
	return Compare(a.data.domain, b.data.domain)
}
