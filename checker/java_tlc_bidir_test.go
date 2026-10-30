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

// The six configurations and their status, state counts, and counterexamples
// are taken from the Java TLC BidirectionalTransitions JUnit tests. Coverage
// assertions are not yet implemented by tla-go.
func TestJavaTLCBidirectionalTransitions(t *testing.T) {
	source, err := os.ReadFile("testdata/java-tlc/BidirectionalTransitions.tla")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		suffix, property, closing  string
		status                     Status
		generated, distinct, depth int
		trace                      []int32
	}{
		{"1", "", "", Success, 13, 3, 2, nil},
		{"1Bx", "Prop1Bx", "B", LivenessViolation, 13, 3, 2, []int32{0, 2, 1}},
		{"1By", "Prop1By", "B", LivenessViolation, 13, 3, 2, []int32{0, 2, 1}},
		{"2", "", "", Success, 9, 4, 3, nil},
		{"2Cx", "Prop2Cx", "C", LivenessViolation, 9, 4, 3, []int32{0, 1, 2, 3}},
		{"2Cy", "Prop2Cy", "C", LivenessViolation, 9, 4, 3, []int32{0, 1, 2, 3}},
	} {
		t.Run(tc.suffix, func(t *testing.T) {
			cfgText, err := os.ReadFile(filepath.Join("testdata", "java-tlc", "BidirectionalTransitions"+tc.suffix+".cfg"))
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Parse(string(cfgText))
			if err != nil {
				t.Fatal(err)
			}
			cfg.CheckDeadlock = false
			got, err := CheckConfig(context.Background(), string(source), cfg)
			if err != nil || got.Status != tc.status || got.GeneratedStates != tc.generated || got.DistinctStates != tc.distinct || got.Depth != tc.depth {
				t.Fatalf("got %+v, %v", got, err)
			}
			if tc.status == LivenessViolation {
				checkBidirectionalTrace(t, got.Violation, tc.property, tc.closing, tc.trace)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				if tc.status == Success {
					compareJavaTLCModel(t, jar, "BidirectionalTransitions", source, cfgText, tc.generated, tc.distinct, tc.depth)
				} else {
					compareJavaTLCBidirectionalTrace(t, jar, source, cfgText, tc.trace, tc.closing, tc.generated, tc.distinct)
				}
			}
		})
	}
}

func checkBidirectionalTrace(t *testing.T, v *Violation, property, closing string, want []int32) {
	t.Helper()
	if v == nil || v.Name != property || v.LoopStart == nil || *v.LoopStart != 0 || v.LoopAction != closing {
		t.Fatalf("unexpected violation: %+v", v)
	}
	var states []int32
	for _, step := range v.Trace {
		x, ok := step.State["x"].Int()
		if !ok {
			t.Fatalf("invalid state: %+v", step.State)
		}
		states = append(states, x)
	}
	if !reflect.DeepEqual(states, want) {
		t.Fatalf("got trace %v, want %v", states, want)
	}
}

func compareJavaTLCBidirectionalTrace(t *testing.T, jar string, source, cfg []byte, want []int32, closing string, generated, distinct int) {
	t.Helper()
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, content := range map[string][]byte{"BidirectionalTransitions.tla": source, "BidirectionalTransitions.cfg": cfg} {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-workers", "1", "-fp", "0", "-deadlock", "BidirectionalTransitions.tla")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if ctx.Err() != nil || !ok || exit.ExitCode() != 13 {
		t.Fatalf("Java TLC: %v, context: %v\n%s", err, ctx.Err(), out)
	}
	matches := regexp.MustCompile(`(?m)^x = ([0-9]+)$`).FindAllStringSubmatch(string(out), -1)
	var states []string
	var expected []string
	for _, match := range matches {
		states = append(states, match[1])
	}
	for _, x := range want {
		expected = append(expected, fmt.Sprint(x))
	}
	stats := fmt.Sprintf("%d states generated, %d distinct states found, 0 states left on queue", generated, distinct)
	back := "Back to state 1: <" + closing + " "
	if !reflect.DeepEqual(states, expected) || !strings.Contains(string(out), back) || !strings.Contains(string(out), stats) {
		t.Fatalf("Java TLC returned an unexpected counterexample:\n%s", out)
	}
}
