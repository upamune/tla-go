package checker

import (
	"bufio"
	"encoding/json"
	"os"
	"testing"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

func TestUpstreamGuardedMatrix(t *testing.T) {
	testGuardedMatrix(t, "testdata/upstream-guarded-matrix.jsonl")
}

func TestUpstreamGuardedFairnessMatrix(t *testing.T) {
	testGuardedMatrix(t, "testdata/upstream-guarded-fairness-matrix.jsonl")
}

func testGuardedMatrix(t *testing.T, path string) {
	t.Helper()
	file, err := os.Open(path)
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
			if result.Status == PropertyViolation || result.Status == LivenessViolation {
				checkGuardedTrace(t, sample.Source, sample.Config, result.Violation)
			}
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 120 {
		t.Fatalf("expected 120 cases, got %d", count)
	}
}

func checkGuardedTrace(t *testing.T, source, configText string, violation *Violation) {
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
	if violation == nil || len(violation.Trace) == 0 || len(m.guarded) != 1 {
		t.Fatalf("invalid violation: %+v", violation)
	}
	init, _ := m.definition(m.cfg.Init)
	ok, err := m.holds(init, violation.Trace[0].State, nil)
	if err != nil || !ok {
		t.Fatalf("trace does not start in Init: %v", err)
	}
	armed, failed := false, false
	for i, step := range violation.Trace {
		if i > 0 && !statesEqual(t, m, violation.Trace[i-1].State, step.State) {
			next, _ := m.definition(m.cfg.Next)
			ctx := evaluator.Context{Current: violation.Trace[i-1].State, Next: step.State, Constants: m.constants}
			result, err := m.program.Evaluate(next, ctx)
			valid, isBool := result.Bool()
			if err != nil || !isBool || !valid {
				t.Fatalf("invalid transition %d: %v", i, err)
			}
		}
		p := m.guarded[0]
		trigger, err := m.holdsProperty(p.trigger, step.State, p.locals)
		if err != nil {
			t.Fatal(err)
		}
		armed = armed || trigger
		if armed {
			valid, err := m.holdsProperty(p.predicate, step.State, p.locals)
			if err != nil {
				t.Fatal(err)
			}
			failed = failed || !valid
		}
	}
	if !failed {
		t.Fatal("trace does not violate the guarded property")
	}
	if violation.LoopStart != nil {
		checkTraceTransitions(t, m, violation)
		checkFairLasso(t, m, violation)
	}
}
