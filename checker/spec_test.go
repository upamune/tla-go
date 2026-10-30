package checker

import (
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
)

func TestSpecificationSubscriptAliases(t *testing.T) {
	for _, tc := range []struct {
		definitions, subscript string
		want                   bool
	}{
		{"vars == <<x,y>>", "vars", true},
		{"vars == <<x,other>>\nother == y", "vars", true},
		{"vars == <<x,x>>", "vars", false},
		{"vars == <<x>>", "vars", false},
		{"vars == other\nother == vars", "vars", false},
		{"vars == <<x,y>>", "<<vars,y>>", false},
	} {
		source := "---- MODULE M ----\nVARIABLES x,y\n" + tc.definitions + "\nResult == [][Next]_" + tc.subscript + "\n===="
		module, err := parser.ParseModule(source)
		if err != nil {
			t.Fatal(err)
		}
		body := module.Definitions[len(module.Definitions)-1].Body
		sub := body.Args[0].Args[1]
		if got := validSubscript(module, sub); got != tc.want {
			t.Errorf("%s with %s: got %v", tc.subscript, tc.definitions, got)
		}
	}
}

func TestSpecificationWithoutVariables(t *testing.T) {
	for _, tc := range []struct {
		name, action, assumption string
		status                   Status
	}{
		{"true action", "TRUE", "TRUE", Success},
		{"false action", "FALSE", "TRUE", Success},
		{"false assumption", "TRUE", "FALSE", AssumptionViolation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nSpec == [][" + tc.action + "]_<<>>\nASSUME " + tc.assumption + "\n===="
			result, err := Check(source, "SPECIFICATION Spec")
			if err != nil {
				t.Fatal(err)
			}
			depth := 0
			if tc.status == Success {
				depth = 1
			}
			if result.Status != tc.status || result.GeneratedStates != 0 || result.DistinctStates != 0 || result.Depth != depth {
				t.Fatalf("unexpected result: %+v", result)
			}
		})
	}
}
