package checker

import (
	"bufio"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func TestUpstreamOftenImplicationMatrix(t *testing.T) {
	file, err := os.Open("testdata/upstream-often-implication-matrix.jsonl")
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
				checkOftenImplicationLasso(t, sample.Source, sample.Config, result.Violation)
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

func checkOftenImplicationLasso(t *testing.T, source, configText string, violation *Violation) {
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
	triggered := false
	for i := *violation.LoopStart; i < len(violation.Trace); i++ {
		trigger := checkOftenTrigger(t, m, p, violation, i)
		triggered = triggered || trigger
		if checkOftenGoal(t, m, p, violation, i) {
			t.Fatal("cycle satisfies the consequent")
		}
	}
	if !triggered {
		t.Fatal("cycle does not satisfy the antecedent")
	}
}

func checkOftenTrigger(t *testing.T, m *model, p temporalProperty, v *Violation, index int) bool {
	t.Helper()
	if p.triggerAction {
		return checkOftenAction(t, m, p.trigger, p.locals, v, index)
	}
	ok, err := m.holdsProperty(p.trigger, v.Trace[index].State, p.locals)
	if err != nil {
		t.Fatal(err)
	}
	return ok
}

func checkOftenGoal(t *testing.T, m *model, p temporalProperty, v *Violation, index int) bool {
	t.Helper()
	state := v.Trace[index].State
	if p.mode == oftenImpliesState {
		ok, err := m.holdsProperty(p.predicate, state, p.locals)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	return checkOftenAction(t, m, p.predicate, p.locals, v, index)
}

func checkOftenAction(t *testing.T, m *model, expr *parser.Expression, locals map[string]value.Value, v *Violation, index int) bool {
	t.Helper()
	state := v.Trace[index].State
	next := v.Trace[*v.LoopStart].State
	if index+1 < len(v.Trace) {
		next = v.Trace[index+1].State
	}
	ctx := evaluator.Context{Current: state, Next: next, Constants: m.constants, Locals: locals, Enabled: m.enabledCallback}
	result, err := m.program.Evaluate(expr, ctx)
	if err != nil {
		t.Fatal(err)
	}
	ok, valid := result.Bool()
	if !valid {
		t.Fatal("action goal is not boolean")
	}
	return ok
}

func TestOftenImplicationWithParameterizedAliases(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 3\nSpec == Init /\\ [][Next]_x\nFair == Spec /\\ WF_x(Next)\nOften(k) == []<>(x = k)\nP == Often(1) => Often(2)\n===="
	for _, tc := range []struct {
		spec   string
		status Status
	}{{"Spec", LivenessViolation}, {"Fair", Success}} {
		t.Run(tc.spec, func(t *testing.T) {
			configuration := "SPECIFICATION " + tc.spec + " PROPERTY P"
			result, err := Check(source, configuration)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != tc.status || result.GeneratedStates != 4 || result.DistinctStates != 3 {
				t.Fatalf("got %+v", result)
			}
			if result.Status == LivenessViolation {
				checkOftenImplicationLasso(t, source, configuration, result.Violation)
			}
		})
	}
}

func TestOftenImplicationInMultiplePropertyReport(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 3\nSpec == Init /\\ [][Next]_x /\\ WF_x(Next)\nMissing == <>(x = 9)\nP == []<>(x = 2) => []<>(x = 9)\nQ == []<>(x = 9) => []<>(x = 1)\nR == []<><<x' = x + 1>>_x => []<>(x = 9)\n===="
	result, err := Check(source, "SPECIFICATION Spec PROPERTIES Missing P Q R")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Missing", "P", "R"}
	if result.Status != LivenessViolation || result.Violation == nil || !reflect.DeepEqual(result.Violation.Properties, want) {
		t.Fatalf("got %+v violation=%+v", result, result.Violation)
	}
}
