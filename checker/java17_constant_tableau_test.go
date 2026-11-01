package checker

import (
	"testing"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func TestJava17TableauClassificationSkipsLazyConstants(t *testing.T) {
	calls := 0
	lazy, err := value.LazyFunction(value.Interval(1, 2), func(value.Value, value.Value) (value.Value, error) {
		calls++
		return value.Bool(true), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	s := &search{model: &model{constants: map[string]value.Value{"F": lazy}}}
	name := &parser.Expression{Kind: "Identifier", Text: "F"}
	expr := &parser.Expression{Kind: "Apply", Text: "=", Args: []*parser.Expression{name, name}}
	if !s.pureConstantAtom(expr, nil) {
		t.Fatal("test expression is not a candidate constant atom")
	}
	if s.scalarConstantOperands(expr, nil) || calls != 0 {
		t.Fatalf("lazy constant was accepted or evaluated: calls=%d", calls)
	}
}

func TestJava17TableauModelPurity(t *testing.T) {
	for _, tc := range []struct {
		name, extensions, extra, next string
		substitution                  bool
		want                          bool
	}{
		{"pure", "Naturals, TLC", `P == \A i \in {0,1} : <>(i = i)`, `x' = (x + 1) % 3`, false, true},
		{"next output", "Naturals, TLC", `P == <>TRUE`, `x' = (x + 1) % 3 /\ PrintT(x)`, false, false},
		{"runtime value", "Naturals, TLC", `P == <>TRUE`, `x' = TLCGet(0)`, false, false},
		{"random operator", "Naturals, TLC", `P == <>TRUE`, `x' = RandomElement({0,1})`, false, false},
		{"json module", "Naturals, Json", `P == <>TRUE`, `x' = x`, false, false},
		{"external module", "Naturals, Other", `P == <>TRUE`, `x' = x`, false, false},
		{"local operator", "Naturals, TLC", `P == <>TRUE`, `LET F == x IN x' = F`, false, false},
		{"higher order", "Naturals, TLC", "F(Op(_)) == Op(x)\nP == <>TRUE", `x' = x`, false, false},
		{"assumption", "Naturals, TLC", "ASSUME PrintT(TRUE)\nP == <>TRUE", `x' = x`, false, false},
		{"configuration substitution", "Naturals", `P == <>TRUE`, `x' = x`, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS " + tc.extensions + "\nVARIABLE x\nInit == x = 0\nNext == " + tc.next + "\n" + tc.extra + "\n===="
			module, err := parser.ParseModule(source)
			if err != nil {
				t.Fatal(err)
			}
			m := &model{module: module, sourceUnits: map[string]*parser.Module{"M": module}, variables: map[string]bool{"x": true}}
			if tc.substitution {
				m.sourceConfig.Substitutions = []config.Substitution{{Name: "N", Target: "Other"}}
			}
			if got := m.java17PureEnvironment(); got != tc.want {
				t.Fatalf("java17PureEnvironment = %t, want %t", got, tc.want)
			}
		})
	}
}
