package checker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/upamune/tla-go/config"
)

// LoopTest and LoopTestWeakFair from Java TLC use the same model with
// different configurations. TLC also checks coverage and on-disk graph sizes;
// those assertions are not yet available in tla-go.
func TestJavaTLCSystemLoop(t *testing.T) {
	source, err := os.ReadFile("testdata/java-tlc/SystemLoop.tla")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		status Status
	}{
		{"SystemLoop", LivenessViolation},
		{"SystemLoopWeakFair", Success},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfgText, err := os.ReadFile(filepath.Join("testdata", "java-tlc", tc.name+".cfg"))
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Parse(string(cfgText))
			if err != nil {
				t.Fatal(err)
			}
			cfg.CheckDeadlock = false
			got, err := CheckConfig(context.Background(), string(source), cfg)
			if err != nil || got.Status != tc.status || got.GeneratedStates != 5 || got.DistinctStates != 4 || got.Depth != 4 {
				t.Fatalf("got %+v, %v", got, err)
			}
			if tc.status == LivenessViolation {
				checkSystemLoopTrace(t, got.Violation)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				if tc.status == Success {
					compareJavaTLCModel(t, jar, "SystemLoop", source, cfgText, 5, 4, 4)
				} else {
					compareJavaTLCStutteringTrace(t, jar, "SystemLoop", source, cfgText, []string{"0", "1", "2"}, 5, 4)
				}
			}
		})
	}
}

func checkSystemLoopTrace(t *testing.T, violation *Violation) {
	t.Helper()
	if violation == nil || violation.LoopStart == nil || *violation.LoopStart != 2 || violation.LoopAction != "Stuttering" || violation.Name != "Liveness" {
		t.Fatalf("unexpected liveness violation: %+v", violation)
	}
	var states []int32
	var actions []string
	for _, step := range violation.Trace {
		x, ok := step.State["x"].Int()
		if !ok {
			t.Fatalf("invalid state: %+v", step.State)
		}
		states = append(states, x)
		actions = append(actions, step.Action)
	}
	if !reflect.DeepEqual(states, []int32{0, 1, 2}) || !reflect.DeepEqual(actions, []string{"", "One", "Two"}) {
		t.Fatalf("got states %v and actions %v", states, actions)
	}
}

func compareJavaTLCStutteringTrace(t *testing.T, jar, name string, source, cfg []byte, want []string, generated, distinct int) {
	t.Helper()
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for filename, content := range map[string][]byte{name + ".tla": source, name + ".cfg": cfg} {
		if err := os.WriteFile(filepath.Join(dir, filename), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-workers", "1", "-fp", "0", "-deadlock", name+".tla")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if ctx.Err() != nil || !ok || exit.ExitCode() != 13 {
		t.Fatalf("Java TLC: %v, context: %v\n%s", err, ctx.Err(), out)
	}
	matches := regexp.MustCompile(`(?m)^x = ([0-9]+)$`).FindAllStringSubmatch(string(out), -1)
	var states []string
	for _, match := range matches {
		states = append(states, match[1])
	}
	stutter := fmt.Sprintf("State %d: Stuttering", len(want)+1)
	stats := fmt.Sprintf("%d states generated, %d distinct states found, 0 states left on queue", generated, distinct)
	if !reflect.DeepEqual(states, want) || !strings.Contains(string(out), stutter) || !strings.Contains(string(out), stats) {
		t.Fatalf("Java TLC returned an unexpected counterexample:\n%s", out)
	}
}

func TestJavaTLCOftenTraceGraphs(t *testing.T) {
	cases := []struct {
		name                string
		next                string
		goal                int
		trace               []int32
		generated, distinct int
	}{
		{"connected", `\/ /\ x = 0 /\ x' \in {1, 2}
        \/ /\ x = 1 /\ x' = 3
        \/ /\ x = 2 /\ x' = 4
        \/ /\ x \in {3, 4} /\ x' = 0`, 5, []int32{0}, 7, 5},
		{"diamond", `\/ /\ x = 0 /\ x' \in {1, 2}
        \/ /\ x \in {1, 2} /\ x' = 3
        \/ /\ x = 3 /\ x' = 4
        \/ /\ x = 4 /\ x' = 0`, 4, []int32{0, 1, 3}, 7, 5},
		{"separate-cycles", `\/ /\ x = 0 /\ x' \in {1, 2}
        \/ /\ x = 1 /\ x' = 0
        \/ /\ x = 2 /\ x' = 3
        \/ /\ x = 3 /\ x' = 2`, 4, []int32{0, 2}, 6, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := fmt.Sprintf("---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == %s\nP == []<>(x = %d)\n====\n", tc.next, tc.goal)
			cfg := "INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE\n"
			got, err := Check(source, cfg)
			if err != nil || got.Status != LivenessViolation || got.GeneratedStates != tc.generated || got.DistinctStates != tc.distinct || got.Violation == nil || got.Violation.LoopStart == nil || *got.Violation.LoopStart != len(tc.trace)-1 || got.Violation.LoopAction != "Stuttering" {
				t.Fatalf("unexpected result: %+v, %v", got, err)
			}
			var states []int32
			var javaStates []string
			for _, step := range got.Violation.Trace {
				x, ok := step.State["x"].Int()
				if !ok {
					t.Fatalf("invalid state: %+v", step.State)
				}
				states = append(states, x)
				javaStates = append(javaStates, fmt.Sprint(x))
			}
			if !reflect.DeepEqual(states, tc.trace) {
				t.Fatalf("got %v; want %v", states, tc.trace)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaTLCStutteringTrace(t, jar, "M", []byte(source), []byte(cfg), javaStates, tc.generated, tc.distinct)
			}
		})
	}
}
