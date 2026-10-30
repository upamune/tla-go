package value

import "testing"

func TestCardinalityWithoutEnumeration(t *testing.T) {
	empty, _ := Set(nil)
	one, _ := Integer(1)
	single, _ := Set([]Value{one})
	large, err := ProductSet([]Value{Interval(1, 50000), Interval(1, 50000)})
	if err != nil {
		t.Fatal(err)
	}
	functionSet, err := FunctionSet(large, single)
	if err != nil {
		t.Fatal(err)
	}
	zeroProduct, _ := ProductSet([]Value{large, empty})
	emptyDomain, _ := FunctionSet(empty, large)
	tests := []struct {
		set  Value
		want int64
	}{{functionSet, 1}, {zeroProduct, 0}, {emptyDomain, 1}}
	for _, tc := range tests {
		got, err := tc.set.Cardinality()
		if err != nil || got != tc.want {
			t.Fatalf("got %d, %v; want %d", got, err, tc.want)
		}
	}
	if _, err := large.Cardinality(); err == nil {
		t.Fatal("cardinality overflow was not detected")
	}
}

func TestIsFiniteRejectsInvalidNestedUnion(t *testing.T) {
	one, _ := Integer(1)
	outer, _ := Set([]Value{one})
	invalidUnion, _ := Union(outer)
	sets, _ := Subsets(invalidUnion)
	if _, err := sets.IsFinite(); err == nil {
		t.Fatal("expected invalid union to be rejected")
	}
}
