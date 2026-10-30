package value

import "testing"

func TestCartesianSetsCanonicalAndShortCircuit(t *testing.T) {
	one, _ := Integer(1)
	two, _ := Integer(2)
	tuples := []Value{}
	for _, n := range []Value{one, two} {
		v, err := Tuple([]Value{n, Bool(true)})
		if err != nil {
			t.Fatal(err)
		}
		tuples = append(tuples, v)
	}
	bools, _ := Set([]Value{Bool(true)})
	product, err := ProductSet([]Value{Interval(1, 2), bools})
	if err != nil {
		t.Fatal(err)
	}
	explicit, err := Set(tuples)
	if err != nil {
		t.Fatal(err)
	}
	equal, err := Equal(product, explicit)
	if err != nil || !equal || key(t, product) != key(t, explicit) {
		t.Fatalf("product equality: %v, %v", equal, err)
	}
	count := 0
	err = product.Each(func(Value) (bool, error) { count++; return false, nil })
	if err != nil || count != 1 {
		t.Fatalf("short circuit: count %d, error %v", count, err)
	}
	member, err := product.Contains(tuples[0])
	if err != nil || !member {
		t.Fatalf("product membership: %v, %v", member, err)
	}
}

func TestRecordSetCanonicalAndMembership(t *testing.T) {
	one, _ := Integer(1)
	oneSet, _ := Set([]Value{one})
	trueSet, _ := Set([]Value{Bool(true)})
	fields := []Pair{{String("b"), trueSet}, {String("a"), oneSet}}
	records, err := RecordSet(fields)
	if err != nil {
		t.Fatal(err)
	}
	fields[0].Value = oneSet
	record, err := Function([]Pair{{String("a"), one}, {String("b"), Bool(true)}})
	if err != nil {
		t.Fatal(err)
	}
	found, err := records.Contains(record)
	if err != nil || !found {
		t.Fatalf("record membership: %v, %v", found, err)
	}
	explicit, _ := Set([]Value{record})
	equal, err := Equal(records, explicit)
	if err != nil || !equal || key(t, records) != key(t, explicit) {
		t.Fatalf("record equality: %v, %v", equal, err)
	}
}

func TestCartesianProductEmptyFactor(t *testing.T) {
	empty, _ := Set(nil)
	product, err := ProductSet([]Value{Interval(1, 2), empty})
	if err != nil {
		t.Fatal(err)
	}
	equal, err := Equal(product, empty)
	if err != nil || !equal {
		t.Fatalf("empty product: %v, %v", equal, err)
	}
}
