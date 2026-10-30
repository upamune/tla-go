package evaluator

import (
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func TestHigherOperatorErrors(t *testing.T) {
	for _, expression := range []string{
		"SelectSeq(<<1>>, LAMBDA n : n)",
		"SelectSeq(<<1>>, LAMBDA a,b : a = b)",
		"LET Apply(F(_), v) == F(v) IN Apply(+, 1)",
	} {
		module, err := parser.ParseModule("---- MODULE M ----\nEXTENDS Naturals, Sequences\nResult == " + expression + "\n====")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = Evaluate(module, module.Definitions[0].Body, Context{}); err == nil {
			t.Errorf("accepted %s", expression)
		}
	}
}

func TestPrimedOperatorArguments(t *testing.T) {
	for _, tc := range []struct{ expr, want string }{
		{"D(x)", "2"},
		{"E(x)", "2"},
		{"F(x)", "3"},
		{"D(x + 1)", "3"},
		{"G(x)", "4"},
		{"H(x)", "TRUE"},
		{"D(y)", "9"},
	} {
		t.Run(tc.expr, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLES x, y\nD(a) == a'\nE(a) == D(a)\nF(a) == D(a + 1)\nG(a) == (a + 2)'\nH(a) == (a = 2)'\nResult == " + tc.expr + "\n===="
			module, err := parser.ParseModule(source)
			if err != nil {
				t.Fatal(err)
			}
			one, _ := value.Integer(1)
			two, _ := value.Integer(2)
			seven, _ := value.Integer(7)
			nine, _ := value.Integer(9)
			ctx := Context{Current: State{"x": one, "y": seven}, Next: State{"x": two, "y": nine}}
			got, err := Evaluate(module, module.Definitions[len(module.Definitions)-1].Body, ctx)
			if err != nil || got.String() != tc.want {
				t.Fatalf("got %s, %v; want %s", got, err, tc.want)
			}
		})
	}
}
