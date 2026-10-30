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

func TestSpecificationFairnessSubscript(t *testing.T) {
	for _, tc := range []struct {
		name, fairness string
		status         Status
	}{
		{"weak x", "WF_x(Next)", Success},
		{"strong x", "SF_x(Next)", Success},
		{"weak y", "WF_y(Next)", LivenessViolation},
		{"strong y", "SF_y(Next)", LivenessViolation},
		{"both", "WF_x(Next) /\\ WF_y(Next)", Success},
		{"expression", "WF_sum(Next)", LivenessViolation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLES x,y\nvars == <<x,y>>\nsum == y+0\n" +
				"Init == x=0 /\\ y=0\nNext == x' \\in {x,1} /\\ y' \\in {y,1}\n" +
				"Spec == Init /\\ [][Next]_vars /\\ " + tc.fairness + "\nP == <>(x=1)\n===="
			cfg := "SPECIFICATION Spec PROPERTY P CHECK_DEADLOCK FALSE"
			result, err := Check(source, cfg)
			if err != nil || result.Status != tc.status {
				t.Fatalf("got %+v, %v; want %s", result, err, tc.status)
			}
			if result.Status == LivenessViolation {
				checkLasso(t, source, cfg, result.Violation)
			}
		})
	}
}
