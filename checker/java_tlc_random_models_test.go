package checker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

type javaRandomModel struct {
	name                              string
	seed                              int64
	generated, distinct, queue, depth int
	x, y                              []int32
	line, endLine, endColumn          int
}

// The unchanged models and expectations come from Java TLC 1.7.4's
// RandomSubsetA/BTest, RandomSubsetNextTest, RandomElementTest and
// RandomElementXandYTest. Coverage is not yet reported.
func TestJavaTLCRandomOriginalModels(t *testing.T) {
	for _, tc := range []javaRandomModel{
		{"RandomSubset", 15041980, 2003, 2003, 2001, 2, []int32{47567, 47567}, []int32{100000003, 100000003}, 10, 11, 21},
		{"RandomSubset", 918347981374, 2003, 2003, 2001, 2, []int32{66058, 66058}, []int32{100000003, 100000003}, 10, 11, 21},
		{"RandomSubsetNext", 15041980, 67321, 7732, 999, 11, []int32{43, 2, 95, 40, 6, 168, 225, 93, 42, 8, 30}, []int32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, 9, 10, 21},
		{"RandomElement", 8006803340504660123, 932, 855, 388, 11, []int32{843, 920, 483, 173, 590, 104, 785, 463, 443, 151, 767}, []int32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, 9, 11, 21},
		{"RandomElementXandY", 8006642976694192746, 3, 3, 0, 3, []int32{0, 1, 0}, []int32{0, 1, 1}, 9, 10, 35},
	} {
		t.Run(fmt.Sprintf("%s/%d", tc.name, tc.seed), func(t *testing.T) {
			dir := filepath.Join("testdata", "java-tlc", "random-subset")
			source, err := os.ReadFile(filepath.Join(dir, tc.name+".tla"))
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := os.ReadFile(filepath.Join(dir, tc.name+".cfg"))
			if err != nil {
				t.Fatal(err)
			}
			got, err := CheckWithOptions(context.Background(), string(source), string(cfg), Options{Seed: tc.seed})
			if err != nil || got.Status != InvariantViolation || got.GeneratedStates != tc.generated || got.DistinctStates != tc.distinct || got.QueuedStates != tc.queue || got.Depth != tc.depth {
				t.Fatalf("got %+v, %v", got, err)
			}
			checkJavaRandomModelTrace(t, got.Violation, tc)
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaRandomModel(t, jar, source, cfg, tc)
			}
		})
	}
}

// RandomSubsetSetOfFcnsTest checks sampling a large function set without
// enumerating it; its original JUnit coverage assertions are not available.
func TestJavaTLCRandomSubsetOriginalFunctionSet(t *testing.T) {
	const name = "RandomSubsetSetOfFcns"
	dir := filepath.Join("testdata", "java-tlc", "random-subset")
	source, err := os.ReadFile(filepath.Join(dir, name+".tla"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := os.ReadFile(filepath.Join(dir, name+".cfg"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := CheckWithOptions(context.Background(), string(source), string(cfg), Options{Seed: 1})
	if err != nil || got.Status != Success || got.GeneratedStates != 2000 || got.DistinctStates != 1000 || got.QueuedStates != 0 || got.Depth != 1 {
		t.Fatalf("got %+v, %v", got, err)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareJavaTLCModel(t, jar, name, source, cfg, 2000, 1000, 1)
	}
}

func checkJavaRandomModelTrace(t *testing.T, v *Violation, tc javaRandomModel) {
	t.Helper()
	if v == nil || v.Name != "Inv" || len(v.Trace) != len(tc.x) {
		t.Fatalf("unexpected violation: %+v", v)
	}
	for i, step := range v.Trace {
		x, okX := step.State["x"].Int()
		y, okY := step.State["y"].Int()
		if !okX || !okY || x != tc.x[i] || y != tc.y[i] {
			t.Fatalf("state %d: got x=%v y=%v; want %d %d", i+1, step.State["x"], step.State["y"], tc.x[i], tc.y[i])
		}
		if tc.name == "RandomSubset" {
			z, ok := step.State["z"].Bool()
			if !ok || z != (i == 0) {
				t.Fatalf("state %d: got z=%v", i+1, step.State["z"])
			}
		}
		if i > 0 && (step.Action != "Next" || step.ActionRange.Module != tc.name || step.ActionRange.Start.Line != tc.line || step.ActionRange.Start.Column != 9 || step.ActionRange.End.Line != tc.endLine || step.ActionRange.End.Column != tc.endColumn) {
			t.Fatalf("state %d: unexpected action %+v", i+1, step)
		}
	}
}

func compareJavaRandomModel(t *testing.T, jar string, source, cfg []byte, tc javaRandomModel) {
	t.Helper()
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, data := range map[string][]byte{tc.name + ".tla": source, tc.name + ".cfg": cfg} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx512m", "-XX:+UseParallelGC", "-jar", jar, "-workers", "1", "-seed", strconv.FormatInt(tc.seed, 10), "-fp", "0", tc.name+".tla")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if ctx.Err() != nil || !ok || exit.ExitCode() != 12 {
		t.Fatalf("Java TLC: %v, context: %v\n%s", err, ctx.Err(), out)
	}
	checkJavaRandomModelOutput(t, string(out), tc)
}

func checkJavaRandomModelOutput(t *testing.T, out string, tc javaRandomModel) {
	t.Helper()
	stats := fmt.Sprintf("%d states generated, %d distinct states found, %d states left on queue.", tc.generated, tc.distinct, tc.queue)
	depth := fmt.Sprintf("The depth of the complete state graph search is %d.", tc.depth)
	label := fmt.Sprintf("<Next line %d, col 9 to line %d, col %d of module %s>", tc.line, tc.endLine, tc.endColumn, tc.name)
	if !strings.Contains(out, stats) || !strings.Contains(out, depth) || strings.Count(out, label) != len(tc.x)-1 {
		t.Fatalf("unexpected Java TLC statistics or labels:\n%s", out)
	}
	for name, expected := range map[string][]int32{"x": tc.x, "y": tc.y} {
		matches := regexp.MustCompile(`(?m)^/\\ `+name+` = ([0-9]+)$`).FindAllStringSubmatch(out, -1)
		var values []int32
		for _, match := range matches {
			value, err := strconv.ParseInt(match[1], 10, 32)
			if err != nil {
				t.Fatal(err)
			}
			values = append(values, int32(value))
		}
		if !reflect.DeepEqual(values, expected) {
			t.Fatalf("Java TLC %s values: got %v, want %v", name, values, expected)
		}
	}
	if tc.name == "RandomSubset" && (!strings.Contains(out, "/\\ z = TRUE") || !strings.Contains(out, "/\\ z = FALSE")) {
		t.Fatalf("Java TLC did not print both z values:\n%s", out)
	}
}
