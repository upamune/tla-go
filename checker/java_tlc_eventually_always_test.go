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

	"github.com/upamune/tla-go/internal/javatmp"

	"github.com/upamune/tla-go/config"
)

// TLCGetLevelTest is an unchanged Java TLC v1.7.4 model and JUnit trace.
func TestJavaTLCEventuallyAlwaysLevel(t *testing.T) {
	source, err := os.ReadFile("testdata/java-tlc/TLCGetLevel.tla")
	if err != nil {
		t.Fatal(err)
	}
	cfgText, err := os.ReadFile("testdata/java-tlc/TLCGetLevel.cfg")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse(string(cfgText))
	if err != nil {
		t.Fatal(err)
	}
	cfg.CheckDeadlock = false
	result, err := CheckConfig(context.Background(), string(source), cfg)
	if err != nil || result.Status != LivenessViolation || result.GeneratedStates != 4 || result.DistinctStates != 4 || result.QueuedStates != 0 || result.Depth != 4 {
		t.Fatalf("got %+v, %v", result, err)
	}
	checkEventuallyAlwaysTrace(t, result.Violation, []int32{0, 1, 2, 3}, 3, "Stuttering")
	for i, step := range result.Violation.Trace {
		y, ok := step.State["y"].Int()
		if !ok || y != int32(i) {
			t.Fatalf("unexpected y at step %d: %v", i, step.State)
		}
		if i > 0 && (step.Action != "Next" || step.ActionRange.Module != "TLCGetLevel" || step.ActionRange.Start.Line != 11 || step.ActionRange.Start.Column != 9 || step.ActionRange.End.Line != 13 || step.ActionRange.End.Column != 31) {
			t.Fatalf("unexpected action at step %d: %+v", i, step)
		}
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareJavaEventuallyAlways(t, jar, "TLCGetLevel", string(source), string(cfgText), []int32{0, 1, 2, 3}, 3, "Stuttering")
	}
}

func TestJavaTLCEventuallyAlwaysGraphOrder(t *testing.T) {
	cases := []struct {
		name, next, loopAction string
		goal, loop             int
		trace                  []int32
	}{
		{"chain", `/\ x < 3 /\ x' = x + 1`, "Stuttering", 2, 3, []int32{0, 1, 2, 3}},
		{"branch", `/\ x = 0 /\ x' \in {1,2}`, "Stuttering", 9, 1, []int32{0, 2}},
		{"diamond", "\\/ /\\ x = 0 /\\ x' \\in {1,2}\n        \\/ /\\ x \\in {1,2} /\\ x' = 3", "Stuttering", 9, 2, []int32{0, 1, 3}},
		{"cycle", `x' = (x + 1) % 3`, "Next", 0, 0, []int32{0, 1, 2}},
		{"cycle-root", `x' = (x + 1) % 3`, "Stuttering", 1, 0, []int32{0}},
		{"nested-scc", "\\/ /\\ x = 0 /\\ x' \\in {2,3}\n        \\/ /\\ x = 1 /\\ x' \\in {0,3}\n        \\/ /\\ x = 2 /\\ x' \\in {0,1,2}\n        \\/ /\\ x = 3 /\\ x' \\in {1,3}", "Next", 0, 0, []int32{0, 3, 1}},
		{"indirect-branch", "\\/ /\\ x = 0 /\\ x' \\in {0,3}\n        \\/ /\\ x = 1 /\\ x' \\in {1}\n        \\/ /\\ x = 2 /\\ x' \\in {0,2}\n        \\/ /\\ x = 3 /\\ x' \\in {1}", "Stuttering", 4, 2, []int32{0, 3, 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := fmt.Sprintf("---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == %s\nP == <>[](x = %d)\n====\n", tc.next, tc.goal)
			cfg := "INIT Init\nNEXT Next\nPROPERTY P\nCHECK_DEADLOCK FALSE\n"
			result, err := Check(source, cfg)
			if err != nil || result.Status != LivenessViolation {
				t.Fatalf("got %+v, %v", result, err)
			}
			checkEventuallyAlwaysTrace(t, result.Violation, tc.trace, tc.loop, tc.loopAction)
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaEventuallyAlways(t, jar, "M", source, cfg, tc.trace, tc.loop, tc.loopAction)
			}
		})
	}
}

func TestJavaTLCEventuallyAlwaysMultipleInitial(t *testing.T) {
	const source = "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x \\in {0,2}\nNext == /\\ x < 3 /\\ x' = x+1\nP == <>[](x = 2)\n====\n"
	const cfg = "INIT Init\nNEXT Next\nPROPERTY P\nCHECK_DEADLOCK FALSE\n"
	result, err := Check(source, cfg)
	if err != nil || result.Status != LivenessViolation || result.GeneratedStates != 5 || result.DistinctStates != 4 {
		t.Fatalf("got %+v, %v", result, err)
	}
	want := []int32{2, 3}
	checkEventuallyAlwaysTrace(t, result.Violation, want, 1, "Stuttering")
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareJavaEventuallyAlways(t, jar, "M", source, cfg, want, 1, "Stuttering")
	}
}

func checkEventuallyAlwaysTrace(t *testing.T, violation *Violation, want []int32, loop int, action string) {
	t.Helper()
	if violation == nil || violation.LoopStart == nil || *violation.LoopStart != loop || violation.LoopAction != action {
		t.Fatalf("unexpected liveness violation: %+v", violation)
	}
	var got []int32
	for _, step := range violation.Trace {
		x, ok := step.State["x"].Int()
		if !ok {
			t.Fatalf("invalid state: %+v", step.State)
		}
		got = append(got, x)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got trace %v; want %v", got, want)
	}
}

func compareJavaEventuallyAlways(t *testing.T, jar, name, source, cfg string, want []int32, loop int, action string) {
	t.Helper()
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for filename, contents := range map[string]string{name + ".tla": source, name + ".cfg": cfg} {
		if err := os.WriteFile(filepath.Join(dir, filename), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-workers", "1", "-fp", "0", "-deadlock", name+".tla")
	cmd.Dir = dir
	javatmp.Apply(cmd)
	out, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if ctx.Err() != nil || !ok || exit.ExitCode() != 13 {
		t.Fatalf("Java TLC: %v, context: %v\n%s", err, ctx.Err(), out)
	}
	checkJavaEventuallyAlwaysOutput(t, string(out), want, loop, action)
	if name == "TLCGetLevel" {
		checkJavaLevelTrace(t, string(out))
	}
}

func checkJavaLevelTrace(t *testing.T, out string) {
	t.Helper()
	matches := regexp.MustCompile(`(?m)^/\\ y = ([0-9]+)$`).FindAllStringSubmatch(out, -1)
	var got []string
	for _, match := range matches {
		got = append(got, match[1])
	}
	label := "<Next line 11, col 9 to line 13, col 31 of module TLCGetLevel>"
	stats := "4 states generated, 4 distinct states found, 0 states left on queue."
	if !reflect.DeepEqual(got, []string{"0", "1", "2", "3"}) || strings.Count(out, label) != 3 || !strings.Contains(out, stats) {
		t.Fatalf("Java TLC returned unexpected y values or action labels:\n%s", out)
	}
}

func checkJavaEventuallyAlwaysOutput(t *testing.T, out string, want []int32, loop int, action string) {
	t.Helper()
	matches := regexp.MustCompile(`(?m)^(?:/\\ )?x = ([0-9]+)$`).FindAllStringSubmatch(out, -1)
	var values []string
	for _, n := range want {
		values = append(values, fmt.Sprint(n))
	}
	var got []string
	for _, match := range matches {
		got = append(got, match[1])
	}
	marker := fmt.Sprintf("Back to state %d:", loop+1)
	if action == "Stuttering" {
		marker = fmt.Sprintf("State %d: Stuttering", len(want)+1)
	}
	if !reflect.DeepEqual(got, values) || !strings.Contains(out, marker) {
		t.Fatalf("Java TLC returned an unexpected trace %v:\n%s", got, out)
	}
}
