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

func TestUpstreamStaysOrOftenMatrix(t *testing.T) {
	file, err := os.Open("testdata/upstream-stays-or-often-matrix.jsonl")
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
				checkStaysOrOftenLasso(t, sample.Source, sample.Config, result.Violation)
			}
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 480 {
		t.Fatalf("expected 480 cases, got %d", count)
	}
}

func checkStaysOrOftenLasso(t *testing.T, source, configText string, v *Violation) {
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
	bad := false
	for i := *v.LoopStart; i < len(v.Trace); i++ {
		state := v.Trace[i].State
		ok, err := m.holdsProperty(p.trigger, state, p.locals)
		if err != nil {
			t.Fatal(err)
		}
		bad = bad || !ok
		var goal bool
		if p.mode == staysOrOftenAction {
			goal = checkOftenAction(t, m, p.predicate, p.locals, v, i)
		} else {
			goal, err = m.holdsProperty(p.predicate, state, p.locals)
			if err != nil {
				t.Fatal(err)
			}
		}
		if goal {
			t.Fatal("cycle satisfies the recurrence disjunct")
		}
	}
	if !bad {
		t.Fatal("cycle satisfies the eventual-stability disjunct")
	}
}

func TestStaysOrOftenAliasesAndMultipleProperties(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 3\nSpec == Init /\\ [][Next]_x /\\ WF_x(Next)\nStays(n) == <>[](x < n)\nOften(n) == []<>(x = n)\nMissing == <>(x = 9)\nP == Stays(2) \\/ Often(9)\nQ == Often(2) \\/ Stays(2)\n===="
	for _, tc := range []struct {
		properties string
		status     Status
		names      []string
	}{
		{"P", LivenessViolation, []string{"P"}},
		{"Q", Success, nil},
		{"Missing P Q", LivenessViolation, []string{"Missing", "P"}},
	} {
		t.Run(tc.properties, func(t *testing.T) {
			result, err := Check(source, "SPECIFICATION Spec PROPERTIES "+tc.properties)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != tc.status || result.GeneratedStates != 4 || result.DistinctStates != 3 {
				t.Fatalf("got %+v", result)
			}
			if tc.names != nil && (result.Violation == nil || !reflect.DeepEqual(result.Violation.Properties, tc.names)) {
				t.Fatalf("got violation %+v", result.Violation)
			}
		})
	}
}
