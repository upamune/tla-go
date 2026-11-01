package value

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/upamune/tla-go/syntax/parser"
)

// ErrOutsideFunctionDomain indicates an attempted lookup outside the domain.
var ErrOutsideFunctionDomain = errors.New("argument is outside the function's domain")

// Set constructs a finite set, sorting and removing duplicates. Unlike a
// mixed set enumeration in TLC, incompatible elements cannot yet be sorted.
func Set(elements []Value) (Value, error) {
	sorted := append([]Value(nil), elements...)
	var err error
	sorted, err = normalizeSetValues(sorted)
	if err != nil {
		return Value{}, err
	}
	return Value{kind: SetKind, data: &payload{elements: sorted}}, nil
}

// SetUnnormalized preserves TLC's comparison order for a newly evaluated set.
func SetUnnormalized(elements []Value) (Value, error) {
	return SetUnnormalizedWithSource(elements, nil)
}

// SetUnnormalizedWithSource retains the source of a set enumeration.
func SetUnnormalizedWithSource(elements []Value, source *parser.Expression) (Value, error) {
	v, err := Set(elements)
	if err != nil {
		return Value{}, err
	}
	v.data.membership = &membershipOrder{items: append([]Value(nil), elements...)}
	v.data.source = source
	return v, nil
}

// SetWithIterationOrder keeps the supplied enumeration order while retaining
// canonical ordering for equality and state keys.
func SetWithIterationOrder(elements []Value) (Value, error) {
	v, err := Set(elements)
	if err != nil {
		return Value{}, err
	}
	seen := make(map[string]bool, len(elements))
	ordered := make([]Value, 0, len(v.data.elements))
	for _, element := range elements {
		key, err := element.Key()
		if err != nil {
			return Value{}, err
		}
		if !seen[key] {
			seen[key] = true
			ordered = append(ordered, element)
		}
	}
	if len(ordered) == len(v.data.elements) {
		v.data.iteration = ordered
	}
	return v, nil
}

// OrderSet returns a set with the same canonical identity and an enumeration
// order determined by compare. The receiver is never modified.
func (v Value) OrderSet(compare func(Value, Value) (int, error)) (Value, error) {
	if v.kind != SetKind {
		return Value{}, fmt.Errorf("expected an enumerable set")
	}
	if len(v.data.elements) < 2 {
		return v, nil
	}
	ordered := append([]Value(nil), v.data.elements...)
	var failure error
	slices.SortStableFunc(ordered, func(a, b Value) int {
		if failure != nil {
			return 0
		}
		var c int
		c, failure = compare(a, b)
		return c
	})
	if failure != nil {
		return Value{}, failure
	}
	if slices.Equal(ordered, v.data.elements) {
		return v, nil
	}
	data := *v.data
	data.iteration = ordered
	v.data = &data
	return v, nil
}

func (v Value) setIteration() []Value {
	if v.data.iteration != nil {
		return v.data.iteration
	}
	return v.data.elements
}

func sortValues(values []Value) error {
	var failure error
	slices.SortStableFunc(values, func(a, b Value) int {
		if failure != nil {
			return 0
		}
		comparison, err := Compare(a, b)
		failure = err
		return comparison
	})
	return failure
}

// TLC ValueVec.sort(true) inserts each value using compareTo and discards
// a comparison-equal value. Its binary-search order matters when comparison
// is not transitive or has observable evaluation effects.
func normalizeSetValues(values []Value) ([]Value, error) {
	if len(values) > 1 && homogeneousScalars(values) {
		return normalizeScalarSet(values), nil
	}
	n := min(len(values), 1)
	for i := 1; i < len(values); i++ {
		v := values[i]
		lo, duplicate, err := setInsertionPoint(values[:n], v)
		if err != nil {
			return nil, err
		}
		if !duplicate {
			copy(values[lo+1:n+1], values[lo:n])
			values[lo] = v
			n++
		}
	}
	return values[:n], nil
}

func homogeneousScalars(values []Value) bool {
	kind := values[0].kind
	if kind != IntegerKind && kind != BooleanKind && kind != StringKind {
		return false
	}
	for _, v := range values[1:] {
		if v.kind != kind {
			return false
		}
	}
	return true
}

// Scalar comparisons are total and have no effects. Retain the first equal
// string's payload, as ValueVec's insertion algorithm does.
func normalizeScalarSet(values []Value) []Value {
	if values[0].kind == StringKind {
		type indexedValue struct {
			value Value
			index int
		}
		indexed := make([]indexedValue, len(values))
		for i, v := range values {
			indexed[i] = indexedValue{v, i}
		}
		slices.SortFunc(indexed, func(a, b indexedValue) int {
			if c := compareString(a.value.data.text, b.value.data.text); c != 0 {
				return c
			}
			return compareIntIndex(a.index, b.index)
		})
		for i, item := range indexed {
			values[i] = item.value
		}
	} else {
		slices.SortFunc(values, func(a, b Value) int { return compareInt(a.number, b.number) })
	}
	n := 1
	for _, v := range values[1:] {
		c, _ := compareScalars(values[n-1], v)
		if c != 0 {
			values[n] = v
			n++
		}
	}
	return values[:n]
}

func compareIntIndex(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func setInsertionPoint(values []Value, v Value) (int, bool, error) {
	lo, hi := 0, len(values)
	for lo < hi {
		mid := (lo + hi) / 2
		c, err := Compare(v, values[mid])
		if err != nil {
			return 0, false, err
		}
		if c == 0 {
			return mid, true, nil
		}
		if c < 0 {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return lo, false, nil
}

// Function constructs a finite function. For duplicate keys, the first pair
// is retained, matching tla-ts's createFunction.
func Function(pairs []Pair) (Value, error) {
	return functionFromOwnedPairs(append([]Pair(nil), pairs...))
}

// functionFromOwnedPairs sorts a slice that is not shared with a caller.
func functionFromOwnedPairs(sorted []Pair) (Value, error) {
	var failure error
	slices.SortStableFunc(sorted, func(a, b Pair) int {
		if failure != nil {
			return 0
		}
		comparison, err := Compare(a.Key, b.Key)
		failure = err
		return comparison
	})
	if failure != nil {
		return Value{}, failure
	}
	return Value{kind: FunctionKind, data: &payload{pairs: uniquePairs(sorted)}}, nil
}

// MapFunctionPairs transforms a materialized function in key order. Lazy
// functions are not accepted. It returns the original value when every pair
// is unchanged; otherwise it constructs a new function with the same
// duplicate-key handling as Function. The callback receives each pair by
// value and cannot mutate the original function.
func (v Value) MapFunctionPairs(f func(Pair) (Pair, error)) (Value, error) {
	if v.kind != FunctionKind {
		return Value{}, fmt.Errorf("expected a function")
	}
	var mapped []Pair
	for i, pair := range v.data.pairs {
		result, err := f(pair)
		if err != nil {
			return Value{}, err
		}
		if mapped == nil && result != pair {
			mapped = make([]Pair, len(v.data.pairs))
			copy(mapped, v.data.pairs[:i])
		}
		if mapped != nil {
			mapped[i] = result
		}
	}
	if mapped == nil {
		return v, nil
	}
	return functionFromOwnedPairs(mapped)
}

// Record constructs a record while retaining its presentation provenance.
func Record(pairs []Pair) (Value, error) {
	return RecordWithSource(pairs, nil)
}

// RecordWithSource retains the record expression for fingerprint diagnostics.
func RecordWithSource(pairs []Pair, source *parser.Expression) (Value, error) {
	v, err := Function(pairs)
	if err != nil {
		return Value{}, err
	}
	if !recordDomain(v.data.pairs) {
		return Value{}, fmt.Errorf("record requires string field names")
	}
	v.number = 2
	v.data.source = source
	return v, nil
}

// FunctionWithFieldOrder retains record field order for presentation without
// changing the function's canonical key ordering or equality.
func FunctionWithFieldOrder(pairs []Pair, order []string) (Value, error) {
	return FunctionWithFieldOrderAndSource(pairs, order, nil)
}

// FunctionWithFieldOrderAndSource also retains the record expression.
func FunctionWithFieldOrderAndSource(pairs []Pair, order []string, source *parser.Expression) (Value, error) {
	v, err := Function(pairs)
	if err != nil {
		return Value{}, err
	}
	if len(order) == len(v.data.pairs) && recordDomain(v.data.pairs) {
		v.data.displayOrder = append([]string(nil), order...)
		v.number = 2
	}
	v.data.source = source
	return v, nil
}

// FunctionWithDomain retains the domain representation for TLC-compatible output.
// The domain must contain exactly the keys in pairs.
func FunctionWithDomain(pairs []Pair, domain Value) (Value, error) {
	v, err := Function(pairs)
	if err != nil {
		return Value{}, err
	}
	v.data.domain = domain
	return v, nil
}

func uniquePairs(pairs []Pair) []Pair {
	if len(pairs) == 0 {
		return pairs
	}
	n := 1
	for _, pair := range pairs[1:] {
		eq, _ := Equal(pairs[n-1].Key, pair.Key)
		if !eq {
			pairs[n] = pair
			n++
		}
	}
	return pairs[:n]
}

// Tuple builds the function 1..n -> elements, whose keys are already sorted.
func Tuple(elements []Value) (Value, error) {
	return TupleWithSource(elements, nil)
}

// TupleWithSource retains the tuple expression for fingerprint diagnostics.
func TupleWithSource(elements []Value, source *parser.Expression) (Value, error) {
	if _, err := Integer(min(int64(len(elements)), math.MaxInt32+1)); err != nil {
		return Value{}, err
	}
	pairs := make([]Pair, len(elements))
	for i, element := range elements {
		pairs[i] = Pair{Value{kind: IntegerKind, number: int32(i + 1)}, element}
	}
	return Value{kind: FunctionKind, number: 1, data: &payload{pairs: pairs, domain: Interval(1, int32(len(elements))), source: source}}, nil
}

func (v Value) Lookup(key Value) (Value, error) {
	return v.LookupWithLazyResult(key, false)
}

// LookupWithLazyResult preserves nested function values when requested.
func (v Value) LookupWithLazyResult(key Value, keepLazy bool) (Value, error) {
	if v.kind == LazyFunctionKind {
		return v.lazyLookup(key, keepLazy)
	}
	if v.kind != FunctionKind {
		return Value{}, fmt.Errorf("expected a function")
	}
	if i, ok := v.intervalIndex(key); ok {
		if i < 0 {
			return Value{}, ErrOutsideFunctionDomain
		}
		return v.data.pairs[i].Value, nil
	}
	for _, pair := range v.data.pairs {
		equal, err := Equal(pair.Key, key)
		if err != nil {
			return Value{}, err
		}
		if equal {
			return pair.Value, nil
		}
	}
	return Value{}, ErrOutsideFunctionDomain
}

func (v Value) Contains(element Value) (bool, error) {
	switch v.kind {
	case FilteredSetKind:
		return v.containsFiltered(element)
	case SetOperationKind:
		return v.containsOperation(element)
	case InfiniteSetKind:
		return v.containsInfinite(element)
	case SequenceSetKind:
		return v.containsSequence(element)
	case SubsetKind:
		found, err := v.containsSubset(element)
		return found, v.withFingerprintSource(err)
	case UnionKind:
		return v.containsSetOperator(element)
	case RecordSetKind, ProductSetKind:
		return v.containsCartesian(element)
	default:
		return v.containsOther(element)
	}
}

func (v Value) containsOther(element Value) (bool, error) {
	if v.kind == FunctionSetKind {
		return v.containsFunction(element)
	}
	if v.kind == IntervalKind {
		return v.intervalContains(element)
	}
	return v.containsFinite(element)
}

func (v Value) containsFinite(element Value) (bool, error) {
	if v.kind != SetKind {
		return false, fmt.Errorf("expected a set")
	}
	if v.data.membership != nil {
		return containsUnnormalized(v.data.membership.items, element)
	}
	low, high := 0, len(v.data.elements)
	for low < high {
		mid := (low + high) / 2
		comparison, err := Compare(element, v.data.elements[mid])
		if err != nil {
			return false, err
		}
		if comparison == 0 {
			return true, nil
		}
		if comparison < 0 {
			high = mid
		} else {
			low = mid + 1
		}
	}
	return false, nil
}

func containsUnnormalized(items []Value, element Value) (bool, error) {
	for _, item := range items {
		equal, err := Equal(item, element)
		if err != nil || equal {
			return equal, err
		}
	}
	return false, nil
}

func (v Value) intervalContains(element Value) (bool, error) {
	n, ok := element.Int()
	if !ok {
		if v.high < v.number || element.kind == ModelKind && modelType(element) == "" {
			return false, nil
		}
		return false, &UnexpectedError{Message: fmt.Sprintf("Attempted to check if the value:\n%s\nis in the integer interval %s", element, v)}
	}
	return n >= v.number && n <= v.high, nil
}

// Each streams elements of a finite set. Returning false stops iteration.
func (v Value) Each(visit func(Value) (bool, error)) error {
	switch v.kind {
	case FilteredSetKind:
		return v.eachFiltered(visit)
	case SetOperationKind:
		return v.eachOperation(visit)
	case SubsetKind:
		return v.eachSubset(visit)
	case UnionKind:
		return v.eachSetOperator(visit)
	case RecordSetKind, ProductSetKind:
		return v.eachCartesian(visit)
	case FunctionSetKind:
		return v.eachFunction(visit)
	case IntervalKind:
		return v.eachInterval(visit)
	default:
		return v.eachFinite(visit)
	}
}

func (v Value) eachFinite(visit func(Value) (bool, error)) error {
	if v.kind != SetKind {
		return fmt.Errorf("expected an enumerable set")
	}
	for _, element := range v.setIteration() {
		more, err := visit(element)
		if err != nil {
			return err
		}
		if !more {
			return nil
		}
	}
	return nil
}

func (v Value) eachInterval(visit func(Value) (bool, error)) error {
	for n := int64(v.number); n <= int64(v.high); n++ {
		more, err := visit(Value{kind: IntegerKind, number: int32(n)})
		if err != nil {
			return err
		}
		if !more {
			return nil
		}
	}
	return nil
}

// TupleElements returns sequence elements if this is a function over 1..n.
func (v Value) TupleElements() ([]Value, error) {
	pairs, err := v.Pairs()
	if err != nil || !tupleDomain(pairs) {
		return nil, fmt.Errorf("expected a tuple")
	}
	result := make([]Value, len(pairs))
	for i, pair := range pairs {
		result[i] = pair.Value
	}
	return result, nil
}

// Update replaces an existing function entry and leaves out-of-domain keys
// unchanged, as TLC's EXCEPT does. The callback is only called for present keys.
func (v Value) Update(key Value, update func(Value) (Value, error)) (Value, error) {
	if v.kind == LazyFunctionKind {
		return v.lazyUpdate(key, update)
	}
	updated, _, err := v.UpdateExisting(key, update)
	return updated, err
}

// UpdateExisting replaces the value at key of an enumerated function and
// reports whether key is in its domain; otherwise v is returned unchanged.
// Unlike Update, it calls update before returning and does not retain it.
func (v Value) UpdateExisting(key Value, update func(Value) (Value, error)) (Value, bool, error) {
	if v.kind != FunctionKind {
		return Value{}, false, fmt.Errorf("expected a function")
	}
	if i, ok := v.intervalIndex(key); ok {
		return v.updateAt(i, update)
	}
	for i, pair := range v.data.pairs {
		same, err := Equal(pair.Key, key)
		if err != nil {
			return Value{}, false, err
		}
		if same {
			return v.updateAt(i, update)
		}
	}
	return v, false, nil
}

// updateAt replaces the value of the pair at index i, or reports a key
// outside the domain when i is negative.
func (v Value) updateAt(i int, update func(Value) (Value, error)) (Value, bool, error) {
	if i < 0 {
		return v, false, nil
	}
	replacement, err := update(v.data.pairs[i].Value)
	if err != nil {
		return Value{}, true, err
	}
	pairs := append([]Pair(nil), v.data.pairs...)
	pairs[i].Value = replacement
	return Value{kind: FunctionKind, number: v.number, data: &payload{pairs: pairs, domain: v.data.domain, displayOrder: v.data.displayOrder}}, true, nil
}

// intervalIndex locates an integer argument of a function whose retained
// domain is an interval, so its sorted keys are exactly that interval. It
// returns -1 for an integer outside the domain, as a scan would find no key.
func (v Value) intervalIndex(key Value) (int, bool) {
	if key.kind != IntegerKind || v.data.domain.kind != IntervalKind {
		return 0, false
	}
	index := int64(key.number) - int64(v.data.domain.number)
	if index < 0 || index >= int64(len(v.data.pairs)) {
		return -1, true
	}
	return int(index), true
}
