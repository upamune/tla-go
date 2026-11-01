package checker

import (
	"context"
	"testing"
)

func BenchmarkSimulationBehaviorViewRecovery(b *testing.B) {
	const source = `---- MODULE M ----
EXTENDS Naturals, TLC
VARIABLES x, y
Init == x=0 /\ y=0
Next == ((TLCGet("level") < 3 /\ x' \in {x, (x+1)%3}) \/ (TLCGet("level") >= 3 /\ x'=(x+2)%3)) /\ y'=1-y
P == <>[](x=0)
V == x
====`
	opts := Options{Seed: 7, Simulation: &SimulationOptions{
		Traces: 1, Depth: 8, OmitLassoPropertyNames: true,
		OnBehaviorStart: func(*Failure) {}, OnBehaviorState: func(TraceStep, int) {},
	}}
	b.ReportAllocs()
	for b.Loop() {
		result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next PROPERTY P VIEW V CHECK_DEADLOCK FALSE", opts)
		if err != nil || result.Status != EvaluationFailure || !result.SimulationBehaviorStreamed {
			b.Fatalf("result=%+v error=%v", result, err)
		}
	}
}
