package checker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

type expectedTrace struct {
	Action string
	State  map[string]string
}
type expectedCheck struct {
	Status                     string
	Name                       string
	Generated, Distinct, Depth int
	Trace                      []expectedTrace
}

func TestUpstreamChecks(t *testing.T) {
	f, err := os.Open("testdata/upstream-checks.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		var sample struct {
			Name, Source, Config string
			Want                 expectedCheck
		}
		if err = json.Unmarshal(scanner.Bytes(), &sample); err != nil {
			t.Fatal(err)
		}
		count++
		t.Run(sample.Name, func(t *testing.T) {
			if sample.Name == "upstream-universal-action" {
				// Java TLC evaluates remaining predicates once after all next-state
				// variables are assigned; tla-ts counts duplicate witnesses.
				sample.Want.Generated = 3
			}
			result, err := Check(sample.Source, sample.Config)
			if sample.Name == "property-quantified-conjunction" || sample.Name == "property-action-quantified" {
				// Java TLC 1.7.4 rejects an action beneath a temporal quantifier
				// with 2214; tla-ts accepts these two fixtures.
				validation, ok := errors.AsType[*ModelValidationError](err)
				if !ok || validation.Code != 2214 {
					t.Fatalf("got %#v, %v; want Java TLC error 2214", result, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got := project(result)
			if !reflect.DeepEqual(got, sample.Want) {
				t.Fatalf("got %#v, want %#v", got, sample.Want)
			}
		})
	}
	if err = scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 100 {
		t.Fatalf("expected 100 cases, got %d", count)
	}
}

func project(result Result) expectedCheck {
	out := expectedCheck{Status: string(result.Status), Generated: result.GeneratedStates, Distinct: result.DistinctStates, Depth: result.Depth}
	if result.Violation == nil {
		return out
	}
	out.Name = result.Violation.Name
	out.Trace = make([]expectedTrace, 0, len(result.Violation.Trace))
	for _, step := range result.Violation.Trace {
		state := make(map[string]string, len(step.State))
		for name, v := range step.State {
			state[name] = v.String()
		}
		out.Trace = append(out.Trace, expectedTrace{step.Action, state})
	}
	return out
}

func TestRejectsUnsupportedOrUnresolvedModels(t *testing.T) {
	cases := []struct{ name, source, config string }{
		{"missing import", "---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == x' = x + 1\n====", "INIT Init NEXT Next"},
		{"unknown symbol", "---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == /\\ x' = x /\\ FALSE => Missing\n====", "INIT Init NEXT Next"},
		{"unsupported temporal property", "---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == x' = x\nP == []<>[](x' = x)\n====", "INIT Init NEXT Next PROPERTY P"},
		{"action inside temporal property", "---- MODULE M ----\nVARIABLE x\nInit == x \\in {}\nNext == x' = x\nP == <>(x' = x)\n====", "INIT Init NEXT Next PROPERTY P"},
		{"state-dependent fairness domain", "---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == x' = x\nSpec == Init /\\ [][Next]_x /\\ \\A n \\in {x} : WF_x(Next)\nP == <>(x = 0)\n====", "SPECIFICATION Spec PROPERTY P"},
		{"undefined fairness action in empty domain", "---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == x' = x\nSpec == Init /\\ [][Next]_x /\\ \\A n \\in {} : WF_x(Missing(n))\nP == <>(x = 0)\n====", "SPECIFICATION Spec PROPERTY P"},
		{"existential fairness", "---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == x' = x\nSpec == Init /\\ [][Next]_x /\\ \\E n \\in {0,1} : WF_x(Next)\nP == <>(x = 0)\n====", "SPECIFICATION Spec PROPERTY P"},
		{"constant next with no initial states", "---- MODULE M ----\nVARIABLE x\nInit == x \\in {}\nNext == TRUE\n====", "INIT Init NEXT Next"},
		{"temporal next without states", "---- MODULE M ----\nVARIABLE x\nInit == x \\in {}\nNext == x' = x /\\ (FALSE => <>TRUE)\n====", "INIT Init NEXT Next"},
		{"action invariant with no initial states", "---- MODULE M ----\nVARIABLE x\nInit == x \\in {}\nNext == x' = x\nInv == x' = x\n====", "INIT Init NEXT Next INVARIANT Inv"},
		{"recursive action", "---- MODULE M ----\nVARIABLE x\nInit == x = 0\nLoop == Loop\nNext == Loop\n====", "INIT Init NEXT Next"},
		{"recursive expression", "---- MODULE M ----\nVARIABLE x\nF(a) == F(a)\nInit == x = 0\nNext == x' = F(1)\n====", "INIT Init NEXT Next"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if result, err := Check(tt.source, tt.config); err == nil {
				t.Fatalf("accepted unsupported model: %#v", result)
			}
		})
	}
}

func TestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := CheckContext(ctx, "---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == x' = x\n====", "INIT Init NEXT Next")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

func BenchmarkCheckCounter(b *testing.B) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == /\\ x < 1000 /\\ x' = x + 1\n===="
	for i := 0; i < b.N; i++ {
		result, err := Check(source, "INIT Init NEXT Next CHECK_DEADLOCK FALSE")
		if err != nil || result.DistinctStates != 1001 {
			b.Fatalf("got %#v, %v", result, err)
		}
	}
}
