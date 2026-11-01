package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Java TLC checks -maxSetSize when a ValueVec of initial capacity 10 grows, so
// the sets it builds element by element fail only above max(bound, 10)
// elements. Set operations on deferred sets stay lazy.
func TestJavaTLCMaxSetSize(t *testing.T) {
	const selectAll = "SelectSeq([i \\in 1..20 |-> i], LAMBDA e : TRUE)"
	for _, tc := range []struct {
		name, body, cfg, bound string
		code                   int
	}{
		{"power set exceeds bound", "Init == x = SUBSET (1..4)\nNext == UNCHANGED x", "", "10", 0},
		{"nested power set exceeds bound", "Init == x = [a |-> SUBSET (1..4)]\nNext == UNCHANGED x", "", "10", 0},
		{"initial filter", "Init == x \\in {i \\in 1..20 : TRUE}\nNext == UNCHANGED x", "", "10", 75},
		{"successor map", "Init == x = 0\nNext == x' \\in {i * 2 : i \\in 1..20}", "", "12", 75},
		{"at bound", "Init == x = 0\nNext == x' \\in {i * 2 : i \\in 1..12}", "", "12", 0},
		{"above bound", "Init == x = 0\nNext == x' \\in {i * 2 : i \\in 1..13}", "", "12", 75},
		{"initial capacity", "Init == x \\in {i \\in 1..10 : TRUE}\nNext == UNCHANGED x", "", "1", 0},
		{"above initial capacity", "Init == x \\in {i \\in 1..11 : TRUE}\nNext == UNCHANGED x", "", "1", 75},
		{"duplicates count", "Init == x \\in {i % 2 : i \\in 1..11}\nNext == UNCHANGED x", "", "1", 75},
		{"assumption", "ASSUME Cardinality({i : i \\in 1..20}) = 20\nInit == x = 0\nNext == UNCHANGED x", "", "15", 10},
		{"initial invariant", "Init == x = 0\nNext == UNCHANGED x\nInv == Cardinality({i \\in 1..20 : i > x}) > 0", "INVARIANT Inv\n", "15", 75},
		{"successor invariant", "Init == x = 0\nNext == x < 3 /\\ x' = x + 1\nInv == x < 2 \\/ Cardinality({i * x : i \\in 1..20}) > 0", "INVARIANT Inv\n", "15", 76},
		{"union", "Init == x \\in (1..20) \\cup {0}\nNext == UNCHANGED x", "", "10", 75},
		{"interval union", "Init == x \\in (1..6) \\cup (7..11)\nNext == UNCHANGED x", "", "10", 75},
		{"union membership error", "Init == x = Cardinality((1..20) \\cup {\"a\"})\nNext == UNCHANGED x", "", "10", 75},
		{"successor union", "Init == x = 0\nNext == x' \\in (1..20) \\cup {0}", "", "10", 75},
		{"difference", "Init == x \\in (1..20) \\ {0}\nNext == UNCHANGED x", "", "10", 75},
		{"intersection below", "Init == x = Cardinality((1..9) \\cap (1..9))\nNext == UNCHANGED x", "", "10", 0},
		{"intersection at", "Init == x = Cardinality((1..10) \\cap (1..10))\nNext == UNCHANGED x", "", "10", 0},
		{"intersection above", "Init == x = Cardinality((1..11) \\cap (1..11))\nNext == UNCHANGED x", "", "10", 75},
		{"intersection huge", "Init == x = Cardinality((1..2147483647) \\cap (1..2147483647))\nNext == UNCHANGED x", "", "10", 75},
		{"intersection", "Init == x \\in (1..20) \\cap (0..30)\nNext == UNCHANGED x", "", "10", 75},
		{"deferred union", "Init == x \\in [1..1 -> 1..6] \\cup [1..1 -> 7..11]\nNext == UNCHANGED x", "", "10", 0},
		{"enumerated UNION", "Init == x \\in UNION {{1, 2, 3, 4, 5, 6}, {7, 8, 9, 10, 11}}\nNext == UNCHANGED x", "", "10", 75},
		{"powerset", "Init == x = SUBSET (1..5)\nNext == UNCHANGED x", "", "10", 0},
		{"SelectSeq", "Init == x = SelectSeq([i \\in 1..20 |-> i], LAMBDA e : TRUE)\nNext == UNCHANGED x", "", "10", 75},
		{"function override", "Init == x = [i \\in 1..8 |-> i] @@ [j \\in 9..12 |-> j]\nNext == UNCHANGED x", "", "10", 75},
		{"BagUnion", "Init == x = BagUnion({[i \\in 1..8 |-> 1], [j \\in 9..12 |-> 1]})\nNext == UNCHANGED x", "", "10", 75},
		{"BagOfAll", "Init == x = BagOfAll(LAMBDA e : e, [i \\in 1..12 |-> 1])\nNext == UNCHANGED x", "", "10", 75},
		{"deferred cardinality", "Init == x = Cardinality(UNION {1..10, 11..20})\nNext == UNCHANGED x", "", "10", 75},
		{"function set cardinality", "Init == x = Cardinality([1..2 -> 1..5])\nNext == UNCHANGED x", "", "10", 0},
		{"SelectSeq successor", "Init == x = 0\nNext == x' = Len(" + selectAll + ")", "", "10", 75},
		{"SelectSeq initial invariant", "Init == x = 0\nNext == UNCHANGED x\nInv == " + selectAll + " # <<>>", "INVARIANT Inv\n", "10", 75},
		{"SelectSeq successor invariant", "Init == x = 0\nNext == x < 2 /\\ x' = x + 1\nInv == x < 1 \\/ " + selectAll + " # <<>>", "INVARIANT Inv\n", "10", 76},
		{"SelectSeq assumption", "ASSUME " + selectAll + " # <<>>\nInit == x = 0\nNext == UNCHANGED x", "", "10", 10},
		{"SelectSeq initial constraint", "Init == x = 0\nNext == x < 2 /\\ x' = x + 1\nC == " + selectAll + " # <<>>", "CONSTRAINT C\n", "10", 75},
		{"SelectSeq successor constraint", "Init == x = 0\nNext == x < 2 /\\ x' = x + 1\nC == x < 1 \\/ " + selectAll + " # <<>>", "CONSTRAINT C\n", "10", 75},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeMaxSetSizeFixture(t, dir, "VARIABLE x\n"+tc.body, "INIT Init\nNEXT Next\n"+tc.cfg)
			args := []string{"-tool", "-deadlock", "-maxSetSize", tc.bound}
			var out, errout bytes.Buffer
			code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
			if code != tc.code || errout.Len() != 0 || (code != 0) != strings.Contains(out.String(), "Attempted to construct a set with too many elements (>"+tc.bound+").") {
				t.Fatalf("exit %d, want %d; stdout:\n%s\nstderr:\n%s", code, tc.code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runJavaToolMessages(t, jar, dir, append(args, "M.tla"), code)
				if got := errorBehaviorMessages(readToolMessages(out.String())); !reflect.DeepEqual(got, java) {
					t.Fatalf("messages differ\nJava: %q\nGo:   %q", java, got)
				}
			}
		})
	}
}

// Java TLC's simulator reports a -maxSetSize failure as an unexpected
// exception, and a failed override as message 2154 without its summary.
func TestJavaTLCMaxSetSizeSimulation(t *testing.T) {
	for _, tc := range []struct{ name, body, message string }{
		{"initial filter", "Init == x \\in {i \\in 1..20 : TRUE}\nNext == x' = x", "The exception was a util.WrongInvocationException\n: Attempted to construct a set with too many elements (>10)."},
		{"successor filter", "Init == x = 0\nNext == x' \\in {i \\in 1..20 : TRUE}", "The exception was a util.WrongInvocationException\n: Attempted to construct a set with too many elements (>10)."},
		{"successor SelectSeq", "Init == x = 0\nNext == x' = Len(SelectSeq([i \\in 1..20 |-> i], LAMBDA e : TRUE))", "@!@!@STARTMSG 2154:1 @!@!@\nAttempted to apply the operator overridden by the Java method"},
		{"successor non-boolean", "Init == x = 0\nNext == x' = 1 /\\ 5", "@!@!@STARTMSG 2247:1 @!@!@\nIn computing next states, TLC expected a boolean expression,"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeMaxSetSizeFixture(t, dir, "VARIABLE x\n"+tc.body, "INIT Init\nNEXT Next\n")
			args := []string{"-tool", "-maxSetSize", "10", "-simulate", "num=1", "-depth", "3", "-seed", "1"}
			var out, errout bytes.Buffer
			code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
			if code != 255 || errout.Len() != 0 || !strings.Contains(out.String(), tc.message) {
				t.Fatalf("exit %d; stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runJavaToolMessages(t, jar, dir, append(args, "M.tla"), code)
				if got := errorBehaviorMessages(readToolMessages(out.String())); !reflect.DeepEqual(got, java) {
					t.Fatalf("messages differ\nJava: %q\nGo:   %q", java, got)
				}
			}
		})
	}
}

func writeMaxSetSizeFixture(t *testing.T, dir, body, cfg string) {
	t.Helper()
	for name, content := range map[string]string{"M.tla": "---- MODULE M ----\nEXTENDS Naturals, Sequences, FiniteSets, Bags, TLC\n" + body + "\n====\n", "M.cfg": cfg} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// The upstream TraceWithLargeSetOfInitialStatesTest checks that a bound below
// the number of initial states still reports the correct trace.
func TestJavaTLCUpstreamLargeInitialStates(t *testing.T) {
	name := "TraceWithLargeSetOfInitialStatesTest"
	dir := copyJavaTLCFixture(t, name)
	args := []string{"-tool", "-deadlock", "-maxSetSize", "10"}
	var out, errout bytes.Buffer
	code := run(context.Background(), append(args, filepath.Join(dir, name+".tla")), &out, &errout)
	want := "1: <Initial predicate>\n/\\ x = 1\n/\\ y = FALSE\n"
	if code != 12 || errout.Len() != 0 || !strings.Contains(out.String(), want) || !strings.Contains(out.String(), "/\\ x = 1\n/\\ y = TRUE\n") {
		t.Fatalf("exit %d; stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java := runJavaToolMessages(t, jar, dir, append(args, name+".tla"), code)
		if got := errorBehaviorMessages(readToolMessages(out.String())); !reflect.DeepEqual(got, java) {
			t.Fatalf("messages differ\nJava: %q\nGo:   %q", java, got)
		}
	}
}

func TestJavaTLCInvalidMaxSetSize(t *testing.T) {
	for _, tc := range []struct {
		args    []string
		message string
	}{
		{[]string{"-maxSetSize", "0", "M.tla"}, "Error: Value in interval [0, 2147483647] for maxSetSize required. But encountered 0"},
		{[]string{"-maxSetSize", "-3", "M.tla"}, "Error: Value in interval [0, 2147483647] for maxSetSize required. But encountered -3"},
		{[]string{"-maxSetSize", "2147483648", "M.tla"}, "Error: An integer for maxSetSize required. But encountered 2147483648"},
		{[]string{"M.tla", "-maxSetSize"}, "Error: maxSetSize required."},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n")
			var out, errout bytes.Buffer
			if code := run(context.Background(), tc.args, &out, &errout); code != 1 || errout.Len() != 0 || out.String() != printedOptionError(tc.message) {
				t.Fatalf("exit %d; stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				cmd := exec.Command("java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-jar", jar}, tc.args...)...)
				cmd.Dir = dir
				output, _ := cmd.CombinedOutput()
				if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 1 || !strings.Contains(string(output), tc.message) {
					t.Fatalf("Java exit %v; output:\n%s", cmd.ProcessState, output)
				}
			}
		})
	}
}
