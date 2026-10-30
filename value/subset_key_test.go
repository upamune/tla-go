package value

import "testing"

func TestSubsetKeyMatchesEnumeratedSet(t *testing.T) {
	ordered, err := SetWithIterationOrder([]Value{number(t, 3), number(t, 1), number(t, 2)})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		base Value
	}{
		{"empty", set(t)},
		{"interval", Interval(-2, 3)},
		{"noncontiguous integers", set(t, number(t, 0), number(t, 2), number(t, 4))},
		{"iteration order", ordered},
		{"strings", set(t, String("b"), String("a"), String("c"))},
		{"nested sets", set(t, set(t), set(t, number(t, 1)), set(t, number(t, 2), number(t, 3)))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			powerset, err := Subsets(tc.base)
			if err != nil {
				t.Fatal(err)
			}
			materialized, err := powerset.finiteSet()
			if err != nil {
				t.Fatal(err)
			}
			want, err := materialized.Key()
			if err != nil {
				t.Fatal(err)
			}
			got, err := powerset.Key()
			if err != nil || got != want {
				t.Fatalf("got %q, %v; want %q", got, err, want)
			}
		})
	}
}

func TestLargeSubsetUsesSymbolicString(t *testing.T) {
	powerset, err := Subsets(Interval(1, 20))
	if err != nil {
		t.Fatal(err)
	}
	if got := powerset.String(); got != "SUBSET 1..20" {
		t.Fatalf("got %q", got)
	}
}

func BenchmarkSubsetKey(b *testing.B) {
	powerset, err := Subsets(Interval(1, 15))
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for b.Loop() {
		if _, err := powerset.Key(); err != nil {
			b.Fatal(err)
		}
	}
}
