package checker

import (
	"context"
	"strings"
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

// TLC generates the initial states again with the last state that the
// worker sampled as the current state: after a trace of full depth, that is
// the successor beyond the trace.
func TestSimulationLivenessInitialRecoveryLevel(t *testing.T) {
	opts := Options{Seed: 1, Simulation: &SimulationOptions{Traces: 1, Depth: 8}}
	for level, want := range map[string]Status{"8": LivenessViolation, "9": EvaluationFailure} {
		source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x = 0 /\\ TLCGet(\"level\") # " + level + "\nNext == x' = (x + 1) % 3\nP == []<>(x = 5)\n===="
		got, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next PROPERTY P", opts)
		if err != nil || got.Status != want || want == EvaluationFailure && got.Failure.NestedToolCode != 2123 {
			t.Fatalf("level %s: got %+v, %v; want %v", level, got, err, want)
		}
	}
}

// TLC's simulator prints an exception of LiveCheck1 and the whole sampled
// behavior. It prints the summary only after an EvalException.
func TestSimulationLivenessEvaluationErrors(t *testing.T) {
	const header = "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nNext == x' = (x + 1) % 4\n"
	cases := []struct {
		name, body, cfg string
		code, nested    int
		summary         bool
		message         string
	}{
		{"tuple", "Init == x = 0\nP == <>(<<1, 2>>[x] = 7)", "INIT Init NEXT Next PROPERTY P", 1000, 0, false, "java.lang.RuntimeException\n: Attempted to apply tuple"},
		{"non-boolean", "Init == x = 0\nP == <>(x + 1)", "INIT Init NEXT Next PROPERTY P", 2250, 0, false, "A state predicate was evaluated to a non-boolean value."},
		{"division", "Init == x = 0\nP == <>(10 \\div x = 1)", "INIT Init NEXT Next PROPERTY P", 1000, 2179, true, "tlc2.tool.EvalException\n: The second argument of \\div is 0."},
		{"fairness", "Init == x = 0\nBad == x' = <<5>>[x]\nSpec == Init /\\ [][Next]_x /\\ WF_x(Bad)\nP == []<>(x = 0)", "SPECIFICATION Spec PROPERTY P", 1000, 0, false, "Attempted to apply tuple"},
	}
	opts := Options{Seed: 1, Simulation: &SimulationOptions{Traces: 2, Depth: 10}}
	for _, tc := range cases {
		got, err := CheckWithOptions(context.Background(), header+tc.body+"\n====", tc.cfg, opts)
		if err != nil || got.Status != EvaluationFailure {
			t.Fatalf("%s: got %+v, %v; want an evaluation failure", tc.name, got, err)
		}
		f := got.Failure
		if f.ToolCode != tc.code || f.NestedToolCode != tc.nested || f.SuppressSummary == tc.summary || f.ExitCode != 255 || len(f.Trace) != 11 || f.TemporalViolation || !strings.Contains(f.Message, tc.message) {
			t.Fatalf("%s: got failure %+v", tc.name, f)
		}
	}
	// An exception while TLC prints the counterexample follows its header.
	source := header + "Init == x = 0 /\\ (TLCGet(\"level\") > 5 => <<1>>[3] = 1)\nP == []<>(x = 5)\n===="
	got, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next PROPERTY P", opts)
	if err != nil || got.Status != EvaluationFailure || !got.Failure.TemporalViolation || len(got.Failure.TemporalTrace) != 0 || got.Failure.ToolCode != 1000 {
		t.Fatalf("got %+v, %v; want TLC's exception while it recovers the initial state", got, err)
	}
}
