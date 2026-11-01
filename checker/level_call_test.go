package checker

import (
	"bytes"
	"encoding/json"
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

func TestLevelExpansionCacheScopeAndRecursion(t *testing.T) {
	module, err := parser.ParseModule(`---- MODULE M ----
VARIABLE x
A(p) == ENABLED p
B(p) == p
Inv == A(x' = x) /\ B(x' = x)
====`)
	if err != nil {
		t.Fatal(err)
	}
	defs := make(map[string]parser.Definition)
	for _, def := range module.Definitions {
		defs[def.Name] = def
	}
	m := model{module: module, defs: defs, levelExpansions: make(map[*parser.Expression]*parser.Expression)}
	a, b := defs["Inv"].Body.Args[0], defs["Inv"].Body.Args[1]
	first, ok := m.levelCallBody(a, map[string]bool{})
	if !ok {
		t.Fatal("expected expansion")
	}
	again, ok := m.levelCallBody(a, map[string]bool{})
	if !ok || first != again {
		t.Fatal("expected cached expansion")
	}
	if _, ok := m.levelCallBody(a, map[string]bool{"A": true}); ok {
		t.Fatal("cache bypassed recursion guard")
	}
	if m.hasAction(a, map[string]bool{}) || !m.hasAction(b, map[string]bool{}) {
		t.Fatal("arguments or ENABLED shielding were confused")
	}
	local := m.temporalScope(&parser.Expression{Kind: "Let", Definitions: []parser.Definition{{Name: "A", Kind: "Operator", Params: defs["B"].Params, Body: defs["B"].Body}}})
	if local.levelExpansions != nil || !local.hasAction(a, map[string]bool{}) || m.hasAction(a, map[string]bool{}) {
		t.Fatal("local definition must not reuse enclosing expansion")
	}
}

func TestLevelExpansionCacheMatchesUncached(t *testing.T) {
	for _, tc := range []struct{ name, definitions, expression string }{
		{"different actual arguments", `F(p) == p`, `F(x' = x) /\ F(TRUE)`},
		{"nested binders and capture", `F(p) == \E n \in {1} : \A q \in {1} : ENABLED p`, `\A n \in {1} : F(x' = n)`},
		{"local shadow", `F(p) == p`, `LET F(p) == ENABLED p IN F(x' = x)`},
		{"recursive calls", "RECURSIVE F(_)\nF(p) == IF p THEN F(FALSE) ELSE p", `F(TRUE)`},
		{"fairness", `F(p) == WF_x(p)`, `F(x' = x)`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			module, err := parser.ParseModule("---- MODULE M ----\nVARIABLE x\n" + tc.definitions + "\nInv == " + tc.expression + "\n====\n")
			if err != nil {
				t.Fatal(err)
			}
			defs := make(map[string]parser.Definition)
			for _, def := range module.Definitions {
				defs[def.Name] = def
			}
			plain := model{module: module, defs: defs}
			cached := model{module: module, defs: defs, levelExpansions: make(map[*parser.Expression]*parser.Expression)}
			expr := defs["Inv"].Body
			before, err := json.Marshal(module)
			if err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if got, want := cached.hasTemporal(expr, map[string]bool{}), plain.hasTemporal(expr, map[string]bool{}); got != want {
					t.Fatalf("temporal: got %v, want %v", got, want)
				}
				if got, want := cached.hasAction(expr, map[string]bool{}), plain.hasAction(expr, map[string]bool{}); got != want {
					t.Fatalf("action: got %v, want %v", got, want)
				}
			}
			after, err := json.Marshal(module)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("analysis mutated its input")
			}
		})
	}
}
