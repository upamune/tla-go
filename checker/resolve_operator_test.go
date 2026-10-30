package checker

import (
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
)

func TestOperatorArgumentResolutionRejectsInvalidPrograms(t *testing.T) {
	for _, definition := range []string{
		"F(Op(_), a) == Op(a)\nBad == F(+, 1)",
		"F(a, a) == a",
		"B == 1\nF(B) == B",
		"F(Op(_)) == Op(1)\nBad == F(LAMBDA a,b : a + b)",
		"D(Op(_)) == Op(42)\nBad == D(D)",
		"B == 1\nF(Op(_)) == Op(1)\nBad == F(LAMBDA B : B)",
		"Bad == SelectSeq(<<1>>, LAMBDA a,b : a = b)",
	} {
		source := "---- MODULE M ----\nEXTENDS Naturals, Sequences\n" + definition + "\n===="
		module, err := parser.ParseModule(source)
		if err != nil {
			t.Fatalf("%s: %v", definition, err)
		}
		if err = validateResolved(module); err == nil {
			t.Errorf("accepted %s", definition)
		}
	}
}
