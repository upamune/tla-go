package checker

import (
	"bufio"
	"encoding/json"
	"os"
	"testing"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
)

func TestUpstreamOrderedMatrix(t *testing.T) {
	for _, name := range []string{"ordered", "sequence"} {
		t.Run(name, func(t *testing.T) { checkOrderedMatrix(t, name) })
	}
}

func checkOrderedMatrix(t *testing.T, name string) {
	t.Helper()
	file, err := os.Open("testdata/upstream-" + name + "-matrix.jsonl")
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
				checkOrderedLasso(t, sample.Source, sample.Config, result.Violation)
			}
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 240 {
		t.Fatalf("expected 240 cases, got %d", count)
	}
}

func checkOrderedLasso(t *testing.T, source, configText string, violation *Violation) {
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
	if violation == nil || violation.LoopStart == nil || *violation.LoopStart < 0 || *violation.LoopStart >= len(violation.Trace) || len(m.properties) != 1 {
		t.Fatalf("invalid lasso: %+v", violation)
	}
	checkTraceTransitions(t, m, violation)
	checkFairLasso(t, m, violation)
	p := m.properties[0]
	stage := 0
	steps := append([]TraceStep(nil), violation.Trace...)
	for range p.sequence {
		steps = append(steps, violation.Trace[*violation.LoopStart:]...)
	}
	for _, step := range steps {
		for stage < len(p.sequence) {
			holds, err := m.holdsProperty(p.sequence[stage], step.State, p.locals)
			if err != nil {
				t.Fatal(err)
			}
			if !holds {
				break
			}
			stage++
		}
		if stage == len(p.sequence) {
			t.Fatal("reported lasso satisfies the ordered property")
		}
	}
}
