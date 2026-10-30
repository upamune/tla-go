package value

import (
	"reflect"
	"testing"
)

func TestSubsetsOrderAndEarlyStop(t *testing.T) {
	sets, err := Subsets(Interval(1, 3))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	err = sets.Each(func(v Value) (bool, error) {
		got = append(got, v.String())
		return len(got) < 5, nil
	})
	want := []string{"{}", "{1}", "{2}", "{3}", "{1, 2}"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, %v; want %v", got, err, want)
	}
}

func TestSubsetsLargeMembershipAndEnumerationLimit(t *testing.T) {
	sets, err := Subsets(Interval(1, 1000000))
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := Set([]Value{{kind: IntegerKind, number: 2}})
	if err != nil {
		t.Fatal(err)
	}
	found, err := sets.Contains(candidate)
	if err != nil || !found {
		t.Fatalf("membership: %v, %v", found, err)
	}
	if err = sets.Each(func(Value) (bool, error) { return true, nil }); err == nil {
		t.Fatal("enumeration above 30 elements should fail")
	}
}

func TestUnionCanonicalAndMembership(t *testing.T) {
	one, _ := Integer(1)
	two, _ := Integer(2)
	first, _ := Set([]Value{one, two})
	second, _ := Set([]Value{two})
	outer, _ := Set([]Value{first, second})
	union, err := Union(outer)
	if err != nil {
		t.Fatal(err)
	}
	found, err := union.Contains(one)
	if err != nil || !found {
		t.Fatalf("membership: %v, %v", found, err)
	}
	equal, err := Equal(union, first)
	if err != nil || !equal || key(t, union) != key(t, first) {
		t.Fatalf("union equality: %v, %v", equal, err)
	}
}

func TestSubsetOfPowerSets(t *testing.T) {
	power := func(base Value) Value {
		t.Helper()
		v, err := Subsets(base)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	for _, tc := range []struct {
		name        string
		left, right Value
		want        bool
	}{
		{"large-contained", power(Interval(1, 23)), power(Interval(1, 42)), true},
		{"large-not-contained", power(Interval(1, 42)), power(Interval(1, 23)), false},
		{"empty", power(Interval(1, 0)), power(Interval(1, 0)), true},
		{"finite-in-naturals", power(Interval(0, 5)), power(NaturalNumbers()), true},
		{"negative-not-in-naturals", power(Interval(-1, 5)), power(NaturalNumbers()), false},
		{"nested", power(power(Interval(1, 23))), power(power(Interval(1, 42))), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.left.SubsetOf(tc.right)
			if err != nil || got != tc.want {
				t.Fatalf("got %v, %v; want %v", got, err, tc.want)
			}
		})
	}
}
