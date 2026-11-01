package value

import (
	"math/rand"
	"slices"
	"testing"
)

// insertionReference is the ValueVec.sort(true) path, including its choice of representative.
func insertionReference(values []Value) ([]Value, error) {
	values = slices.Clone(values)
	n := min(len(values), 1)
	for i := 1; i < len(values); i++ {
		v := values[i]
		pos, duplicate, err := setInsertionPoint(values[:n], v)
		if err != nil {
			return nil, err
		}
		if !duplicate {
			copy(values[pos+1:n+1], values[pos:n])
			values[pos] = v
			n++
		}
	}
	return values[:n], nil
}

func TestScalarNormalizationMatchesInsertion(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	for _, kind := range []Kind{IntegerKind, BooleanKind, StringKind} {
		for size := 0; size <= 100; size++ {
			input := make([]Value, size)
			for i := range input {
				n := rng.Intn(11) - 5
				switch kind {
				case IntegerKind:
					input[i], _ = Integer(int64(n))
				case BooleanKind:
					input[i] = Bool(n > 0)
				case StringKind:
					input[i] = String(string(rune('a' + n + 5)))
				}
			}
			want, err := insertionReference(input)
			if err != nil {
				t.Fatal(err)
			}
			got, err := normalizeSetValues(slices.Clone(input))
			if err != nil || !slices.Equal(got, want) {
				t.Fatalf("kind %v size %d: got %v want %v, err %v", kind, size, got, want, err)
			}
		}
	}
}

// Fingerprints produced by Java TLC 1.7.4 SetEnumValue.fingerPrint(FP64.New())
// after FP64.Init(0). Strings were interned in the order "", "a", "z".
func TestScalarSetJavaFingerprints(t *testing.T) {
	integer := func(n int64) Value {
		v, err := Integer(n)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	first := String("z")
	cases := []struct {
		values []Value
		want   uint64
	}{
		{[]Value{integer(3), integer(-2), integer(3), integer(0)}, 0x19f046d4a5b6b47e},
		{[]Value{Bool(true), Bool(false), Bool(true)}, 0x0fb936ab6ac86fcd},
		{[]Value{first, String("a"), String("z"), String("")}, 0x69664fbe6206c7f3},
	}
	for _, tc := range cases {
		set, err := Set(tc.values)
		if err != nil {
			t.Fatal(err)
		}
		got, err := TLCFingerprint([]Value{set})
		if err != nil || got != tc.want {
			t.Fatalf("%v: got %016x want %016x err %v", tc.values, got, tc.want, err)
		}
		if tc.values[0].kind == StringKind && set.data.elements[2].data != first.data {
			t.Fatal("first string representative not retained")
		}
	}
}

func TestNormalizationFallbackComparisonOrder(t *testing.T) {
	one, _ := Integer(1)
	cases := [][]Value{{one, Bool(true), String("x")}, {Bool(true), one, String("x")}}
	for _, input := range cases {
		_, want := insertionReference(input)
		_, got := normalizeSetValues(slices.Clone(input))
		if want == nil || got == nil || want.Error() != got.Error() {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}
