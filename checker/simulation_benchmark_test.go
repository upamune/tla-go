package checker

import (
	"context"
	"fmt"
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
	benchmarkSimulationProperty(b, "<>(x = 0)")
}

func BenchmarkSimulationRecurrence(b *testing.B) {
	benchmarkSimulationProperty(b, "[]<>(x \\in {0, 1, 2})")
}

func BenchmarkSimulationLeadsTo(b *testing.B) {
	benchmarkSimulationProperty(b, "(x = 1) ~> (x = 1)")
}

func BenchmarkSimulationMixedTemporal(b *testing.B) {
	benchmarkSimulationProperty(b, "((x = 1) ~> (x = 1)) /\\ []<>(x \\in {0, 1, 2})")
}

func BenchmarkSimulationLeadsToUniqueStates(b *testing.B) {
	const source = "---- MODULE M ----\nEXTENDS Integers\nVARIABLE x\nInit == x = 0\nNext == x' = x + 1\nP == (x < 0) ~> (x = -1)\n===="
	for _, depth := range []int{250, 500, 1000} {
		b.Run(fmt.Sprintf("depth-%d", depth), func(b *testing.B) {
			opts := Options{Simulation: &SimulationOptions{Traces: 1, Depth: depth}}
			b.ReportAllocs()
			for b.Loop() {
				result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next PROPERTY P", opts)
				if err != nil || result.Status != Success {
					b.Fatalf("simulation: %+v, %v", result, err)
				}
				b.ReportMetric(float64(result.GeneratedStates), "states/op")
			}
		})
	}
}

func BenchmarkSimulationRecurrenceUniqueStates(b *testing.B) {
	const source = "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = x + 1\nP == []<>(x >= 0)\n===="
	for _, depth := range []int{250, 500, 1000} {
		b.Run(fmt.Sprintf("depth-%d", depth), func(b *testing.B) {
			opts := Options{Simulation: &SimulationOptions{Traces: 1, Depth: depth}}
			b.ReportAllocs()
			for b.Loop() {
				result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next PROPERTY P", opts)
				if err != nil || result.Status != Success {
					b.Fatalf("simulation: %+v, %v", result, err)
				}
				b.ReportMetric(float64(result.GeneratedStates), "states/op")
			}
		})
	}
}

// Weak and strong fairness add possible error models to TLC's solutions.
func BenchmarkSimulationFairness(b *testing.B) {
	benchmarkSimulationConfig(b, "Spec == Init /\\ [][Next]_x /\\ WF_x(Next) /\\ SF_x(C)\nP == (x = 1) ~> (x \\in {0, 2})", "SPECIFICATION Spec PROPERTY P")
}

func benchmarkSimulationProperty(b *testing.B, property string) {
	b.Helper()
	benchmarkSimulationConfig(b, "P == "+property, "INIT Init NEXT Next PROPERTY P")
}

func benchmarkSimulationConfig(b *testing.B, definitions, configuration string) {
	b.Helper()
	source := strings.Replace(simulationWalkSource, "====", definitions+"\n====", 1)
	module, err := parser.ParseModule(source)
	if err != nil {
		b.Fatal(err)
	}
	cfg, err := config.Parse(configuration)
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
