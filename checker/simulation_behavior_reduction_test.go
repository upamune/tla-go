package checker

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/value"
)

// These two distinct integer sequences have the same FP64 at polynomial 0.
// The collision is also reproduced by the pinned JAR's FP64 and IntValue.
func TestSimulationBehaviorSuppressesFingerprintCollision(t *testing.T) {
	const source = "---- MODULE M ----\nEXTENDS Naturals\nVARIABLES x,y,z\nInit == x=0 /\\ y=0 /\\ z=0\nNext == UNCHANGED <<x,y,z>>\n===="
	m := prepareSimulationTest(t, source, "INIT Init NEXT Next")
	order := m.symmetryVariableOrder()
	if len(order) != 3 {
		t.Fatalf("variable order: %v", order)
	}
	zero, _ := value.Integer(0)
	a, _ := value.Integer(-744498434)
	b, _ := value.Integer(-848252597)
	previous := evaluator.State{order[0]: zero, order[1]: zero, order[2]: zero}
	sampled := evaluator.State{order[0]: a, order[1]: b, order[2]: zero}
	oldFP, oldErr := m.javaFingerprint(previous, order)
	newFP, newErr := m.javaFingerprint(sampled, order)
	if oldErr != nil || newErr != nil || oldFP != newFP || oldFP != 0x460deba07d8025bf {
		t.Fatalf("fingerprints: %x %v, %x %v", oldFP, oldErr, newFP, newErr)
	}
	// getState may return no action; Java still suppresses the state if the
	// current and previous fingerprints collide.
	r := simulationRun{model: m}
	if _, show, err := r.recoverBehaviorStep(previous, sampled, order); err != nil || show {
		t.Fatalf("colliding state: show=%t error=%v", show, err)
	}
}

// Java fingerprints the current state before its predecessor and stops if the
// first fingerprint raises an exception. The public streaming path excludes
// effectful VIEW expressions; this isolates the lower-level ordering contract.
func TestSimulationBehaviorFingerprintOrderAndError(t *testing.T) {
	const source = "---- MODULE M ----\nEXTENDS Integers, TLC\nVARIABLE x\nInit == x=1\nNext == x'=2\nV == IF PrintT(x) THEN 1 \\div x ELSE 0\n===="
	for _, tc := range []struct {
		name       string
		sampled    int64
		wantPrints []string
		wantError  string
	}{
		{"current fails", 0, []string{"0"}, "division by zero"},
		{"both evaluated", 2, []string{"2", "1"}, "failed to recover sampled state action"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := prepareSimulationTest(t, source, "INIT Init NEXT Next VIEW V")
			var printed []string
			m.program = newEvaluatorProgram(m.module, func(s string) { printed = append(printed, s) }, m.tlcRuntime)
			prev, _ := value.Integer(1)
			cur, _ := value.Integer(tc.sampled)
			r := simulationRun{model: m}
			_, _, err := r.recoverBehaviorStep(evaluator.State{"x": prev}, evaluator.State{"x": cur}, m.symmetryVariableOrder())
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), tc.wantError) || !reflect.DeepEqual(printed, tc.wantPrints) {
				t.Fatalf("printed %q, error %v; want %q and %q", printed, err, tc.wantPrints, tc.wantError)
			}
		})
	}
}

func TestSimulationBehaviorPreviousIncludesSuppressedState(t *testing.T) {
	const source = `---- MODULE M ----
EXTENDS Naturals, TLC
VARIABLES x, y
Init == x=0 /\ y=0
Next == x' = (IF TLCGet("level") = 1 THEN x ELSE (x+1)%3) /\ y'=1-y
P == <>[](x=0)
V == x
====`
	for _, oldCallback := range []bool{false, true} {
		t.Run(fmt.Sprint(oldCallback), func(t *testing.T) {
			var previous []*TraceStep
			var current []TraceStep
			var events []string
			oldCount := 0
			options := SimulationOptions{
				Traces: 1, Depth: 8, OmitLassoPropertyNames: true,
				OnBehaviorStart: func(*Failure) {},
				OnBehaviorStateWithPrevious: func(step TraceStep, prior *TraceStep, _ int) {
					events = append(events, "with-previous")
					previous = append(previous, prior)
					current = append(current, step)
				},
			}
			if oldCallback {
				options.OnBehaviorState = func(TraceStep, int) { oldCount++; events = append(events, "old") }
			}
			result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next PROPERTY P VIEW V CHECK_DEADLOCK FALSE", Options{Seed: 1, ViewStates: true, Simulation: &options})
			if err != nil || result.Status != EvaluationFailure || !result.SimulationBehaviorStreamed || len(current) < 2 || previous[0] != nil {
				t.Fatalf("result %+v, error %v, current=%v previous=%v", result, err, current, previous)
			}
			if oldCallback && oldCount != len(current) {
				t.Fatalf("old callback received %d states; new received %d", oldCount, len(current))
			}
			if oldCallback {
				for i := 0; i < len(events); i += 2 {
					if events[i] != "old" || events[i+1] != "with-previous" {
						t.Fatalf("callback ordering: %q", events)
					}
				}
			}
			previousX, okX := previous[1].State["x"].Int()
			previousY, okY := previous[1].State["y"].Int()
			currentX, okCurrentX := current[1].State["x"].Int()
			currentY, okCurrentY := current[1].State["y"].Int()
			if !okX || !okY || !okCurrentX || !okCurrentY || previousX != 0 || previousY != 1 || currentX != 1 || currentY != 0 {
				t.Fatalf("second state: previous=%v current=%v", previous[1], current[1])
			}
			if previous[1].View != "0" || current[1].View != "1" || !reflect.DeepEqual(previous[1].StateOrder, current[1].StateOrder) || len(previous[1].StateOrder) != 2 || previous[1].ActionRange.Start.Line == 0 {
				t.Fatalf("unpresented second state: previous=%+v current=%+v", previous[1], current[1])
			}
		})
	}
}

func TestSimulationBehaviorEffectfulViewRetainsExistingPath(t *testing.T) {
	const source = `---- MODULE M ----
EXTENDS Naturals, TLC
VARIABLE x
Init == x=0
Next == x'=(x+1)%3
Spec == Init /\ [][Next]_x /\ WF_x(Next)
P == <>(x=8)
V == IF PrintT(<<"view", x>>) THEN x ELSE x
====`
	var baselinePrints, actualPrints []string
	baseOpts := Options{Seed: 1, ViewStates: true, OnPrint: func(s string) { baselinePrints = append(baselinePrints, s) }, Simulation: &SimulationOptions{Traces: 1, Depth: 5}}
	config := "SPECIFICATION Spec PROPERTY P VIEW V CHECK_DEADLOCK FALSE"
	baseline, baselineErr := CheckWithOptions(context.Background(), source, config, baseOpts)
	called := false
	baseOpts.OnPrint = func(s string) { actualPrints = append(actualPrints, s) }
	baseOpts.Simulation = &SimulationOptions{
		Traces: 1, Depth: 5, OmitLassoPropertyNames: true,
		OnBehaviorStart:             func(*Failure) { called = true },
		OnBehaviorStateWithPrevious: func(TraceStep, *TraceStep, int) { called = true },
	}
	actual, err := CheckWithOptions(context.Background(), source, config, baseOpts)
	if baselineErr != nil || err != nil || called || !reflect.DeepEqual(actual, baseline) || !reflect.DeepEqual(actualPrints, baselinePrints) || len(baselinePrints) == 0 {
		t.Fatalf("baseline=%+v error=%v prints=%q; actual=%+v error=%v prints=%q called=%t", baseline, baselineErr, baselinePrints, actual, err, actualPrints, called)
	}
}

// Both modes encounter the same failed liveness recovery. Only the explicit
// streaming mode calls the behavioral callbacks and compresses stutters.
func TestSimulationBehaviorReductionCallbacks(t *testing.T) {
	const common = `Next == PrintT(<<"set", TLCGet("level")>>) /\ ((TLCGet("level") < 3 /\ x' \in {x, (x+1)%3}) \/ (TLCGet("level") >= 3 /\ x' = (x+2)%3)) /\ `
	for _, tc := range []struct {
		name, variables, next, config string
		states                        int
	}{
		{"view", "VARIABLES x, y\nInit == x=0 /\\ y=0\n", "y'=1-y", "VIEW V", 7},
		{"symmetry", "CONSTANTS a,b\nVARIABLES x, y\nInit == x=0 /\\ y=a\n", "y'=IF y=a THEN b ELSE a", "CONSTANTS a=a b=b SYMMETRY Sym", 7},
		{"view-and-symmetry", "CONSTANTS a,b\nVARIABLES x, y\nInit == x=0 /\\ y=a\n", "y'=IF y=a THEN b ELSE a", "CONSTANTS a=a b=b SYMMETRY Sym VIEW V", 7},
		{"model-values", "CONSTANTS a,b\nVARIABLES x, y\nInit == x=0 /\\ y=a\n", "y'=IF y=a THEN b ELSE a", "CONSTANTS a=a b=b", 9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS Naturals, FiniteSets, TLC\n" + tc.variables + common + tc.next + "\nP == <>[](x=0)\nV == x\n"
			if tc.name != "view" {
				source += "Sym == Permutations({a,b})\n"
			}
			source += "===="
			cfg := "INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE " + tc.config
			var baselinePrints []string
			baseline, err := CheckWithOptions(context.Background(), source, cfg, Options{
				Seed: 7, OnPrint: func(s string) { baselinePrints = append(baselinePrints, s) }, Simulation: &SimulationOptions{Traces: 1, Depth: 8},
			})
			if err != nil || baseline.Status != EvaluationFailure {
				t.Fatalf("default API baseline: status %s, error %v", baseline.Status, err)
			}
			for _, omit := range []bool{false, true} {
				t.Run(fmt.Sprint(omit), func(t *testing.T) {
					starts, states := 0, 0
					var printed []string
					opts := Options{Seed: 7, OnPrint: func(s string) { printed = append(printed, s) }, Simulation: &SimulationOptions{
						Traces: 1, Depth: 8, OmitLassoPropertyNames: omit,
						OnBehaviorStart: func(*Failure) { starts++ },
						OnBehaviorState: func(_ TraceStep, number int) {
							states++
							if number != states {
								t.Fatalf("state number %d after %d states", number, states)
							}
						},
					}}
					result, err := CheckWithOptions(context.Background(), source, cfg, opts)
					if err != nil || result.Status != EvaluationFailure {
						t.Fatalf("status %s, error %v", result.Status, err)
					}
					if omit && (starts != 1 || states != tc.states || !result.SimulationBehaviorStreamed) {
						t.Fatalf("streamed=%t starts=%d states=%d, want 1 and %d", result.SimulationBehaviorStreamed, starts, states, tc.states)
					}
					if !omit && (starts != 0 || states != 0 || result.SimulationBehaviorStreamed) {
						t.Fatalf("default API unexpectedly streamed: %+v starts=%d states=%d", result, starts, states)
					}
					if !omit && (!reflect.DeepEqual(result, baseline) || !reflect.DeepEqual(printed, baselinePrints)) {
						t.Fatalf("default API changed: result=%+v baseline=%+v prints=%q baseline prints=%q", result, baseline, printed, baselinePrints)
					}
				})
			}
		})
	}
}
