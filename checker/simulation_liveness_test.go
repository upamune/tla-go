package checker

import (
	"context"
	"testing"
)

// TLC recovers a counterexample state with its predecessor as the current
// state, so TLCGet("level") in the next-state action is the predecessor's
// position in the counterexample, not the level at the end of the trace.
func TestSimulationLivenessRecoveryLevel(t *testing.T) {
	const header = "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x = 0\n"
	opts := Options{Seed: 1, Simulation: &SimulationOptions{Traces: 3, Depth: 8}}
	cfg := "INIT Init NEXT Next PROPERTY P"
	source := header + "Next == x' = IF TLCGet(\"level\") < 4 THEN (x + 1) % 3 ELSE x\nP == []<>(x = 0)\n===="
	got, err := CheckWithOptions(context.Background(), source, cfg, opts)
	if err != nil || got.Status != LivenessViolation || len(got.Violation.Trace) != 3 || got.Violation.LoopAction != "Stuttering" {
		t.Fatalf("got %+v, %v; want TLC's three recovered states and stuttering", got, err)
	}
	// At level 1 only the first disjunct generates successors, so TLC fails
	// to recover the step from x = 0 to x = 2 that the trace takes later.
	source = header + "Next == \\/ TLCGet(\"level\") < 3 /\\ x' = (x + 1) % 3\n        \\/ TLCGet(\"level\") >= 3 /\\ x' = (x + 2) % 3\nP == <>[](x = 0)\n===="
	got, err = CheckWithOptions(context.Background(), source, cfg, opts)
	if err != nil || got.Status != EvaluationFailure || got.Failure.NestedToolCode != 2117 || len(got.Failure.TemporalTrace) != 1 {
		t.Fatalf("got %+v, %v; want TLC's failure to recover the second state", got, err)
	}
}
