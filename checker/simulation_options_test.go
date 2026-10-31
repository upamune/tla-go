package checker

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestSimulationRejectsUnsupportedCallbacks(t *testing.T) {
	const source = "---- MODULE M ----\nVARIABLE x\nInit == x=0\nNext == UNCHANGED x\n===="
	for _, opts := range []Options{
		{Simulation: &SimulationOptions{Traces: 1, Depth: 1}, OnGraph: func(GraphEvent) error { return nil }},
		{Simulation: &SimulationOptions{Traces: 1, Depth: 1}, OnDebug: func(string) {}},
	} {
		_, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next", opts)
		if err == nil || !strings.Contains(err.Error(), "not supported in simulation mode") {
			t.Fatalf("got %v; want unsupported simulation callback", err)
		}
	}
}

func TestSimulationRejectsUnsupportedLivenessOptions(t *testing.T) {
	const source = "---- MODULE M ----\nVARIABLE x\nInit == x=0\nNext == UNCHANGED x\nSpec == Init /\\ [][Next]_x /\\ WF_x(Next)\nP == <>(x=0)\nV == x\n===="
	for _, tc := range []struct {
		name, cfg, message string
		depth              int
	}{
		{"fairness", "SPECIFICATION Spec PROPERTY P", "simulation of temporal properties is not yet supported", 1},
		{"view", "INIT Init NEXT Next VIEW V PROPERTY P", "simulation of temporal properties is not yet supported", 1},
		{"zero depth", "INIT Init NEXT Next PROPERTY P", "zero-depth liveness simulation is not yet supported", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CheckWithOptions(context.Background(), source, tc.cfg, Options{Simulation: &SimulationOptions{Traces: 1, Depth: tc.depth}})
			if err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("got %v; want %s", err, tc.message)
			}
		})
	}
}

func TestSimulationCompletedTraceCallback(t *testing.T) {
	const source = "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x=0\nNext == x'=x+1\nC == x < 2\n===="
	var indices []int
	var states [][]int32
	opts := Options{Simulation: &SimulationOptions{Traces: 2, Depth: 6, OnTrace: func(index int, trace []TraceStep) error {
		indices = append(indices, index)
		var traceStates []int32
		for i, step := range trace {
			n, ok := step.State["x"].Int()
			if !ok {
				t.Fatal("trace state is not an integer")
			}
			if i > 0 && step.Action == "" {
				t.Fatal("trace is missing its action")
			}
			traceStates = append(traceStates, n)
		}
		states = append(states, traceStates)
		return nil
	}}}
	result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next CONSTRAINT C CHECK_DEADLOCK FALSE", opts)
	if err != nil || result.Status != Success || !reflect.DeepEqual(indices, []int{0, 1}) || !reflect.DeepEqual(states, [][]int32{{0, 1, 2}, {0, 1, 2}}) {
		t.Fatalf("result %+v, err %v, indices %v, states %v", result, err, indices, states)
	}
}

func TestSimulationTraceCallbackError(t *testing.T) {
	const source = "---- MODULE M ----\nVARIABLE x\nInit == x=0\nNext == UNCHANGED x\n===="
	failure := errors.New("trace output failed")
	opts := Options{Simulation: &SimulationOptions{Traces: 2, Depth: 1, OnTrace: func(int, []TraceStep) error { return failure }}}
	_, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next", opts)
	if !errors.Is(err, failure) {
		t.Fatalf("got %v; want %v", err, failure)
	}
}

func TestSimulationAssertionFailureTrace(t *testing.T) {
	const source = "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x=0\nNext == x'=x+1 /\\ Assert(x < 2, \"stop\")\n===="
	result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next", Options{Simulation: &SimulationOptions{Traces: 1, Depth: 10}})
	failure := result.Failure
	if err != nil || result.Status != EvaluationFailure || failure == nil || failure.ToolCode != 1000 || failure.NestedToolCode != 2132 || failure.ExitCode != 255 || len(failure.Trace) != 3 || failure.SuppressStatistics {
		t.Fatalf("result %+v, failure %+v, err %v", result, failure, err)
	}
	if !strings.HasSuffix(failure.Message, "The exception was a tlc2.tool.EvalException\n: "+failure.NestedMessage) {
		t.Fatalf("message %q", failure.Message)
	}
}
