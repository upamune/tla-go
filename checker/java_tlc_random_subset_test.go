package checker

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/upamune/tla-go/internal/javatmp"
)

const javaRandomSubsetSource = `---- MODULE M ----
EXTENDS Naturals, TLC, Randomization
VARIABLE x
Init == x = 0
Next == UNCHANGED x
ASSUME PrintT(RandomSubset(3, 1..20))
ASSUME PrintT(RandomSubset(5, 0..2147483646))
ASSUME PrintT(RandomSubset(0, 1..20))
ASSUME PrintT(RandomSubset(3, 1..20))
ASSUME PrintT(RandomSubset(4, {0,1,2,3,4,5,6,7,8,9}))
ASSUME PrintT(RandomSubset(4, [1..3 -> {0,1}]))
ASSUME PrintT(RandomSubset(4, [b: 1..3, a: 1..2]))
====
`

const javaRandomSubsetConfig = "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n"

func TestJavaTLCRandomSubsetSequence(t *testing.T) {
	for _, tc := range []struct {
		seed int64
		want []string
	}{
		{1, []string{"{6, 8, 17}", "{215764588}", "{}", "{5, 14, 16}", "{4, 5, 6, 7}", "{<<0, 0, 0>>, <<1, 0, 1>>, <<1, 1, 0>>, <<1, 1, 1>>}", "{[b |-> 1, a |-> 1], [b |-> 1, a |-> 2], [b |-> 2, a |-> 1], [b |-> 3, a |-> 2]}"}},
		{0, []string{"{1, 3, 12}", "{1785505948}", "{}", "{8, 10, 19}", "{5, 6, 7, 8}", "{<<0, 0, 0>>, <<0, 0, 1>>, <<0, 1, 0>>, <<1, 1, 1>>}", "{[b |-> 2, a |-> 1], [b |-> 2, a |-> 2], [b |-> 3, a |-> 1], [b |-> 3, a |-> 2]}"}},
		{-42, []string{"{6, 8, 17}", "{878838546}", "{}", "{8, 17, 19}", "{6, 7, 8, 9}", "{<<0, 1, 1>>, <<1, 0, 0>>, <<1, 0, 1>>, <<1, 1, 0>>}", "{[b |-> 1, a |-> 2], [b |-> 2, a |-> 1], [b |-> 2, a |-> 2], [b |-> 3, a |-> 1]}"}},
		{8006642976694192746, []string{"{1, 3, 12}", "{721414883}", "{}", "{8, 17, 19}", "{1, 2, 3, 4}", "{<<0, 1, 1>>, <<1, 0, 0>>, <<1, 0, 1>>, <<1, 1, 0>>}", "{[b |-> 1, a |-> 2], [b |-> 2, a |-> 1], [b |-> 2, a |-> 2], [b |-> 3, a |-> 1]}"}},
	} {
		t.Run(strconv.FormatInt(tc.seed, 10), func(t *testing.T) {
			var printed []string
			got, err := CheckWithOptions(context.Background(), javaRandomSubsetSource, javaRandomSubsetConfig, Options{Seed: tc.seed, OnPrint: func(line string) { printed = append(printed, line) }})
			if err != nil || got.Status != Success || !reflect.DeepEqual(printed, tc.want) {
				t.Fatalf("got %+v, %v, output %q; want %q", got, err, printed, tc.want)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaRandomSubsetSequence(t, jar, javaRandomSubsetSource, javaRandomSubsetConfig, tc.seed, tc.want)
			}
		})
	}
}

func TestJavaTLCRandomSubsetModernSequence(t *testing.T) {
	want := []string{"{7, 13, 15}", "{17257894, 21101017, 881375518, 1736525855, 1737806896}", "{}", "{3, 15, 17}", "{2, 3, 5, 6}", "{<<0, 0, 0>>, <<0, 0, 1>>, <<0, 1, 0>>, <<1, 1, 0>>}", "{[b |-> 1, a |-> 2], [b |-> 2, a |-> 2], [b |-> 3, a |-> 1], [b |-> 3, a |-> 2]}"}
	var printed []string
	got, err := CheckWithOptions(context.Background(), javaRandomSubsetSource, javaRandomSubsetConfig, Options{Seed: 1, ModernRandomSubsets: true, OnPrint: func(line string) { printed = append(printed, line) }})
	if err != nil || got.Status != Success || !reflect.DeepEqual(printed, want) {
		t.Fatalf("got %+v, %v, output %q; want %q", got, err, printed, want)
	}
	if jar := os.Getenv("TLC_MODERN_JAR"); jar != "" {
		compareJavaRandomSubsetSequence(t, jar, javaRandomSubsetSource, javaRandomSubsetConfig, 1, want)
	}
}

func TestJavaTLCRandomSubsetIntervalEdges(t *testing.T) {
	const source = `---- MODULE M ----
EXTENDS Integers, TLC, Randomization
VARIABLE x
Init == x = 0
Next == UNCHANGED x
ASSUME PrintT(RandomSubset(0, 1..0))
ASSUME PrintT(RandomSubset(0, 1..1))
ASSUME PrintT(RandomSubset(4, 1..2))
ASSUME PrintT(RandomSubset(5, 1..190))
ASSUME PrintT(RandomSubset(5, 1..191))
ASSUME PrintT(RandomSubset(5, 1..192))
ASSUME PrintT(RandomSubset(5, 0..2147483645))
ASSUME PrintT(RandomSubset(5, 0..2147483646))
ASSUME PrintT(RandomSubset(1, (-20)..(-1)))
====
`
	want := []string{"{}", "{}", "{1, 2}", "{18, 19, 20, 21, 22}", "{9, 32, 55, 78, 177}", "{60, 63, 125, 187, 190}", "{77814904, 77814905, 77814906, 77814907, 77814908}", "{714504434}", "{-14}"}
	var printed []string
	got, err := CheckWithOptions(context.Background(), source, javaRandomSubsetConfig, Options{Seed: 1, OnPrint: func(line string) { printed = append(printed, line) }})
	if err != nil || got.Status != Success || !reflect.DeepEqual(printed, want) {
		t.Fatalf("got %+v, %v, output %q; want %q", got, err, printed, want)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareJavaRandomSubsetSequence(t, jar, source, javaRandomSubsetConfig, 1, want)
	}
}

func TestJavaTLCRandomSubsetWorker(t *testing.T) {
	const source = `---- MODULE Worker ----
EXTENDS Naturals, TLC, Randomization
VARIABLES x, step
Init == /\ x = {} /\ step = 0
Next == /\ step < 2 /\ x' = RandomSubset(3, 1..20) /\ step' = step + 1
Inv == step < 2
====
`
	const cfg = "INIT Init\nNEXT Next\nINVARIANT Inv\nCHECK_DEADLOCK FALSE\n"
	want := []string{"{}", "{1, 3, 12}", "{1, 10, 19}"}
	got, err := CheckWithOptions(context.Background(), source, cfg, Options{Seed: 1})
	if err != nil || got.Status != InvariantViolation || got.Violation == nil {
		t.Fatalf("got %+v, %v", got, err)
	}
	var states []string
	for _, step := range got.Violation.Trace {
		states = append(states, step.State["x"].String())
	}
	if !reflect.DeepEqual(states, want) {
		t.Fatalf("got trace %q; want %q", states, want)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareJavaRandomSubsetWorker(t, jar, source, cfg, want)
	}
}

func compareJavaRandomSubsetWorker(t *testing.T, jar, source, cfg string, want []string) {
	t.Helper()
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, content := range map[string]string{"Worker.tla": source, "Worker.cfg": cfg} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-workers", "1", "-seed", "1", "-fp", "0", "-deadlock", "Worker.tla")
	cmd.Dir = dir
	javatmp.Apply(cmd)
	out, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if ctx.Err() != nil || !ok || exit.ExitCode() != 12 {
		t.Fatalf("Java TLC: %v, context: %v\n%s", err, ctx.Err(), out)
	}
	matches := regexp.MustCompile(`(?m)^/\\ x = (.*)$`).FindAllStringSubmatch(string(out), -1)
	var got []string
	for _, match := range matches {
		got = append(got, match[1])
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Java TLC returned trace %q; want %q", got, want)
	}
}

func compareJavaRandomSubsetSequence(t *testing.T, jar, source, cfg string, seed int64, want []string) {
	t.Helper()
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, content := range map[string]string{"M.tla": source, "M.cfg": cfg} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-workers", "1", "-seed", strconv.FormatInt(seed, 10), "-fp", "0", "-deadlock", "M.tla")
	cmd.Dir = dir
	javatmp.Apply(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil || ctx.Err() != nil {
		t.Fatalf("Java TLC: %v, context: %v\n%s", err, ctx.Err(), out)
	}
	checkJavaPrintedSequence(t, string(out), want)
}

func checkJavaPrintedSequence(t *testing.T, out string, want []string) {
	t.Helper()
	for _, expected := range want {
		index := strings.Index(out, "\n"+expected+"\n")
		if index < 0 {
			t.Fatalf("Java TLC did not print %q in order:\n%s", expected, out)
		}
		out = out[index+len(expected)+1:]
	}
}
