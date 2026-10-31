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

// petersonSource follows the translation of the upstream test-model
// pcal/Peterson.tla, with its fairness and leads-to property.
const petersonSource = `---- MODULE Peterson ----
VARIABLES flag, turn, pc
vars == <<flag, turn, pc>>
Not(i) == IF i = 0 THEN 1 ELSE 0
Init == /\ flag = [i \in {0, 1} |-> FALSE]
        /\ turn = 0
        /\ pc = [self \in {0, 1} |-> "a0"]
a0(self) == /\ pc[self] = "a0" /\ pc' = [pc EXCEPT ![self] = "a1"] /\ UNCHANGED << flag, turn >>
a1(self) == /\ pc[self] = "a1" /\ flag' = [flag EXCEPT ![self] = TRUE] /\ pc' = [pc EXCEPT ![self] = "a2"] /\ turn' = turn
a2(self) == /\ pc[self] = "a2" /\ turn' = Not(self) /\ pc' = [pc EXCEPT ![self] = "a3"] /\ flag' = flag
a3(self) == /\ pc[self] = "a3"
            /\ IF flag[Not(self)] /\ turn = Not(self) THEN pc' = [pc EXCEPT ![self] = "a3"] ELSE pc' = [pc EXCEPT ![self] = "cs"]
            /\ UNCHANGED << flag, turn >>
cs(self) == /\ pc[self] = "cs" /\ pc' = [pc EXCEPT ![self] = "a4"] /\ UNCHANGED << flag, turn >>
a4(self) == /\ pc[self] = "a4" /\ flag' = [flag EXCEPT ![self] = FALSE] /\ pc' = [pc EXCEPT ![self] = "a0"] /\ turn' = turn
proc(self) == a0(self) \/ a1(self) \/ a2(self) \/ a3(self) \/ cs(self) \/ a4(self)
Next == \E self \in {0, 1} : proc(self)
FairSpec == /\ Init /\ [][Next]_vars
            /\ \A self \in {0, 1} : WF_vars(proc(self) /\ (pc[self] # "a0"))
DeadlockFree == \A i \in {0, 1} : (pc[i] = "a1") ~> (pc[i] = "cs")
====
`

// Quantified weak fairness evaluates ENABLED for every trace state. The run
// without a property measures the simulation itself.
func BenchmarkSimulationQuantifiedFairness(b *testing.B) {
	for name, configuration := range map[string]string{"spec": "SPECIFICATION FairSpec", "property": "SPECIFICATION FairSpec PROPERTY DeadlockFree"} {
		b.Run(name, func(b *testing.B) {
			opts := Options{Seed: 1, Simulation: &SimulationOptions{Traces: 4, Depth: 500}}
			b.ReportAllocs()
			for b.Loop() {
				result, err := CheckWithOptions(context.Background(), petersonSource, configuration, opts)
				if err != nil || result.Status != Success {
					b.Fatalf("simulation: %+v, %v", result, err)
				}
			}
		})
	}
}
