package checker

import (
	"context"
	"strings"
	"testing"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
)

func BenchmarkSimulationTraces(b *testing.B) {
	module, err := parser.ParseModule(simulationWalkSource)
	if err != nil {
		b.Fatal(err)
	}
	cfg, err := config.Parse("INIT Init NEXT Next")
	if err != nil {
		b.Fatal(err)
	}
	model, err := prepareWithOutput(module, cfg, Options{Simulation: &SimulationOptions{Traces: 100, Depth: 20}})
	if err != nil {
		b.Fatal(err)
	}
	model.ctx = context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		result, err := model.simulate(100, 20, 1)
		if err != nil || result.Status != Success {
			b.Fatalf("simulation: %+v, %v", result, err)
		}
		b.ReportMetric(float64(result.GeneratedStates), "states/op")
	}
}

func BenchmarkSimulationLiveness(b *testing.B) {
	source := strings.Replace(simulationWalkSource, "====", "P == <>(x = 0)\n====", 1)
	module, err := parser.ParseModule(source)
	if err != nil {
		b.Fatal(err)
	}
	cfg, err := config.Parse("INIT Init NEXT Next PROPERTY P")
	if err != nil {
		b.Fatal(err)
	}
	model, err := prepareWithOutput(module, cfg, Options{Simulation: &SimulationOptions{Traces: 100, Depth: 20}})
	if err != nil {
		b.Fatal(err)
	}
	model.ctx = context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		result, err := model.simulate(100, 20, 1)
		if err != nil || result.Status != Success {
			b.Fatalf("simulation: %+v, %v", result, err)
		}
		b.ReportMetric(float64(result.GeneratedStates), "states/op")
	}
}

func BenchmarkSimulationTraceConstraint(b *testing.B) {
	const source = `---- MODULE M ----
EXTENDS Naturals, Sequences, TLCExt
VARIABLE x
Init == x = 0
Next == x' = x + 1
Bound == Len(Trace) <= 100
====
`
	ctx := context.Background()
	module, err := parser.ParseModule(source)
	if err != nil {
		b.Fatal(err)
	}
	cfg, err := config.Parse("INIT Init NEXT Next CONSTRAINT Bound CHECK_DEADLOCK FALSE")
	if err != nil {
		b.Fatal(err)
	}
	session := newInstanceSession(module.Name)
	session.sourceUnits[module.Name] = module
	module, err = resolveExtensionsScoped(ctx, module, nil, session)
	if err != nil {
		b.Fatal(err)
	}
	module, err = resolveInstancesScoped(ctx, module, nil, session)
	if err != nil {
		b.Fatal(err)
	}
	model, err := prepareWithSources(module, cfg, Options{Simulation: &SimulationOptions{Traces: 100, Depth: 20}}, session.sourceUnits)
	if err != nil {
		b.Fatal(err)
	}
	model.ctx = ctx
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		result, err := model.simulate(100, 20, 1)
		if err != nil || result.Status != Success {
			b.Fatalf("simulation: %+v, %v", result, err)
		}
		b.ReportMetric(float64(result.GeneratedStates), "states/op")
	}
}
