package checker

import (
	"context"
	"errors"
	"maps"
	"reflect"
	"slices"
	"sync"
	"testing"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func TestSimulationCounterexamplePresentationRetainsPrefixes(t *testing.T) {
	const source = "---- MODULE M ----\nEXTENDS Naturals\nVARIABLES z, a\nInit == z=0 /\\ a=0\nNext == z'=z+1 /\\ UNCHANGED a\nP == <>(z=5)\nV == <<z,a>>\n===="
	m := prepareSimulationTest(t, source, "INIT Init NEXT Next PROPERTY P VIEW V")
	m.viewStates = true
	var retained [][]TraceStep
	var snapshots [][]TraceStep
	stopRead := make(chan struct{})
	var reader sync.WaitGroup
	defer func() { close(stopRead); reader.Wait() }()
	r := &simulationRun{model: m, options: SimulationOptions{OnCounterexampleStates: func(trace []TraceStep, from int) {
		if len(trace) != from+1 || cap(trace) != len(trace) {
			t.Fatalf("invalid prefix from=%d len=%d cap=%d", from, len(trace), cap(trace))
		}
		for _, step := range trace {
			if !reflect.DeepEqual(step.StateOrder, []string{"a", "z"}) || step.View == "" {
				t.Fatalf("missing presentation in callback: %+v", step)
			}
			if step.Action != "" && step.ActionRange.Start.Line == 0 {
				t.Fatalf("missing action location in callback: %+v", step)
			}
		}
		retained = append(retained, trace)
		snapshot := slices.Clone(trace)
		for i := range snapshot {
			snapshot[i].State = maps.Clone(snapshot[i].State)
			snapshot[i].StateOrder = slices.Clone(snapshot[i].StateOrder)
		}
		snapshots = append(snapshots, snapshot)
		if from == 0 {
			reader.Go(func() {
				for {
					select {
					case <-stopRead:
						return
					default:
						_ = trace[0].State["z"]
						_ = trace[0].StateOrder[0]
					}
				}
			})
		}
	}}}
	present := r.counterexamplePresenter()
	var trace []TraceStep
	for i := range 3 {
		v, err := value.Integer(int64(i))
		if err != nil {
			t.Fatal(err)
		}
		step := TraceStep{State: evaluator.State{"z": v, "a": v}}
		if i > 0 {
			step.Action = "Next"
		}
		trace = append(trace, step)
		present(trace, i)
	}
	if !reflect.DeepEqual(retained, snapshots) {
		t.Fatalf("retained prefixes were changed: got %+v, want %+v", retained, snapshots)
	}
}

func TestSimulationToolViewEligibility(t *testing.T) {
	for _, tc := range []struct {
		name, definitions string
		want              bool
	}{
		{"pure", "V == x", true},
		{"print", `V == IF PrintT(x) THEN x ELSE x`, false},
		{"indirect-print", "V == Helper\nHelper == IF PrintT(x) THEN x ELSE x", false},
		{"random", "V == RandomElement({x})", false},
		{"tlc-level", `V == <<x, TLCGet("level")>>`, false},
		{"tlc-register", `V == <<x, TLCGet(0)>>`, false},
		{"tlc-helper", "V == <<x, Helper>>\nHelper == TLCGet(\"level\")", false},
		{"tlc-higher-order", "V == <<x, Read(TLCGet)>>\nRead(F(_)) == F(\"level\")", false},
		{"tlc-let", "V == LET Read == TLCGet(\"level\") IN <<x, Read>>", false},
		{"tlc-instance", "T == INSTANCE TLC\nV == <<x, T!TLCGet(\"level\")>>", false},
		{"print-higher-order", "V == F(PrintT)\nF(op(_)) == IF op(x) THEN x ELSE x", false},
		{"pure-higher-order", "V == F(G)\nF(op(_)) == op(x)\nG(y) == y", false},
		{"local-higher-order", "V == LET F(op(_)) == op(x) IN F(PrintT)", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x=0\nNext == x'=x+1\nP == <>(x=3)\n" + tc.definitions + "\n===="
			var m *model
			if tc.name == "tlc-instance" {
				module, err := parser.ParseModule(source)
				if err != nil {
					t.Fatal(err)
				}
				module, err = resolveInstances(context.Background(), module, nil)
				if err != nil {
					t.Fatal(err)
				}
				cfg, err := config.Parse("INIT Init NEXT Next PROPERTY P VIEW V")
				if err != nil {
					t.Fatal(err)
				}
				m, err = prepare(module, cfg)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				m = prepareSimulationTest(t, source, "INIT Init NEXT Next PROPERTY P VIEW V")
			}
			if got := (&simulationRun{model: m}).toolLivenessCompatible(); got != tc.want {
				t.Fatalf("eligible=%t, want %t", got, tc.want)
			}
		})
	}
}

func TestSimulationToolViewOperatorConstantFallback(t *testing.T) {
	const source = `---- MODULE M ----
EXTENDS Naturals, TLC
CONSTANT F(_)
VARIABLE x
Init == x=0
Next == x'=(x+1)%3
P == <>(x=8)
Read(i) == TLCGet("level")
V == <<x, F(x)>>
====`
	called := false
	result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next PROPERTY P VIEW V CONSTANT F <- Read", Options{
		Seed: 1, Simulation: &SimulationOptions{Traces: 1, Depth: 5, OmitLassoPropertyNames: true, OnCounterexampleStart: func() { called = true }},
	})
	if err != nil || called || result.Status != LivenessViolation && result.Status != EvaluationFailure {
		t.Fatalf("runtime VIEW must retain the existing path: status=%s called=%t error=%v", result.Status, called, err)
	}
}

func TestSimulationToolViewConfiguredOverrideFallback(t *testing.T) {
	const source = `---- MODULE M ----
EXTENDS Naturals, TLC
VARIABLE x
Init == x=0
Next == x'=(x+1)%3
P == <>(x=8)
Read(i) == TRUE
V == IF Read(x) THEN x ELSE x
====`
	called := false
	result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next PROPERTY P VIEW V CONSTANT Read <- PrintT", Options{
		Seed: 1, Simulation: &SimulationOptions{Traces: 1, Depth: 5, OmitLassoPropertyNames: true, OnCounterexampleStart: func() { called = true }},
	})
	if err != nil || called || result.Status != LivenessViolation && result.Status != EvaluationFailure {
		t.Fatalf("overridden VIEW must retain the existing path: status=%s called=%t error=%v", result.Status, called, err)
	}
}

func TestSimulationToolViewRuntimeFallback(t *testing.T) {
	for _, tc := range []struct{ name, view string }{
		{"level", `V == <<x, TLCGet("level")>>`},
		{"higher-order", "F(op(_)) == IF op(x) THEN x ELSE x\nV == F(PrintT)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x=0\nNext == x'=(x+1)%3\nSpec == Init /\\ [][Next]_x /\\ WF_x(Next)\nP == <>(x=8)\n" + tc.view + "\n===="
			called := false
			result, err := CheckWithOptions(context.Background(), source, "SPECIFICATION Spec PROPERTY P VIEW V CHECK_DEADLOCK FALSE", Options{
				Seed: 1, Simulation: &SimulationOptions{
					Traces: 1, Depth: 5, OmitLassoPropertyNames: true,
					OnCounterexampleStart:  func() { called = true },
					OnCounterexampleStates: func([]TraceStep, int) { called = true },
					OnBehaviorStart:        func(*Failure) { called = true },
					OnBehaviorState:        func(TraceStep, int) { called = true },
				},
			})
			if err != nil || called || result.Status != LivenessViolation && result.Status != EvaluationFailure {
				t.Fatalf("runtime VIEW must retain the existing path: status=%s called=%t error=%v", result.Status, called, err)
			}
		})
	}
}

func TestSimulationCounterexampleStart(t *testing.T) {
	const header = "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\n"
	for _, tc := range []struct {
		name, init, prop string
		status           Status
		want             []string
	}{
		{"lasso", `x=0 /\ PrintT("init")`, `[]<>(x=5)`, LivenessViolation, []string{`"init"`, "start", `"init"`}},
		{"failed recovery", `x=0 /\ PrintT("init") /\ TLCGet("level") # 4`, `[]<>(x=5)`, EvaluationFailure, []string{`"init"`, "start", `"init"`}},
		{"success", `x=0 /\ PrintT("init")`, `<>(x>=0)`, Success, []string{`"init"`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var events []string
			source := header + "Init == " + tc.init + "\nNext == x'=(x+1)%2\nP == " + tc.prop + "\n===="
			opts := Options{Seed: 1, OnPrint: func(s string) { events = append(events, s) }, Simulation: &SimulationOptions{Traces: 1, Depth: 3, OmitLassoPropertyNames: true, OnCounterexampleStart: func() { events = append(events, "start") }}}
			got, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE", opts)
			if err != nil || got.Status != tc.status || !reflect.DeepEqual(events, tc.want) || got.SimulationCounterexampleStarted != (tc.status != Success) {
				t.Fatalf("got %+v, %v, events %q; want %s %q", got, err, events, tc.status, tc.want)
			}
		})
	}
}

func TestSimulationFailedBehaviorCancellation(t *testing.T) {
	const source = `---- MODULE M ----
EXTENDS Naturals, TLC
VARIABLE x
Init == x=0
Next == /\ PrintT(TLCGet("level")) /\ x' = IF TLCGet("level") < 3 THEN (x+1)%3 ELSE (x+2)%3
P == <>[](x=0)
====`
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var events []string
	_, err := CheckWithOptions(ctx, source, "INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE", Options{
		Seed:    1,
		OnPrint: func(s string) { events = append(events, s) },
		Simulation: &SimulationOptions{
			Traces: 1, Depth: 8, OmitLassoPropertyNames: true,
			OnBehaviorStart: func(*Failure) { events = append(events, "failure") },
			OnBehaviorState: func(_ TraceStep, number int) {
				events = append(events, "state")
				if number == 1 {
					cancel()
				}
			},
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if len(events) < 2 || !reflect.DeepEqual(events[len(events)-2:], []string{"failure", "state"}) {
		t.Fatalf("unexpected events: %q", events)
	}
}

func TestSimulationFailedBehaviorRecoveryError(t *testing.T) {
	const source = `---- MODULE M ----
EXTENDS Naturals, TLC
VARIABLE x
Init == /\ TLCSet(0, 0) /\ x=0
Next == IF TLCGet(0)=1 /\ TLCGet("level")=1
        THEN x' = 1 \div 0
        ELSE /\ TLCSet(0, 1) /\ x' = IF TLCGet("level") < 3 THEN (x+1)%3 ELSE (x+2)%3
P == <>[](x=0)
====`
	var events []string
	_, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE", Options{
		Seed: 1, Simulation: &SimulationOptions{
			Traces: 1, Depth: 8, OmitLassoPropertyNames: true,
			OnBehaviorStart: func(*Failure) { events = append(events, "failure") },
			OnBehaviorState: func(_ TraceStep, _ int) { events = append(events, "state") },
		},
	})
	if err == nil || !reflect.DeepEqual(events, []string{"failure", "state"}) {
		t.Fatalf("expected separate recovery error after the first state, got %v and %q", err, events)
	}
}

func TestSimulationCounterexampleStateStream(t *testing.T) {
	const source = `---- MODULE M ----
EXTENDS Naturals, TLC
VARIABLE x
Init == x=0
Next == /\ PrintT(x) /\ x'=(x+1)%3
P == <>[](x=0)
====`
	var events []string
	result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE", Options{
		Seed: 1, OnPrint: func(s string) { events = append(events, s) },
		Simulation: &SimulationOptions{
			Traces: 1, Depth: 5, OmitLassoPropertyNames: true,
			OnCounterexampleStart: func() { events = append(events, "start") },
			OnCounterexampleStates: func(trace []TraceStep, from int) {
				for range trace[from:] {
					events = append(events, "state")
				}
			},
		},
	})
	if err != nil || result.Status != LivenessViolation || !result.SimulationCounterexampleStates {
		t.Fatalf("result %+v, error %v", result, err)
	}
	start := -1
	for i, event := range events {
		if event == "start" {
			start = i
			break
		}
	}
	if start < 0 || !reflect.DeepEqual(events[start:], []string{"start", "state", "0", "state", "1", "state", "2"}) {
		t.Fatalf("unexpected recovery events: %q", events)
	}
}
