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
