package checker

import (
	"bufio"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
)

func TestUpstreamLeadsToStayingMatrix(t *testing.T) {
	file, err := os.Open("testdata/upstream-leads-staying-matrix.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		var sample struct{ Name, Source, Config, Status string }
		if err := json.Unmarshal(scanner.Bytes(), &sample); err != nil {
			t.Fatal(err)
		}
		count++
		t.Run(sample.Name, func(t *testing.T) {
			result, err := Check(sample.Source, sample.Config)
			if err != nil {
				t.Fatal(err)
			}
			if string(result.Status) != sample.Status {
				t.Fatalf("got %s, want %s", result.Status, sample.Status)
			}
			if result.Status == LivenessViolation {
				checkLeadsToStayingLasso(t, sample.Source, sample.Config, result.Violation)
			}
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 400 {
		t.Fatalf("expected 400 cases, got %d", count)
	}
}

func checkLeadsToStayingLasso(t *testing.T, source, configText string, v *Violation) {
	t.Helper()
	module, err := parser.ParseModule(source)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse(configText)
	if err != nil {
		t.Fatal(err)
	}
	m, err := prepare(module, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if v == nil || v.LoopStart == nil || *v.LoopStart < 0 || *v.LoopStart >= len(v.Trace) || len(m.properties) != 1 {
		t.Fatalf("invalid lasso: %+v", v)
	}
	checkTraceTransitions(t, m, v)
	checkFairLasso(t, m, v)
	p := m.properties[0]
	triggered, bad := false, false
	for i, step := range v.Trace {
		trigger, err := m.holdsProperty(p.trigger, step.State, p.locals)
		if err != nil {
			t.Fatal(err)
		}
		triggered = triggered || trigger
		if i >= *v.LoopStart {
			ok, err := m.holdsProperty(p.predicate, step.State, p.locals)
			if err != nil {
				t.Fatal(err)
			}
			bad = bad || !ok
		}
	}
	if !triggered || !bad {
		t.Fatalf("lasso does not violate property: %+v", v)
	}
}

func TestLeadsToStayingAliasesAndMultipleProperties(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 3\nSpec == Init /\\ [][Next]_x /\\ WF_x(Next)\nStayBelow(n) == [](x < n)\nMissing == <>(x = 9)\nP == (x = 2) ~> StayBelow(2)\nQ == (x = 0) ~> StayBelow(3)\n===="
	for _, tc := range []struct {
		properties string
		names      []string
	}{
		{"P", []string{"P"}},
		{"Missing P Q", []string{"Missing", "P"}},
	} {
		t.Run(tc.properties, func(t *testing.T) {
			result, err := Check(source, "SPECIFICATION Spec PROPERTIES "+tc.properties)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != LivenessViolation || result.Violation == nil || !reflect.DeepEqual(result.Violation.Properties, tc.names) || result.GeneratedStates != 4 || result.DistinctStates != 3 {
				t.Fatalf("got %+v violation=%+v", result, result.Violation)
			}
		})
	}
}
