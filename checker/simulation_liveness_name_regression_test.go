package checker

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/upamune/tla-go/internal/javatmp"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

// Replay can find no violation on the recovered lasso, or fail. Preserve the
// existing fallback for multiple properties in both cases.
func TestSimulationLivenessNameFallback(t *testing.T) {
	module, err := parser.ParseModule("---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == x' = x\nP == <>(x = 0)\nR == []<>(x = 0)\n====")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse("INIT Init NEXT Next PROPERTIES P R")
	if err != nil {
		t.Fatal(err)
	}
	m, err := prepare(module, cfg)
	if err != nil {
		t.Fatal(err)
	}
	x, err := value.Integer(0)
	if err != nil {
		t.Fatal(err)
	}
	loop := 0
	v := &Violation{Trace: []TraceStep{{State: evaluator.State{"x": x}}}, LoopStart: &loop}
	r := &simulationRun{model: m, search: search{model: m}}
	for _, state := range []evaluator.State{{"x": x}, {}} {
		v.Trace[0].State = state
		result := r.liveViolationResult(v)
		if result == nil || !reflect.DeepEqual(v.Properties, []string{"P", "R"}) {
			t.Fatalf("result %+v, violation %+v", result, v)
		}
	}
}

// Java does not reevaluate predicates while reporting a simulation violation.
func TestSimulationLivenessViolationEffectsOracle(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("set TLC_JAR to pinned v1.7.4 jar")
	}
	data, err := os.ReadFile(jar)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != "936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88" {
		t.Fatal("TLC_JAR is not the pinned v1.7.4 jar")
	}
	jar, err = filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, definitions, properties string }{
		{"single", "P == []<>Q", "PROPERTY P"},
		{"single tableau", "P == <>[]<>Q", "PROPERTY P"},
		{"multiple", "P == []<>Q\nR == <> (x = 0)", "PROPERTIES P R"},
		{"multiple tableau", "P == <>[]<>Q\nR == <> (x = 0)", "PROPERTIES P R"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 2\nQ == /\\ PrintT(<<\"Q\", x>>) /\\ x = 0\n" + tc.definitions + "\n====\n"
			cfg := "INIT Init\nNEXT Next\n" + tc.properties + "\nCHECK_DEADLOCK FALSE\n"
			var printed []string
			result, err := CheckWithOptions(context.Background(), source, cfg, Options{Seed: 1, Simulation: &SimulationOptions{Traces: 1, Depth: 3, OmitLassoPropertyNames: true}, OnPrint: func(s string) { printed = append(printed, s) }})
			if err != nil || result.Status != LivenessViolation || result.GeneratedStates != 4 || result.Violation == nil || result.Violation.Properties != nil || result.Violation.Name != "" {
				t.Fatalf("Go: %+v, %v", result, err)
			}
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte(cfg), 0o600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command("java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-jar", jar, "-workers", "1", "-fp", "0", "-simulate", "num=1", "-seed", "1", "-depth", "3", "M.tla")
			cmd.Dir = dir
			javatmp.Apply(cmd)
			output, runErr := cmd.CombinedOutput()
			if exit, ok := runErr.(*exec.ExitError); !ok || exit.ExitCode() != 13 {
				t.Fatalf("Java: %v\n%s", runErr, output)
			}
			var java []string
			for line := range strings.SplitSeq(string(output), "\n") {
				if strings.HasPrefix(line, "<<\"Q\"") {
					java = append(java, line)
				}
			}
			if !strings.Contains(string(output), "Progress: 4 states checked.") || !strings.Contains(string(output), "Temporal properties were violated") {
				t.Fatalf("Java result:\n%s", output)
			}
			if !reflect.DeepEqual(printed, java) {
				t.Fatalf("Go prints %q; Java prints %q", printed, java)
			}
		})
	}
}

func TestSimulationOmitLassoNamesAPI(t *testing.T) {
	const source = "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x=0\nNext == x'=(x+1)%2\nP == []<>(x=2)\n===="
	for _, omit := range []bool{false, true} {
		options := SimulationOptions{Traces: 1, Depth: 3, OmitLassoPropertyNames: omit}
		result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next PROPERTY P", Options{Seed: 1, Simulation: &options})
		if err != nil || result.Status != LivenessViolation || result.Violation == nil || result.Violation.LoopStart == nil {
			t.Fatalf("omit %v: %+v %v", omit, result, err)
		}
		if omit && (result.Violation.Name != "" || result.Violation.Properties != nil) {
			t.Fatalf("omit: %+v", result.Violation)
		}
		if !omit && (result.Violation.Name != "P" || !reflect.DeepEqual(result.Violation.Properties, []string{"P"})) {
			t.Fatalf("default: %+v", result.Violation)
		}
	}
}

func TestSimulationOmitNamesCallbacks(t *testing.T) {
	const source = "---- MODULE M ----\nVARIABLE x\nInit == x=0\nNext == x'=1\nInv == x=0\n===="
	var names []string
	opts := SimulationOptions{Traces: 2, Depth: 2, Continue: true, OmitLassoPropertyNames: true, OnViolation: func(r Result) { names = append(names, r.Violation.Name) }}
	result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next INVARIANT Inv", Options{Simulation: &opts})
	if err != nil || result.Status != ContinuedViolation || !reflect.DeepEqual(names, []string{"Inv", "Inv"}) {
		t.Fatalf("result %+v names %v err %v", result, names, err)
	}
	var count int
	opts.OnTrace = func(_ int, trace []TraceStep) error {
		if len(trace) == 0 {
			t.Error("empty trace")
		}
		count++
		return nil
	}
	opts.Continue = false
	result, err = CheckWithOptions(context.Background(), source, "INIT Init NEXT Next", Options{Simulation: &opts})
	if err != nil || result.Status != Success || count != 2 {
		t.Fatalf("result %+v count %d err %v", result, count, err)
	}
}
