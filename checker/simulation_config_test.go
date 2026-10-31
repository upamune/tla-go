package checker

import (
	"context"
	"fmt"
	"testing"
)

func TestSimulationConfig(t *testing.T) {
	const source = `---- MODULE M ----
EXTENDS TLC
VARIABLE x
C == TLCGet("config")
Init == /\ PrintT(<<C.mode,C.deadlock,C.worker,C.seed,C.traces,C.depth,C.aril,C.sched>>) /\ x=0
Next == x'=1
Inv == x=0
====`
	for _, tc := range []struct {
		name          string
		options       SimulationOptions
		traces, depth int
		status        Status
	}{
		{"bounded", SimulationOptions{Traces: 2, Depth: 1, Aril: 17}, 2, 1, InvariantViolation},
		{"zero", SimulationOptions{Traces: 0, Depth: 0}, 0, 0, Success},
		{"negative", SimulationOptions{Traces: -2, Depth: 1}, -2, 1, InvariantViolation},
		{"unlimited depth", SimulationOptions{Traces: 1, Depth: -1}, 1, -1, InvariantViolation},
		{"maximum depth", SimulationOptions{Traces: 1, Depth: 2147483647}, 1, -1, InvariantViolation},
		{"unlimited traces", SimulationOptions{Traces: 77, Depth: 1, UnlimitedTraces: true}, -1, 1, InvariantViolation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output []string
			opts := Options{Seed: 42, Simulation: &tc.options, OnPrint: func(line string) { output = append(output, line) }}
			result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next INVARIANT Inv", opts)
			want := fmt.Sprintf(`<<"simulate", TRUE, 1, "42", %d, %d, "0", "random">>`, tc.traces, tc.depth)
			if err != nil || result.Status != tc.status || len(output) != 1 || output[0] != want {
				t.Fatalf("result %+v, output %q, error %v; want %s", result, output, err, want)
			}
		})
	}
}

func TestSimulationRejectsInvalidDepth(t *testing.T) {
	const source = "---- MODULE M ----\nVARIABLE x\nInit == x=0\nNext == UNCHANGED x\n===="
	_, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next", Options{Simulation: &SimulationOptions{Traces: 1, Depth: -2}})
	if err == nil {
		t.Fatal("accepted depth -2")
	}
}
