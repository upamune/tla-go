package checker

import (
	"context"
	"encoding/json"
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

type javaExternalModel struct {
	dir, name, property        string
	generated, distinct, depth int
	order                      []string
	trace                      []string
}

// These models and expected traces are from Java TLC v1.7.4 fixtures.
func TestJavaTLCExternalLivenessModels(t *testing.T) {
	cases := []javaExternalModel{
		{"codeplex", "MCa", "prop_14273156623745000", 18, 11, 4, []string{"b", "x"}, []string{"FALSE,1", "TRUE,2", "FALSE,2", "TRUE,3", "FALSE,3", "TRUE,4", "FALSE,4", "TRUE,5"}},
		{"codeplex", "MC", "prop_14235897021223000", 18, 11, 4, []string{"b", "x"}, []string{"FALSE,2", "TRUE,3", "FALSE,3", "TRUE,4", "FALSE,4", "TRUE,5"}},
		{"one-bit", "OneBitMutexNoSymmetryMC", "prop_144491423293819000", 244, 127, 13, []string{"unchecked", "other", "x", "pc"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join("testdata", "java-tlc", tc.dir)
			source, err := os.ReadFile(filepath.Join(dir, tc.name+".tla"))
			if err != nil {
				t.Fatal(err)
			}
			cfgText, err := os.ReadFile(filepath.Join(dir, tc.name+".cfg"))
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Parse(string(cfgText))
			if err != nil {
				t.Fatal(err)
			}
			cfg.CheckDeadlock = false
			loader := func(_ context.Context, name string) (string, error) {
				data, err := os.ReadFile(filepath.Join(dir, name+".tla"))
				return string(data), err
			}
			got, err := CheckConfigWithLoader(context.Background(), string(source), cfg, loader)
			if err != nil || got.Status != LivenessViolation || got.GeneratedStates != tc.generated || got.DistinctStates != tc.distinct || got.Depth != tc.depth || got.Violation == nil || got.Violation.Name != tc.property {
				t.Fatalf("got %+v, %v", got, err)
			}
			for _, step := range got.Violation.Trace {
				if !reflect.DeepEqual(step.StateOrder, tc.order) {
					t.Fatalf("got variable order %v; want %v", step.StateOrder, tc.order)
				}
			}
			if tc.trace != nil {
				checkCodeplexTrace(t, got.Violation, tc.trace)
			}
			if tc.dir == "one-bit" {
				checkOneBitTrace(t, got.Violation, filepath.Join(dir, tc.name+".trace.json"))
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				out := runJavaExternalModel(t, jar, dir, tc.name)
				checkJavaExternalOutput(t, out, tc)
				checkJavaExternalActionRanges(t, out, got.Violation.Trace)
				if tc.dir == "one-bit" {
					checkJavaOneBitTrace(t, out, filepath.Join(dir, tc.name+".trace.json"))
				}
				if got.Violation.LoopAction != "Stuttering" {
					closing := TraceStep{Action: got.Violation.LoopAction, ActionRange: got.Violation.LoopActionRange}
					checkJavaExternalActionRanges(t, out, []TraceStep{closing})
				}
			}
		})
	}
}

type oneBitTraceStep struct {
	Action string            `json:"action"`
	State  map[string]string `json:"state"`
}

func checkOneBitTrace(t *testing.T, violation *Violation, path string) {
	t.Helper()
	want := readOneBitTrace(t, path)
	if violation == nil || violation.LoopStart == nil || *violation.LoopStart != 8 || violation.LoopAction != "e2" || len(violation.Trace) != len(want) {
		t.Fatalf("unexpected OneBitMutex counterexample: %+v", violation)
	}
	for i, step := range violation.Trace {
		checkOneBitTraceStep(t, i, step, want[i])
	}
}

func readOneBitTrace(t *testing.T, path string) []oneBitTraceStep {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var want []oneBitTraceStep
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	return want
}

func checkJavaOneBitTrace(t *testing.T, out, path string) {
	t.Helper()
	want := readOneBitTrace(t, path)
	values := regexp.MustCompile(`(?m)^/\\ (unchecked|other|x|pc) = (.*)$`).FindAllStringSubmatch(out, -1)
	labels := regexp.MustCompile(`(?m)^State [0-9]+: <([^ >]+)`).FindAllStringSubmatch(out, -1)
	if len(values) != len(want)*4 || len(labels) != len(want) {
		t.Fatalf("unexpected Java TLC trace:\n%s", out)
	}
	for i, step := range want {
		if i > 0 && labels[i][1] != step.Action {
			t.Fatalf("Java state %d: got action %q, want %q", i+1, labels[i][1], step.Action)
		}
		for _, match := range values[i*4 : (i+1)*4] {
			if match[2] != step.State[match[1]] {
				t.Fatalf("Java state %d: unexpected %s = %s", i+1, match[1], match[2])
			}
		}
	}
}

func checkOneBitTraceStep(t *testing.T, index int, step TraceStep, want oneBitTraceStep) {
	t.Helper()
	if step.Action != want.Action {
		t.Fatalf("state %d: got action %q, want %q", index+1, step.Action, want.Action)
	}
	for name, expected := range want.State {
		if got := step.State[name].String(); got != expected {
			t.Fatalf("state %d variable %s: got %s, want %s", index+1, name, got, expected)
		}
	}
}

func checkJavaExternalActionRanges(t *testing.T, out string, trace []TraceStep) {
	t.Helper()
	pattern := regexp.MustCompile(`(?m)^State [0-9]+: <([A-Za-z_][A-Za-z_0-9]*) (line [^>]+)>$`)
	labels := make(map[string]string)
	for _, match := range pattern.FindAllStringSubmatch(out, -1) {
		labels[match[1]] = match[2]
	}
	checked := 0
	for _, step := range trace {
		want, ok := labels[step.Action]
		if !ok {
			continue
		}
		r := step.ActionRange
		got := fmt.Sprintf("line %d, col %d to line %d, col %d of module %s", r.Start.Line, r.Start.Column, r.End.Line, r.End.Column, r.Module)
		if got != want {
			t.Fatalf("action %s: got %q; Java TLC returned %q", step.Action, got, want)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no action locations could be compared")
	}
}

func checkCodeplexTrace(t *testing.T, v *Violation, want []string) {
	t.Helper()
	var got []string
	for _, step := range v.Trace {
		got = append(got, step.State["b"].String()+","+step.State["x"].String())
	}
	if !reflect.DeepEqual(got, want) || v.LoopStart == nil || *v.LoopStart != len(want)-1 || v.LoopAction != "Stuttering" {
		t.Fatalf("got trace %v and violation %+v; want %v with final stutter", got, v, want)
	}
}

func runJavaExternalModel(t *testing.T, jar, sourceDir, name string) string {
	t.Helper()
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".tla" && entry.Name() != name+".cfg" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(sourceDir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, entry.Name()), data, 0o600); err != nil {
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
	return string(out)
}

func checkJavaExternalOutput(t *testing.T, out string, tc javaExternalModel) {
	t.Helper()
	matches := regexp.MustCompile(`(?m)^/\\ ([A-Za-z_][A-Za-z_0-9]*) = `).FindAllStringSubmatch(out, -1)
	if len(matches) < len(tc.order) {
		t.Fatalf("missing Java TLC state variables:\n%s", out)
	}
	var order []string
	for _, match := range matches[:len(tc.order)] {
		order = append(order, match[1])
	}
	stats := fmt.Sprintf("%d states generated, %d distinct states found, 0 states left on queue", tc.generated, tc.distinct)
	if !reflect.DeepEqual(order, tc.order) || !strings.Contains(out, stats) {
		t.Fatalf("unexpected Java TLC order %v or statistics:\n%s", order, out)
	}
	if tc.trace == nil {
		if !strings.Contains(out, "Back to state 9:") {
			t.Fatalf("unexpected Java TLC cycle:\n%s", out)
		}
		return
	}
	checkJavaCodeplexTrace(t, out, tc.trace)
}

func checkJavaCodeplexTrace(t *testing.T, out string, want []string) {
	t.Helper()
	matches := regexp.MustCompile(`(?m)^/\\ b = (TRUE|FALSE)\n/\\ x = ([0-9]+)$`).FindAllStringSubmatch(out, -1)
	var trace []string
	for _, match := range matches {
		trace = append(trace, match[1]+","+match[2])
	}
	marker := fmt.Sprintf("State %d: Stuttering", len(want)+1)
	if !reflect.DeepEqual(trace, want) || !strings.Contains(out, marker) {
		t.Fatalf("unexpected Java TLC trace %v:\n%s", trace, out)
	}
}
