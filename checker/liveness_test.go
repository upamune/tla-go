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

func TestUpstreamLiveness(t *testing.T) {
	f, err := os.Open("testdata/upstream-liveness.jsonl")
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
		if err := json.Unmarshal(scanner.Bytes(), &sample); err != nil {
			t.Fatal(err)
		}
		count++
		t.Run(sample.Name, func(t *testing.T) {
			if sample.Name == "no-initial-state" {
				// Java TLC reports search depth 1 even with no initial states.
				sample.Want.Depth = 1
			}
			result, err := Check(sample.Source, sample.Config)
			if err != nil {
				t.Fatal(err)
			}
			got := project(result)
			got.Trace = nil
			if !reflect.DeepEqual(got, sample.Want) {
				t.Fatalf("got %#v, want %#v", got, sample.Want)
			}
			if result.Status == LivenessViolation {
				checkLasso(t, sample.Source, sample.Config, result.Violation)
			} else if result.Status == PropertyViolation {
				checkGuardedTrace(t, sample.Source, sample.Config, result.Violation)
			}
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 99 {
		t.Fatalf("expected 99 cases, got %d", count)
	}
}

func TestUpstreamFairnessMatrix(t *testing.T) {
	f, err := os.Open("testdata/upstream-fairness-matrix.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
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
				checkLasso(t, sample.Source, sample.Config, result.Violation)
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

func checkLasso(t *testing.T, source, cfgText string, violation *Violation) {
	t.Helper()
	if violation.LoopStart == nil || *violation.LoopStart < 0 || *violation.LoopStart >= len(violation.Trace) {
		t.Fatalf("invalid loop: %#v", violation)
	}
	module, err := parser.ParseModule(source)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse(cfgText)
	if err != nil {
		t.Fatal(err)
	}
	m, err := prepare(module, cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, property := range m.guarded {
		if property.name == violation.Name {
			checkGuardedTrace(t, source, cfgText, violation)
			return
		}
	}
	for _, property := range m.properties {
		if property.name == violation.Name && (property.mode == staysOrOftenState || property.mode == staysOrOftenAction) {
			checkStaysOrOftenLasso(t, source, cfgText, violation)
			return
		}
		if property.name == violation.Name && property.mode == leadsToStaying {
			checkLeadsToStayingLasso(t, source, cfgText, violation)
			return
		}
		if property.name == violation.Name && (property.mode == oftenImpliesState || property.mode == oftenImpliesAction) {
			checkOftenImplicationLasso(t, source, cfgText, violation)
			return
		}
		if property.name == violation.Name && property.mode == orderedEventually {
			checkOrderedLasso(t, source, cfgText, violation)
			return
		}
	}
	checkTraceTransitions(t, m, violation)
	matched := false
	for _, p := range m.properties {
		if p.name == violation.Name && lassoViolates(t, m, p, violation) {
			matched = true
			break
		}
	}
	if !matched {
		t.Fatal("counterexample does not violate any instance of the named property")
	}
	checkFairLasso(t, m, violation)
}

func lassoViolates(t *testing.T, m *model, p temporalProperty, violation *Violation) bool {
	t.Helper()
	if p.mode == actionOften {
		return actionNeverInCycle(t, m, p, violation)
	}
	start := *violation.LoopStart
	falseInCycle := false
	allCycleBad := true
	triggered := false
	for i, step := range violation.Trace {
		holds, err := m.holdsProperty(p.predicate, step.State, p.locals)
		if err != nil {
			t.Fatal(err)
		}
		if !holds && i >= start {
			falseInCycle = true
		}
		if holds && i >= start {
			allCycleBad = false
		}
		if p.mode == leadsTo {
			trigger, err := m.holdsProperty(p.trigger, step.State, p.locals)
			if err != nil {
				t.Fatal(err)
			}
			triggered = !holds && (triggered || trigger)
		}
		if holds && (p.mode == eventually || (p.mode == often && i >= start)) {
			return false
		}
	}
	if p.mode == leadsTo {
		return triggered && allCycleBad
	}
	return p.mode != eventuallyAlways || falseInCycle
}

func actionNeverInCycle(t *testing.T, m *model, p temporalProperty, violation *Violation) bool {
	t.Helper()
	for i := *violation.LoopStart; i < len(violation.Trace); i++ {
		current := violation.Trace[i].State
		next := violation.Trace[*violation.LoopStart].State
		if i+1 < len(violation.Trace) {
			next = violation.Trace[i+1].State
		}
		result, err := m.program.Evaluate(p.predicate, evaluator.Context{Current: current, Next: next, Constants: m.constants, Locals: p.locals, Enabled: m.enabledCallback})
		if err != nil {
			t.Fatal(err)
		}
		ok, valid := result.Bool()
		if !valid || ok {
			return false
		}
	}
	return true
}

func checkTraceTransitions(t *testing.T, m *model, v *Violation) {
	t.Helper()
	init, _ := m.definition(m.cfg.Init)
	ok, err := m.holds(init, v.Trace[0].State, nil)
	if err != nil || !ok {
		t.Fatalf("trace does not start in Init: %v", err)
	}
	nextAction, _ := m.definition(m.cfg.Next)
	for i, step := range v.Trace {
		next := v.Trace[*v.LoopStart].State
		if i+1 < len(v.Trace) {
			next = v.Trace[i+1].State
		}
		if statesEqual(t, m, step.State, next) {
			continue
		}
		ok, err := m.holds(nextAction, step.State, next)
		if err != nil || !ok {
			t.Fatalf("invalid transition %d: %v", i, err)
		}
	}
}

func statesEqual(t *testing.T, m *model, a, b evaluator.State) bool {
	t.Helper()
	for _, name := range m.names {
		equal, err := value.Equal(a[name], b[name])
		if err != nil {
			t.Fatal(err)
		}
		if !equal {
			return false
		}
	}
	return true
}

func checkFairLasso(t *testing.T, m *model, v *Violation) {
	t.Helper()
	for _, fair := range m.fairness {
		if fair.recurrence {
			checkRecurrenceLasso(t, m, fair, v)
			continue
		}
		enabled, disabled, fired := false, false, false
		for i := *v.LoopStart; i < len(v.Trace); i++ {
			current := v.Trace[i].State
			next := v.Trace[*v.LoopStart].State
			if i+1 < len(v.Trace) {
				next = v.Trace[i+1].State
			}
			ok, err := m.enabledWithLocals(m.fairAngle(fair), current, nil, fair.locals)
			if err != nil {
				t.Fatal(err)
			}
			enabled = enabled || ok
			disabled = disabled || !ok
			ok, err = m.holdsFair(fair, current, next)
			if err != nil {
				t.Fatal(err)
			}
			fired = fired || ok
		}
		if !fired && ((fair.strong && enabled) || (!fair.strong && !disabled)) {
			t.Fatalf("counterexample loop violates fairness: %#v", v)
		}
	}
}

func checkRecurrenceLasso(t *testing.T, m *model, fair fairnessCondition, v *Violation) {
	t.Helper()
	for i := *v.LoopStart; i < len(v.Trace); i++ {
		current := v.Trace[i].State
		if !fair.recurrenceAction {
			ok, err := m.holdsProperty(fair.action, current, fair.locals)
			if err != nil {
				t.Fatal(err)
			}
			if ok {
				return
			}
			continue
		}
		next := v.Trace[*v.LoopStart].State
		if i+1 < len(v.Trace) {
			next = v.Trace[i+1].State
		}
		context := evaluator.Context{Current: current, Next: next, Constants: m.constants, Locals: fair.locals, Enabled: m.enabledCallback}
		result, err := m.program.Evaluate(fair.action, context)
		if err != nil {
			t.Fatal(err)
		}
		if ok, valid := result.Bool(); valid && ok {
			return
		}
	}
	t.Fatalf("counterexample loop violates recurrence assumption: %#v", v)
}
