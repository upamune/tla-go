package evaluator

import (
	"math"
	"testing"

	"github.com/upamune/tla-go/value"
)

func BenchmarkRandomSubsetInterval(b *testing.B) {
	for _, tc := range []struct {
		name string
		set  value.Value
	}{
		{"signed-range", value.Interval(0, math.MaxInt32-1)},
		{"small-range", value.Interval(0, 999)},
	} {
		b.Run(tc.name, func(b *testing.B) {
			for _, mode := range []struct {
				name   string
				modern bool
			}{{"tlc-1.7.4", false}, {"modern", true}} {
				b.Run(mode.name, func(b *testing.B) {
					random := NewJavaRandom(1)
					random.SetModernSubsets(mode.modern)
					for b.Loop() {
						items, err := subsetInterval(5, tc.set, random)
						if err != nil || len(items) != 5 {
							b.Fatalf("got %d items, %v", len(items), err)
						}
					}
				})
			}
		})
	}
}
