package value

import "testing"

func BenchmarkSetIntersection(b *testing.B) {
	left, right := Interval(1, 200), Interval(101, 300)
	for b.Loop() {
		intersection, err := SetOperation("\\cap", left, right)
		if err != nil {
			b.Fatal(err)
		}
		if _, err = intersection.Cardinality(); err != nil {
			b.Fatal(err)
		}
	}
}
