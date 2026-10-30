package value

import "testing"

func TestFunctionSetMatchesEnumeration(t *testing.T) {
	domain := Interval(1, 2)
	codomain := Interval(0, 1)
	functions, err := FunctionSet(domain, codomain)
	if err != nil {
		t.Fatal(err)
	}
	zero, _ := Integer(0)
	one, _ := Integer(1)
	var members []Value
	for _, a := range []Value{zero, one} {
		for _, b := range []Value{zero, one} {
			member, err := Tuple([]Value{a, b})
			if err != nil {
				t.Fatal(err)
			}
			present, err := functions.Contains(member)
			if err != nil || !present {
				t.Fatalf("membership: %v, %v", present, err)
			}
			members = append(members, member)
		}
	}
	explicit, err := Set(members)
	if err != nil {
		t.Fatal(err)
	}
	equal, err := Equal(functions, explicit)
	if err != nil || !equal {
		t.Fatalf("equality: %v, %v", equal, err)
	}
	if key(t, functions) != key(t, explicit) {
		t.Fatal("equal sets have different keys")
	}
	count := 0
	err = functions.Each(func(Value) (bool, error) { count++; return false, nil })
	if err != nil || count != 1 {
		t.Fatalf("short circuit: count %d, error %v", count, err)
	}
}

func TestFunctionSetEmptyDomainAndRange(t *testing.T) {
	empty, err := Set(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		domain Value
		count  int
	}{{empty, 1}, {Interval(1, 1), 0}} {
		functions, err := FunctionSet(tc.domain, empty)
		if err != nil {
			t.Fatal(err)
		}
		elements, err := functions.Elements()
		if err != nil || len(elements) != tc.count {
			t.Fatalf("got %d elements, %v; want %d", len(elements), err, tc.count)
		}
	}
}

func TestFunctionSetMembershipChecksDomainAndRange(t *testing.T) {
	functions, err := FunctionSet(Interval(1, 2), Interval(0, 1))
	if err != nil {
		t.Fatal(err)
	}
	one, _ := Integer(1)
	two, _ := Integer(2)
	for _, elements := range [][]Value{{one}, {one, two}} {
		member, err := Tuple(elements)
		if err != nil {
			t.Fatal(err)
		}
		found, err := functions.Contains(member)
		if err != nil || found {
			t.Fatalf("%s: got %v, %v", member, found, err)
		}
	}
}

func TestKeyRejectsInvalidValue(t *testing.T) {
	if _, err := (Value{}).Key(); err == nil {
		t.Fatal("invalid value received a state key")
	}
}
