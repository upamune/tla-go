package evaluator

import (
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
)

func BenchmarkConstructedFunctionDomain(b *testing.B) {
	module, err := parser.ParseModule("---- MODULE M ----\nEXTENDS Naturals\nDomain == DOMAIN [i \\in 1..4, j \\in 1..4 |-> i+j]\n====")
	if err != nil {
		b.Fatal(err)
	}
	program := New(module)
	for b.Loop() {
		if _, err := program.Evaluate(module.Definitions[0].Body, Context{}); err != nil {
			b.Fatal(err)
		}
	}
}
