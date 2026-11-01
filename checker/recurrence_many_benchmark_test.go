package checker

import (
	"fmt"
	"strings"
	"testing"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
)

func BenchmarkPrepareManyRecurrences(b *testing.B) {
	for _, count := range []int{2000, 4000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			var source, configuration strings.Builder
			source.WriteString("---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x=0\nNext == x'=x\n")
			configuration.WriteString("INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n")
			for i := range count {
				fmt.Fprintf(&source, "P%d == []<>(x=0)\n", i)
				fmt.Fprintf(&configuration, "PROPERTY P%d\n", i)
			}
			source.WriteString("====\n")
			module, err := parser.ParseModule(source.String())
			if err != nil {
				b.Fatal(err)
			}
			cfg, err := config.Parse(configuration.String())
			if err != nil {
				b.Fatal(err)
			}
			units := map[string]*parser.Module{"M": module}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				model, err := prepareWithSources(module, cfg, Options{}, units)
				if err != nil || len(model.livenessOrder) != count {
					b.Fatalf("model=%v err=%v", model, err)
				}
			}
		})
	}
}
