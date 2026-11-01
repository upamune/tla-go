package checker

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func BenchmarkSimulationScalarViewGraph(b *testing.B) {
	const source = `---- MODULE M ----
EXTENDS Naturals
VARIABLE x
Init == x=0
Next == x'=(x+1)%2
P == []<>(x=5)
V == x
====`
	opts := Options{Seed: 1, Simulation: &SimulationOptions{Traces: 1, Depth: 100}}
	b.ReportAllocs()
	for b.Loop() {
		result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next PROPERTY P VIEW V CHECK_DEADLOCK FALSE", opts)
		if err != nil || result.Status != LivenessViolation {
			b.Fatalf("result=%+v error=%v", result, err)
		}
	}
}

func BenchmarkSimulationScalarViewWideGraph(b *testing.B) {
	var names, initial []string
	for i := range 300 {
		name := fmt.Sprintf("v%d", i)
		names = append(names, name)
		initial = append(initial, name+"=0")
	}
	variables := strings.Join(names, ",")
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLES x," + variables +
		"\nInit == /\\ x=0 /\\ " + strings.Join(initial, " /\\ ") +
		"\nNext == /\\ x'=(x+1)%2 /\\ UNCHANGED <<" + variables +
		">>\nP == []<>(x=5)\nV == x\n===="
	opts := Options{Seed: 1, Simulation: &SimulationOptions{Traces: 1, Depth: 1000}}
	b.ReportAllocs()
	for b.Loop() {
		result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next PROPERTY P VIEW V CHECK_DEADLOCK FALSE", opts)
		if err != nil || result.Status != LivenessViolation {
			b.Fatalf("result=%+v error=%v", result, err)
		}
	}
}
