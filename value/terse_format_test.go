package value

import "testing"

// Java TLC's -terse prints lazy set values as formulas at any depth; the
// expected strings were checked against Java TLC 1.7.4.
func TestTerseString(t *testing.T) {
	one, two := mustInteger(t, 1), mustInteger(t, 2)
	base := mustSet(t, one, two)
	subset, err := Subsets(base)
	if err != nil {
		t.Fatal(err)
	}
	functions, err := FunctionSet(base, base)
	if err != nil {
		t.Fatal(err)
	}
	records, err := RecordSet([]Pair{{Key: String("a"), Value: base}})
	if err != nil {
		t.Fatal(err)
	}
	product, err := ProductSet([]Value{base, mustSet(t, mustInteger(t, 3))})
	if err != nil {
		t.Fatal(err)
	}
	tuple, err := Tuple([]Value{subset, functions, records, product, base})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		value        Value
		terse, plain string
	}{
		{subset, "SUBSET {1, 2}", "{{}, {1}, {2}, {1, 2}}"},
		{tuple, "<<SUBSET {1, 2}, [{1, 2} -> {1, 2}], [a: {1, 2}], ({1, 2} \\X {3}), {1, 2}>>", ""},
	} {
		got, err := tc.value.TerseString()
		if err != nil || got != tc.terse {
			t.Errorf("TerseString() = %q, %v; want %q", got, err, tc.terse)
		}
		if tc.plain == "" {
			continue
		}
		if got, err := tc.value.StrictString(); err != nil || got != tc.plain {
			t.Errorf("StrictString() = %q, %v; want %q", got, err, tc.plain)
		}
	}
}

func mustInteger(t *testing.T, n int64) Value {
	t.Helper()
	v, err := Integer(n)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func mustSet(t *testing.T, elements ...Value) Value {
	t.Helper()
	v, err := Set(elements)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
