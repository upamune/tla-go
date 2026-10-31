package checker

import (
	"context"
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
