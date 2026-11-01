package checker

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

const parallelModel = `---- MODULE M ----
EXTENDS Naturals
VARIABLE x
Init == x = 0
Next == \/ /\ x < 40 /\ x' = x + 1
        \/ /\ x < 40 /\ x' = x + 2
Inv == x < 35
====`

func TestParallelEligibility(t *testing.T) {
	module, err := parser.ParseModule(heavyParallelModel())
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse("INIT Init NEXT Next")
	if err != nil {
		t.Fatal(err)
	}
	session := newInstanceSession(module.Name)
	session.sourceUnits[module.Name] = module
	module, err = resolveExtensionsScoped(context.Background(), module, nil, session)
	if err != nil {
		t.Fatal(err)
	}
	module, err = resolveInstancesScoped(context.Background(), module, nil, session)
	if err != nil {
		t.Fatal(err)
	}
	m, err := prepareWithSources(module, cfg, Options{Workers: 4}, session.sourceUnits)
	if err != nil {
		t.Fatal(err)
	}
	next, err := m.definition("Next")
	if err != nil {
		t.Fatal(err)
	}
	if !m.parallelEligible(next) {
		t.Fatal("heavy model should be eligible")
	}
}

func TestParallelDifferential(t *testing.T) {
	cases := []struct{ name, source, config string }{
		{"heavy success", heavyParallelModel(), "INIT Init NEXT Next CHECK_DEADLOCK FALSE"},
		{"heavy invariant", strings.Replace(strings.Replace(heavyParallelModel(), "0..999", "0..99", 1), "====", "Inv == x < 100\n====", 1), "INIT Init NEXT Next INVARIANT Inv"},
		{"success", parallelModel, "INIT Init NEXT Next CHECK_DEADLOCK FALSE"},
		{"invariant", parallelModel, "INIT Init NEXT Next INVARIANT Inv"},
		{"constraint", parallelModel, "INIT Init NEXT Next CONSTRAINT Inv"},
		{"invariant evaluation", `---- MODULE M ----
EXTENDS Naturals
VARIABLE x
Init == x = 0
Next == \/ /\ x < 6 /\ x' = x + 1
        \/ /\ x < 6 /\ x' = x + 2
Inv == x < 3 \/ 1 \div (x - 3) = 1
====`, "INIT Init NEXT Next INVARIANT Inv"},
		{"evaluation", `---- MODULE M ----
EXTENDS Naturals
VARIABLE x
Init == x = 0
Next == \/ /\ x < 4 /\ x' = x + 1
        \/ /\ x > 1 /\ x' = 1 \div (x - 2)
====`, "INIT Init NEXT Next"},
		{"effects", `---- MODULE M ----
EXTENDS Naturals, TLC
VARIABLE x
Init == x = 0
Next == /\ PrintT(x) /\ x' = (x + 1) % 4
====`, "INIT Init NEXT Next"},
		{"liveness", `---- MODULE M ----
EXTENDS Naturals
VARIABLE x
Init == x = 0
Next == x' = (x + 1) % 4
Live == <> (x = 3)
====`, "INIT Init NEXT Next PROPERTY Live"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var outputs [2][]string
			var results [2]Result
			var errs [2]error
			for i, w := range []int{1, 4} {
				results[i], errs[i] = CheckWithOptions(context.Background(), tc.source, tc.config, Options{Workers: w, OnPrint: func(s string) { outputs[i] = append(outputs[i], s) }})
			}
			if errs[0] != nil {
				t.Fatalf("unexpected error: %v", errs[0])
			}
			if !reflect.DeepEqual(results[0], results[1]) || !reflect.DeepEqual(outputs[0], outputs[1]) || (errs[0] == nil) != (errs[1] == nil) || errs[0] != nil && errs[0].Error() != errs[1].Error() {
				t.Fatalf("serial: %+v %v %q\nparallel: %+v %v %q", results[0], errs[0], outputs[0], results[1], errs[1], outputs[1])
			}
		})
	}
}

func BenchmarkParallelBFS(b *testing.B) {
	for _, w := range []int{1, 4} {
		b.Run(map[int]string{1: "workers1", 4: "workers4"}[w], func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				r, err := CheckWithOptions(context.Background(), parallelModel, "INIT Init NEXT Next CHECK_DEADLOCK FALSE", Options{Workers: w})
				if err != nil || r.Status != Success {
					b.Fatalf("%+v %v", r, err)
				}
			}
		})
	}
}

func BenchmarkParallelWideBFS(b *testing.B) {
	source := `---- MODULE M ----
EXTENDS Naturals
VARIABLE x
Init == x \in 0..999
Next == x' = (x * x + 3 * x + 123) % 1000
====`
	for _, w := range []int{1, 4} {
		b.Run(map[int]string{1: "workers1", 4: "workers4"}[w], func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next", Options{Workers: w})
				if err != nil || result.Status != Success {
					b.Fatalf("%+v %v", result, err)
				}
			}
		})
	}
}

func TestParallelFallbacks(t *testing.T) {
	cases := []struct {
		name, source, cfg string
		options           Options
	}{
		{"tlc effects", strings.Replace(strings.Replace(heavyParallelModel(), "EXTENDS Naturals", "EXTENDS Naturals, TLC", 1), "Next ==", `Next == /\ PrintT(x) /\`, 1), "INIT Init NEXT Next", Options{Workers: 4}},
		{"liveness", strings.Replace(heavyParallelModel(), "====", "Live == <> (x = 1)\n====", 1), "INIT Init NEXT Next PROPERTY Live", Options{Workers: 4}},
		{"view", heavyParallelModel(), "INIT Init NEXT Next", Options{Workers: 4, ViewStates: true}},
		{"coverage", heavyParallelModel(), "INIT Init NEXT Next", Options{Workers: 4, Coverage: true}},
		{"checkpoint serial", heavyParallelModel(), "INIT Init NEXT Next", Options{Workers: 4, SerialExploration: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			module, err := parser.ParseModule(tc.source)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Parse(tc.cfg)
			if err != nil {
				t.Fatal(err)
			}
			m, err := prepareWithOutput(module, cfg, tc.options)
			if err != nil {
				t.Fatal(err)
			}
			if tc.options.Coverage {
				m.coverage = &coverageCollector{}
			}
			next, err := m.definition(cfg.Next)
			if err != nil {
				t.Fatal(err)
			}
			if m.parallelEligible(next) {
				t.Fatal("unsafe model was eligible")
			}
		})
	}
}

func TestParallelBatch(t *testing.T) {
	module, err := parser.ParseModule(parallelModel)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse("INIT Init NEXT Next")
	if err != nil {
		t.Fatal(err)
	}
	m, err := prepareWithOutput(module, cfg, Options{Workers: 4})
	if err != nil {
		t.Fatal(err)
	}
	next, err := m.definition("Next")
	if err != nil {
		t.Fatal(err)
	}
	one, _ := value.Integer(1)
	two, _ := value.Integer(2)
	s := newSearch(m)
	s.nodes = []stateNode{{state: evaluator.State{"x": one}}, {state: evaluator.State{"x": two}}}
	batch, ok := s.generateBatch([]*model{m.forkParallelModel(), m.forkParallelModel()}, next, 0, 2)
	if !ok {
		t.Fatal("unexpected budget exhaustion")
	}
	for i, result := range batch {
		if !result.usable || len(result.candidates) != 2 {
			t.Fatalf("batch %d: %+v", i, result)
		}
	}
}

func TestParallelBudgetConcurrent(t *testing.T) {
	var budget parallelBudget
	var group sync.WaitGroup
	for range 32 {
		group.Go(func() {
			for budget.reserve() {
			}
		})
	}
	group.Wait()
	if got := budget.count.Load(); got != parallelBatchCandidateLimit || !budget.stop.Load() {
		t.Fatalf("reserved %d, stopped %v", got, budget.stop.Load())
	}
}

func wideDuplicateModel(branches int, failure string) string {
	return "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x \\in 0..39\nNext == " +
		strings.Repeat("\\/ x' = x\n        ", branches) + failure + "\n===="
}

func TestParallelBudgetFallback(t *testing.T) {
	for _, tc := range []struct {
		name     string
		branches int
		failure  string
	}{
		{"aggregate", 150, "\\/ x' = x"},
		{"single expansion", parallelBatchCandidateLimit + 1, "\\/ x' = x"},
		{"failure order", 150, "\\/ /\\ x = 3 /\\ x' = 1 \\div 0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := wideDuplicateModel(tc.branches, tc.failure)
			module, err := parser.ParseModule(source)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Parse("INIT Init NEXT Next CHECK_DEADLOCK FALSE")
			if err != nil {
				t.Fatal(err)
			}
			m, err := prepareWithOutput(module, cfg, Options{Workers: 4})
			if err != nil {
				t.Fatal(err)
			}
			next, err := m.definition("Next")
			if err != nil {
				t.Fatal(err)
			}
			s := newSearch(m)
			zero, _ := value.Integer(0)
			for range 40 {
				s.nodes = append(s.nodes, stateNode{state: evaluator.State{"x": zero}})
			}
			p := parallelSearch{enabled: true}
			p.generate(&s, 0, next)
			if p.enabled || len(p.batch) != 0 {
				t.Fatal("overflow did not discard batch and disable speculation")
			}
			if tc.name == "single expansion" {
				batch, ok := s.generateBatch([]*model{m.forkParallelModel()}, next, 0, 1)
				if ok || batch != nil {
					t.Fatal("one oversized expansion must discard the batch")
				}
			}
			var results [2]Result
			var errs [2]error
			var outputs [2][]string
			var events [2][]SearchEvent
			for i, workers := range []int{1, 4} {
				results[i], errs[i] = CheckWithOptions(context.Background(), source, "INIT Init NEXT Next CHECK_DEADLOCK FALSE", Options{Workers: workers, OnPrint: func(line string) { outputs[i] = append(outputs[i], line) }, OnSearch: func(event SearchEvent) { events[i] = append(events[i], event) }})
			}
			wantStatus := Success
			if tc.name == "failure order" {
				wantStatus = EvaluationFailure
			}
			if results[0].Status != wantStatus || len(events[0]) == 0 {
				t.Fatalf("unexpected serial status or missing events: %+v %v", results[0], errs[0])
			}
			if !reflect.DeepEqual(results[0], results[1]) || fmt.Sprint(errs[0]) != fmt.Sprint(errs[1]) || !reflect.DeepEqual(outputs[0], outputs[1]) || !reflect.DeepEqual(events[0], events[1]) {
				t.Fatalf("serial: %+v %v %q\nparallel: %+v %v %q", results[0], errs[0], outputs[0], results[1], errs[1], outputs[1])
			}
		})
	}
}

func TestParallelBatchCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	module, err := parser.ParseModule(wideDuplicateModel(150, "\\/ x' = x"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse("INIT Init NEXT Next")
	if err != nil {
		t.Fatal(err)
	}
	m, err := prepareWithOutput(module, cfg, Options{Workers: 4})
	if err != nil {
		t.Fatal(err)
	}
	m.ctx = ctx
	next, err := m.definition("Next")
	if err != nil {
		t.Fatal(err)
	}
	s := newSearch(m)
	zero, _ := value.Integer(0)
	for range 512 {
		s.nodes = append(s.nodes, stateNode{state: evaluator.State{"x": zero}})
	}
	forks := m.parallelForks()
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.generateBatch(forks, next, 0, len(s.nodes))
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("canceled workers did not exit")
	}
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatal("context was not canceled")
	}
}

// The action performs substantial arithmetic for each of 1,000 initial states.
func heavyParallelModel() string {
	expression := strings.Repeat("(x % 13) + ", 200) + "x"
	return "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x \\in 0..999\nNext == x' = (" + expression + ") % 1000\n===="
}

func BenchmarkParallelHeavyBFS(b *testing.B) {
	source := heavyParallelModel()
	for _, w := range []int{1, 4} {
		b.Run(map[int]string{1: "workers1", 4: "workers4"}[w], func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next", Options{Workers: w})
				if err != nil || result.Status != Success {
					b.Fatalf("%+v %v", result, err)
				}
			}
		})
	}
}
