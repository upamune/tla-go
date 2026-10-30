package value

import "testing"

func TestInfiniteSetMembershipAndComparison(t *testing.T) {
	neg, _ := Integer(-1)
	pos, _ := Integer(1)
	for _, tc := range []struct {
		set, item Value
		want      bool
	}{
		{NaturalNumbers(), neg, false}, {NaturalNumbers(), pos, true}, {Integers(), neg, true}, {Strings(), String("a"), true}, {Strings(), Model("M"), false},
	} {
		got, err := tc.set.Contains(tc.item)
		if err != nil || got != tc.want {
			t.Fatalf("%s in %s: %v, %v", tc.item, tc.set, got, err)
		}
	}
	if _, err := Strings().Contains(pos); err == nil {
		t.Fatal("invalid STRING membership was accepted")
	}
	if _, err := Equal(Strings(), NaturalNumbers()); err == nil {
		t.Fatal("STRING and Nat comparison was accepted")
	}
}

func TestInfiniteSetComposites(t *testing.T) {
	empty, _ := Set(nil)
	one, _ := Integer(1)
	single, _ := Set([]Value{one})
	for _, tc := range []struct {
		domain, rangeSet Value
		finite           bool
		size             int64
	}{
		{NaturalNumbers(), empty, true, 0}, {empty, NaturalNumbers(), true, 1}, {NaturalNumbers(), single, true, 1}, {NaturalNumbers(), NaturalNumbers(), false, -1},
	} {
		functions, err := FunctionSet(tc.domain, tc.rangeSet)
		if err != nil {
			t.Fatal(err)
		}
		finite, err := functions.IsFinite()
		if err != nil || finite != tc.finite {
			t.Fatalf("finiteness: %v, %v", finite, err)
		}
		if tc.size >= 0 {
			size, err := functions.Cardinality()
			if err != nil || size != tc.size {
				t.Fatalf("cardinality: %v, %v", size, err)
			}
		}
	}
	product, _ := ProductSet([]Value{NaturalNumbers(), empty})
	if err := product.Each(func(Value) (bool, error) { t.Fatal("empty product produced a value"); return false, nil }); err != nil {
		t.Fatal(err)
	}
}

func TestSetOperationsOverInfiniteSets(t *testing.T) {
	one, _ := Integer(1)
	two, _ := Integer(2)
	finite, _ := Set([]Value{one, two})
	intersection, _ := SetOperation("\\cap", NaturalNumbers(), finite)
	got, err := intersection.Elements()
	if err != nil || len(got) != 2 {
		t.Fatalf("intersection: %v, %v", got, err)
	}
	outer, _ := Set([]Value{finite})
	union, _ := Union(outer)
	nested, _ := SetOperation("\\cap", union, NaturalNumbers())
	got, err = nested.Elements()
	if err != nil || len(got) != 2 {
		t.Fatalf("nested intersection: %v, %v", got, err)
	}
	unbounded, _ := SetOperation("\\cup", NaturalNumbers(), finite)
	if err := unbounded.Each(func(Value) (bool, error) { return true, nil }); err == nil {
		t.Fatal("unbounded set was enumerated")
	}
}
