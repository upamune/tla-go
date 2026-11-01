package value

import (
	"math/rand"
	"testing"
)

func BenchmarkNestedEquality(b *testing.B) {
	one, _ := Integer(1)
	tuple, _ := Tuple([]Value{one, Bool(true), String("a")})
	outer, _ := Tuple([]Value{tuple, tuple, tuple})
	b.ReportAllocs()
	for b.Loop() {
		equal, err := Equal(outer, outer)
		if err != nil || !equal {
			b.Fatal(equal, err)
		}
	}
}

func BenchmarkSetNormalization(b *testing.B) {
	const n = 4000
	for _, order := range []string{"ascending", "reverse", "shuffled", "duplicate-heavy"} {
		b.Run(order, func(b *testing.B) {
			items := make([]Value, n)
			permutation := rand.New(rand.NewSource(42)).Perm(n)
			for i := range items {
				v := i
				switch order {
				case "reverse":
					v = n - i
				case "shuffled":
					v = permutation[i]
				case "duplicate-heavy":
					v = permutation[i] % 16
				}
				items[i], _ = Integer(int64(v))
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := Set(items); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
