package checker

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func parallelConstantModel() string {
	expression := strings.Repeat("(x % 13) + ", 40) + "x"
	return "---- MODULE M ----\nEXTENDS Naturals, Sequences, FiniteSets\nCONSTANT N, Flag\nVARIABLE x\nInit == x \\in 0..63\nNext == /\\ Flag /\\ x' = (" + expression + ") % N\n====\n"
}

const parallelConstantConfig = "INIT Init NEXT Next CONSTANTS N = 97 Flag = TRUE CHECK_DEADLOCK FALSE"

func parallelConstantBenchmarkModel() string {
	expression := strings.Repeat("(x % 13) + ", 200) + "x"
	return "---- MODULE M ----\nEXTENDS Naturals, Sequences, FiniteSets\nCONSTANT N, Flag\nVARIABLE x\nInit == x \\in 0..999\nNext == /\\ Flag /\\ x' = (" + expression + ") % N\n====\n"
}

func TestParallelPrimitiveConstantsEligibility(t *testing.T) {
	old := runtime.GOMAXPROCS(4)
	defer runtime.GOMAXPROCS(old)
	module, err := parser.ParseModule(parallelConstantModel())
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse(parallelConstantConfig)
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
		t.Fatal("pure scalar constants should be eligible")
	}
	search := newSearch(m)
	for i := range 64 {
		v, _ := value.Integer(int64(i))
		search.nodes = append(search.nodes, stateNode{state: evaluator.State{"x": v}})
	}
	batch, ok := search.generateBatch(m.parallelForks(), next, 0, len(search.nodes))
	if !ok {
		t.Fatal("parallel batch exceeded the candidate budget")
	}
	for i, expansion := range batch {
		if !expansion.usable || len(expansion.candidates) != 1 {
			t.Fatalf("state %d was not expanded in parallel: %+v", i, expansion)
		}
	}
}

func TestParallelPrimitiveConstantsDifferential(t *testing.T) {
	old := runtime.GOMAXPROCS(4)
	defer runtime.GOMAXPROCS(old)
	for _, tc := range []struct {
		name, source, cfg string
		want              Status
	}{
		{"success", parallelConstantModel(), parallelConstantConfig, Success},
		{"evaluation error", strings.Replace(parallelConstantModel(), "/\\ Flag", "/\\ (1 \\div (x - 7) = 1) /\\ Flag", 1), parallelConstantConfig, EvaluationFailure},
		{"invariant", strings.Replace(parallelConstantModel(), "====", "Inv == x < 90\n====", 1), parallelConstantConfig + " INVARIANT Inv", InvariantViolation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var results [2]Result
			var events [2][]SearchEvent
			var outputs [2][]string
			var errs [2]error
			for i, workers := range []int{1, 4} {
				results[i], errs[i] = CheckWithOptions(context.Background(), tc.source, tc.cfg, Options{Workers: workers, Fingerprint: 0, OnSearch: func(e SearchEvent) { events[i] = append(events[i], e) }, OnPrint: func(s string) { outputs[i] = append(outputs[i], s) }})
			}
			if errs[0] != nil || results[0].Status != tc.want {
				t.Fatalf("unexpected serial outcome: %+v, %v", results[0], errs[0])
			}
			if !reflect.DeepEqual(results[0], results[1]) || !reflect.DeepEqual(events[0], events[1]) || !reflect.DeepEqual(outputs[0], outputs[1]) || (errs[0] == nil) != (errs[1] == nil) || errs[0] != nil && errs[0].Error() != errs[1].Error() {
				t.Fatalf("serial: %+v %v %q\nparallel: %+v %v %q", results[0], errs[0], outputs[0], results[1], errs[1], outputs[1])
			}
		})
	}
}

func TestParallelPrimitiveConstantsUnsafeFallback(t *testing.T) {
	old := runtime.GOMAXPROCS(4)
	defer runtime.GOMAXPROCS(old)
	for _, tc := range []struct{ name, source, cfg string }{
		{"set constant", parallelConstantModel(), "INIT Init NEXT Next CONSTANTS N = {1, 2} Flag = TRUE"},
		{"model value constant", parallelConstantModel(), "INIT Init NEXT Next CONSTANTS N = modelValue Flag = TRUE"},
		{"operator substitution", strings.Replace(parallelConstantModel(), "Init ==", "Replacement == 97\nInit ==", 1), "INIT Init NEXT Next CONSTANTS N <- Replacement Flag = TRUE"},
		{"builtin substitution", strings.Replace(parallelConstantModel(), "Init ==", "Replacement(a, b) == a\nInit ==", 1), parallelConstantConfig + " CONSTANT + <- Replacement"},
		{"effectful action", strings.Replace(strings.Replace(parallelConstantModel(), "FiniteSets", "FiniteSets, TLC", 1), "/\\ Flag", "/\\ PrintT(x) /\\ Flag", 1), parallelConstantConfig},
		{"tlc imported", strings.Replace(parallelConstantModel(), "FiniteSets", "FiniteSets, TLC", 1), parallelConstantConfig},
	} {
		t.Run(tc.name, func(t *testing.T) {
			module, err := parser.ParseModule(tc.source)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Parse(tc.cfg)
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
			if m.parallelEligible(next) {
				t.Fatal("unsafe model was eligible for speculative evaluation")
			}
		})
	}
}

func BenchmarkParallelPrimitiveConstants(b *testing.B) {
	source := parallelConstantBenchmarkModel()
	configuration := "INIT Init NEXT Next CONSTANTS N = 1009 Flag = TRUE CHECK_DEADLOCK FALSE"
	for _, workers := range []int{1, 4} {
		b.Run(map[int]string{1: "workers1", 4: "workers4"}[workers], func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				result, err := CheckWithOptions(context.Background(), source, configuration, Options{Workers: workers})
				if err != nil || result.Status != Success {
					b.Fatalf("%+v %v", result, err)
				}
			}
		})
	}
}

// This original finite model is compared against the pinned Java TLC 1.7.4.
func TestJavaTLCParallelPrimitiveConstants(t *testing.T) {
	old := runtime.GOMAXPROCS(4)
	defer runtime.GOMAXPROCS(old)
	source := parallelConstantModel()
	result, err := CheckWithOptions(context.Background(), source, parallelConstantConfig, Options{Workers: 4})
	if err != nil || result.Status != Success {
		t.Fatalf("result %+v, error %v", result, err)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		data, err := os.ReadFile(jar)
		if err != nil || fmt.Sprintf("%x", sha256.Sum256(data)) != "936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88" {
			t.Fatalf("TLC_JAR must be the pinned 1.7.4 jar: %v", err)
		}
		compareJavaTLCModel(t, jar, "M", []byte(source), []byte(parallelConstantConfig), result.GeneratedStates, result.DistinctStates, result.Depth)
		jar, err = filepath.Abs(jar)
		if err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		for name, contents := range map[string]string{"M.tla": source, "M.cfg": parallelConstantConfig} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		out, code := runNameResolutionJava(t, dir, "-XX:+UseParallelGC", "-jar", jar, "-tool", "-workers", "4", "-fp", "0", "M.tla")
		stats := fmt.Sprintf("%d states generated, %d distinct states found, 0 states left on queue.", result.GeneratedStates, result.DistinctStates)
		if code != 0 || !strings.Contains(out, stats) {
			t.Fatalf("Java 4 workers exit %d, expected %q: %s", code, stats, out)
		}
	}
}

func TestJavaTLCParallelPrimitiveConstantFailures(t *testing.T) {
	old := runtime.GOMAXPROCS(4)
	defer runtime.GOMAXPROCS(old)
	jar := os.Getenv("TLC_JAR")
	if jar != "" {
		data, err := os.ReadFile(jar)
		if err != nil || fmt.Sprintf("%x", sha256.Sum256(data)) != "936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88" {
			t.Fatalf("TLC_JAR must be the pinned 1.7.4 jar: %v", err)
		}
		jar, err = filepath.Abs(jar)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name, source, cfg           string
		want                        Status
		exit, code                  int
		generated, distinct, queued int
		states                      []int32
	}{
		{"invariant", strings.Replace(parallelConstantModel(), "====", "Inv == x < 90\n====", 1), parallelConstantConfig + " INVARIANT Inv", InvariantViolation, 12, 2110, 72, 67, 58, []int32{7, 93}},
		{"evaluation", strings.Replace(parallelConstantModel(), "/\\ Flag", "/\\ (1 \\div (x - 7) = 1) /\\ Flag", 1), parallelConstantConfig, EvaluationFailure, 255, 2179, 64, 64, 56, []int32{7}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := CheckWithOptions(context.Background(), tc.source, tc.cfg, Options{Workers: 4, Fingerprint: 0})
			if err != nil || result.Status != tc.want {
				t.Fatalf("result %+v, error %v", result, err)
			}
			if result.GeneratedStates != tc.generated || result.DistinctStates != tc.distinct || result.QueuedStates != tc.queued {
				t.Fatalf("unexpected statistics: %+v", result)
			}
			if tc.want == EvaluationFailure && (result.Failure == nil || result.Failure.ExitCode != tc.exit || result.Failure.ToolCode != tc.code) {
				t.Fatalf("unexpected evaluation failure: %+v", result.Failure)
			}
			var trace []TraceStep
			if result.Violation != nil {
				trace = result.Violation.Trace
			} else if result.Failure != nil {
				trace = result.Failure.Trace
			}
			if len(trace) != len(tc.states) {
				t.Fatalf("trace %+v, want %v", trace, tc.states)
			}
			for i, want := range tc.states {
				got, ok := trace[i].State["x"].Int()
				if !ok || got != want {
					t.Fatalf("state %d: got %d, want %d", i, got, want)
				}
			}
			if jar == "" {
				return
			}
			dir := t.TempDir()
			for name, contents := range map[string]string{"M.tla": tc.source, "M.cfg": tc.cfg} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			out, code := runNameResolutionJava(t, dir, "-XX:+UseParallelGC", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "M.tla")
			if code != tc.exit || !strings.Contains(out, fmt.Sprintf("@!@!@STARTMSG %d:", tc.code)) {
				t.Fatalf("Java exit %d: %s", code, out)
			}
			statistics := fmt.Sprintf("%d states generated, %d distinct states found, %d states left on queue.", tc.generated, tc.distinct, tc.queued)
			if !strings.Contains(out, statistics) {
				t.Fatalf("Java missing statistics %q: %s", statistics, out)
			}
			remaining := out
			for _, state := range tc.states {
				marker := fmt.Sprintf("x = %d\n", state)
				index := strings.Index(remaining, marker)
				if index < 0 {
					t.Fatalf("Java missing state %d: %s", state, out)
				}
				remaining = remaining[index+len(marker):]
			}
		})
	}
}
