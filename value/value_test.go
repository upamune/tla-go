package value

import (
	"math"
	"testing"
)

func number(t *testing.T, n int64) Value {
	t.Helper()
	v, err := Integer(n)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func set(t *testing.T, items ...Value) Value {
	t.Helper()
	v, err := Set(items)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func function(t *testing.T, pairs ...Pair) Value {
	t.Helper()
	v, err := Function(pairs)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func key(t *testing.T, v Value) string {
	t.Helper()
	k, err := v.Key()
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestSetAndIntervalEquality(t *testing.T) {
	one, two, three := number(t, 1), number(t, 2), number(t, 3)
	cases := [][2]Value{{set(t, three, one, two, one), Interval(1, 3)}, {set(t), Interval(3, 2)}, {set(t, set(t, two, one)), set(t, Interval(1, 2))}}
	for _, pair := range cases {
		equal, err := Equal(pair[0], pair[1])
		if err != nil || !equal {
			t.Fatalf("%s != %s: %v", pair[0], pair[1], err)
		}
		if key(t, pair[0]) != key(t, pair[1]) {
			t.Fatalf("equal values have different keys: %s and %s", pair[0], pair[1])
		}
	}
	if got := cases[0][0].String(); got != "{1, 2, 3}" {
		t.Fatalf("got %s", got)
	}
}

func TestTupleFunctionAndRecord(t *testing.T) {
	one, two := number(t, 1), number(t, 2)
	tuple, err := Tuple([]Value{one, String("x")})
	if err != nil {
		t.Fatal(err)
	}
	fn := function(t, Pair{two, String("x")}, Pair{one, one}, Pair{one, two})
	equal, err := Equal(tuple, fn)
	if err != nil || !equal || key(t, tuple) != key(t, fn) {
		t.Fatalf("tuple %s and function %s: %v", tuple, fn, err)
	}
	if tuple.String() != `<<1, "x">>` {
		t.Fatalf("got %s", tuple)
	}
	v, err := fn.Lookup(two)
	if err != nil || v.String() != `"x"` {
		t.Fatalf("lookup: %s %v", v, err)
	}
	record := function(t, Pair{String("b"), two}, Pair{String("a"), one})
	if record.String() != "[a |-> 1, b |-> 2]" {
		t.Fatalf("got %s", record)
	}
	if _, err = Equal(record, tuple); err == nil {
		t.Fatal("record compared with non-record")
	}
}

func TestRecordProvenancePreservesIdentity(t *testing.T) {
	if _, err := Record([]Pair{{Key: number(t, 1), Value: Bool(true)}}); err == nil {
		t.Fatal("accepted a non-string record field")
	}
	pairs := []Pair{{Key: String("a"), Value: number(t, 1)}}
	record, err := Record(pairs)
	if err != nil {
		t.Fatal(err)
	}
	function, err := Function(pairs)
	if err != nil {
		t.Fatal(err)
	}
	equal, err := Equal(record, function)
	if err != nil || !equal || !record.IsRecord() || function.IsRecord() || key(t, record) != key(t, function) {
		t.Fatalf("record %v, function %v, equal %t, error %v", record.IsRecord(), function.IsRecord(), equal, err)
	}
	recordFP, err := TLCFingerprint([]Value{record})
	if err != nil {
		t.Fatal(err)
	}
	functionFP, err := TLCFingerprint([]Value{function})
	if err != nil || recordFP != functionFP {
		t.Fatalf("fingerprints %d and %d, error %v", recordFP, functionFP, err)
	}
	updated, err := record.Update(String("a"), func(Value) (Value, error) { return number(t, 2), nil })
	if err != nil || !updated.IsRecord() || record.String() != "[a |-> 1]" || updated.String() != "[a |-> 2]" {
		t.Fatalf("updated record %s, original %s, error %v", updated, record, err)
	}
	tuple, err := Tuple([]Value{number(t, 1)})
	if err != nil {
		t.Fatal(err)
	}
	updatedTuple, err := tuple.Update(number(t, 1), func(Value) (Value, error) { return number(t, 2), nil })
	if err != nil || !updatedTuple.IsTuple() || tuple.String() != "<<1>>" || updatedTuple.String() != "<<2>>" {
		t.Fatalf("updated tuple %s, original %s, error %v", updatedTuple, tuple, err)
	}
}

func TestBoundsTypesAndImmutability(t *testing.T) {
	if _, err := Integer(math.MaxInt32 + 1); err == nil {
		t.Fatal("overflow accepted")
	}
	if _, err := Integer(math.MinInt32 - 1); err == nil {
		t.Fatal("underflow accepted")
	}
	if _, err := Equal(number(t, 1), String("1")); err == nil {
		t.Fatal("different scalar types compared")
	}
	if _, err := Equal(Model("A_x"), Model("B_x")); err == nil {
		t.Fatal("incompatible typed values compared")
	}
	if member, err := Interval(3, 2).Contains(String("x")); err != nil || member {
		t.Fatalf("empty interval membership: %v %v", member, err)
	}
	items := []Value{number(t, 2), number(t, 1)}
	s := set(t, items...)
	items[0] = number(t, 9)
	copy, err := s.Elements()
	if err != nil {
		t.Fatal(err)
	}
	copy[0] = number(t, 9)
	if s.String() != "{1, 2}" {
		t.Fatalf("set mutated: %s", s)
	}
}

func TestKeyDistinguishesNestedValues(t *testing.T) {
	vals := []Value{String("1"), Model("1"), number(t, 1), Bool(true), set(t, number(t, 1)), set(t, number(t, 2)), function(t, Pair{number(t, 1), number(t, 1)})}
	seen := map[string]bool{}
	for _, v := range vals {
		key := key(t, v)
		if seen[key] {
			t.Fatalf("duplicate key for %s", v)
		}
		seen[key] = true
	}
}

func BenchmarkSetKey(b *testing.B) {
	items := make([]Value, 100)
	for i := range items {
		items[i], _ = Integer(int64(i))
	}
	v, _ := Set(items)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = v.Key()
	}
}

func TestFunctionWithFieldOrderPreservesEquality(t *testing.T) {
	one, err := Integer(1)
	if err != nil {
		t.Fatal(err)
	}
	two, err := Integer(2)
	if err != nil {
		t.Fatal(err)
	}
	pairs := []Pair{{Key: String("a"), Value: one}, {Key: String("b"), Value: two}}
	order := []string{"b", "a"}
	ordered, err := FunctionWithFieldOrder(pairs, order)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := Function(pairs)
	if err != nil {
		t.Fatal(err)
	}
	order[0] = "a"
	equal, err := Equal(ordered, canonical)
	if err != nil || !equal {
		t.Fatalf("equal: %v, %v", equal, err)
	}
	first, err := ordered.Key()
	if err != nil {
		t.Fatal(err)
	}
	second, err := canonical.Key()
	if err != nil || first != second {
		t.Fatalf("keys differ: %q, %q, %v", first, second, err)
	}
	if got := ordered.String(); got != "[b |-> 2, a |-> 1]" {
		t.Fatalf("got %s", got)
	}
	if got, err := ordered.StrictString(); err != nil || got != "[b |-> 2, a |-> 1]" {
		t.Fatalf("got %s, %v", got, err)
	}
}

func TestSetIterationOrderPreservesIdentity(t *testing.T) {
	a, b := Model("a"), Model("b")
	ordered, err := SetWithIterationOrder([]Value{b, a, b})
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := Set([]Value{a, b})
	if err != nil {
		t.Fatal(err)
	}
	equal, err := Equal(ordered, canonical)
	if err != nil || !equal {
		t.Fatalf("equal: %v, %v", equal, err)
	}
	first, err := ordered.Key()
	if err != nil {
		t.Fatal(err)
	}
	second, err := canonical.Key()
	if err != nil || first != second {
		t.Fatalf("keys %q, %q, %v", first, second, err)
	}
	elements, err := ordered.Elements()
	if err != nil || len(elements) != 2 || elements[0].String() != "b" || elements[1].String() != "a" {
		t.Fatalf("elements %v, %v", elements, err)
	}
	var iterated []string
	err = ordered.Each(func(v Value) (bool, error) { iterated = append(iterated, v.String()); return true, nil })
	if err != nil || len(iterated) != 2 || iterated[0] != "b" || iterated[1] != "a" {
		t.Fatalf("iteration %v, %v", iterated, err)
	}
	if got := ordered.String(); got != "{b, a}" {
		t.Fatalf("got %s", got)
	}
}
