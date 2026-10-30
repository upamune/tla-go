package value

import (
	"strings"
	"testing"
)

func TestSortedSetMembership(t *testing.T) {
	s := set(t, number(t, 8), number(t, 2), number(t, 5), number(t, 4))
	for n := int64(0); n <= 10; n++ {
		got, err := s.Contains(number(t, n))
		want := n == 2 || n == 4 || n == 5 || n == 8
		if err != nil || got != want {
			t.Fatalf("%d: got %t, %v; want %t", n, got, err, want)
		}
	}
	if got, err := set(t).Contains(String("any")); got || err != nil {
		t.Fatalf("empty set: got %t, %v", got, err)
	}
}

func TestSortedSetMembershipComparisonError(t *testing.T) {
	var tuples []Value
	for i := int64(1); i <= 4; i++ {
		tuple, err := Tuple([]Value{number(t, i)})
		if err != nil {
			t.Fatal(err)
		}
		tuples = append(tuples, tuple)
	}
	_, err := set(t, tuples...).Contains(number(t, 2))
	if err == nil || !strings.Contains(err.Error(), "Attempted to compare integer 2 with non-integer:\n<<3>>") {
		t.Fatalf("unexpected comparison error: %v", err)
	}
}

func TestDeepMembershipNormalizationKeepsOriginal(t *testing.T) {
	var tuples []Value
	for i := int64(1); i <= 4; i++ {
		tuple, err := Tuple([]Value{number(t, i)})
		if err != nil {
			t.Fatal(err)
		}
		tuples = append(tuples, tuple)
	}
	original, err := SetUnnormalized(tuples)
	if err != nil {
		t.Fatal(err)
	}
	record := function(t, Pair{String("item"), original})
	normalized := record.DeepNormalizedMembership()
	inner, err := normalized.Lookup(String("item"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = inner.Contains(number(t, 2))
	if err == nil || !strings.Contains(err.Error(), "compare integer 2 with non-integer:\n<<3>>") {
		t.Fatalf("normalized comparison: %v", err)
	}
	_, err = original.Contains(number(t, 2))
	if err == nil || !strings.Contains(err.Error(), "check equality of the function <<1>>") {
		t.Fatalf("original comparison: %v", err)
	}
}

func BenchmarkSortedSetMembership(b *testing.B) {
	items := make([]Value, 4096)
	for i := range items {
		items[i], _ = Integer(int64(i))
	}
	s, _ := Set(items)
	key, _ := Integer(4095)
	b.ResetTimer()
	for b.Loop() {
		_, _ = s.Contains(key)
	}
}
