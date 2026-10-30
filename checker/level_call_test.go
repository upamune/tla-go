package checker

import (
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
)

func TestActionCallLevel(t *testing.T) {
	for _, tc := range []struct {
		name, definitions, expression string
		want                          bool
	}{
		{"enabled", `B(d) == ENABLED d`, `B(x' = x)`, false},
		{"ignored", `B(d) == TRUE`, `B(x' = x)`, false},
		{"unshielded", `B(d) == ENABLED d /\ d`, `B(x' = x)`, true},
		{"separate arguments", `B(a, b) == ENABLED a /\ b`, `B(x' = x, x' = 1)`, true},
		{"forwarded", "C(d) == ENABLED d\nB(d) == C(d)", `B(x' = x)`, false},
		{"capture", `B(d) == \E n \in {1} : ENABLED d`, `\A n \in {1} : B(x' = n)`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			module, err := parser.ParseModule("---- MODULE M ----\nVARIABLE x\n" + tc.definitions + "\nInv == " + tc.expression + "\n====\n")
			if err != nil {
				t.Fatal(err)
			}
			defs := make(map[string]parser.Definition, len(module.Definitions))
			for _, def := range module.Definitions {
				defs[def.Name] = def
			}
			m := model{module: module, defs: defs}
			if got := m.hasAction(defs["Inv"].Body, map[string]bool{}); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
