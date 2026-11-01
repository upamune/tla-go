package checker

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func BenchmarkConstantTableauManyProperties(b *testing.B) {
	for _, count := range []int{500, 2000, 4000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			var source, cfg strings.Builder
			source.WriteString("---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = x\n")
			cfg.WriteString("INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n")
			for i := range count {
				fmt.Fprintf(&source, "P%d == <>FALSE\n", i)
				fmt.Fprintf(&cfg, "PROPERTY P%d\n", i)
			}
			source.WriteString("====")
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				result, err := CheckWithOptions(context.Background(), source.String(), cfg.String(), Options{})
				if err != nil || result.Status != LivenessViolation {
					b.Fatalf("got %+v, %v", result, err)
				}
			}
		})
	}
}
