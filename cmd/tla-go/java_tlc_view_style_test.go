package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const viewDiffTraceModel = diffTraceModel + "\nV == <<x, \"view\">>"

// Java TLC prints a -difftrace difference with TLCStateMut.toString(TLCState),
// which lists the changed variables even under -view; states printed in full
// go through VIEW.
func TestJavaTLCViewDiffTrace(t *testing.T) {
	for _, tc := range []struct {
		name, cfg string
		extra     []string
		code      int
		want      []string
	}{
		{"invariant", "INIT Init\nNEXT Next\nINVARIANT Inv4\nVIEW V\n", nil, 12, []string{"1: <Initial predicate>\n<<0, \"view\">>\n", "2: <A line 6, col 6 to line 6, col 37 of module M>\n/\\ x = 1\n", "4: <A line 6, col 6 to line 6, col 37 of module M>\n<<3, \"view\">>"}},
		{"liveness", "SPECIFICATION Live\nPROPERTY Never\nVIEW V\n", nil, 13, []string{"2: <Cycle line 10, col 10 to line 10, col 47 of module M>\n/\\ x = 1\n"}},
		{"simulation", "INIT Init\nNEXT Next\nINVARIANT Inv4\nVIEW V\n", []string{"-simulate", "num=1", "-depth", "10", "-seed", "4"}, 12, []string{"<<0, \"view\">>\n", "/\\ x = 1\n"}},
		{"simulation liveness", "SPECIFICATION Live\nPROPERTY Never\nVIEW V\n", []string{"-simulate", "num=2", "-depth", "6", "-seed", "1"}, 13, []string{"<<0, \"view\">>\n", "/\\ x = 1\n"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, viewDiffTraceModel, tc.cfg)
			args := append(append([]string{"-tool", "-workers", "1", "-fp", "0", "-view", "-difftrace"}, tc.extra...), "M.tla")
			messages, code, stderr := runGoToolInDir(t, dir, args)
			traces := strings.Join(traceStateMessages(messages), "\n")
			if code != tc.code || stderr != "" || !containsAll(traces, tc.want) || strings.Contains(traces, "<<1, \"view\">>") {
				t.Fatalf("exit %d, want %d; traces %q; stderr %s", code, tc.code, traces, stderr)
			}
			compareJavaTraceStates(t, dir, args, messages, code)
		})
	}
}

// compareJavaTraceStates runs Java TLC with the same arguments and compares
// the printed trace states.
func compareJavaTraceStates(t *testing.T, dir string, args []string, messages []toolMessage, code int) {
	t.Helper()
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		return
	}
	java, javaCode := runJavaTLCArgs(t, jar, dir, args)
	traces := traceStateMessages(messages)
	if javaTraces := traceStateMessages(readToolMessages(java)); javaCode != code || !reflect.DeepEqual(javaTraces, traces) {
		t.Fatalf("Java exit %d, Go exit %d\nJava: %q\nGo: %q", javaCode, code, javaTraces, traces)
	}
}

const terseViewModel = "VARIABLES x, s\nInit == x = 0 /\\ s = SUBSET {1, 2}\nNext == x < 3 /\\ x' = x + 1 /\\ UNCHANGED s\nInv == x < 2\n"

// Java TLC's -terse leaves values unexpanded wherever TLCStateMut.toString
// prints them, including in VIEW text.
func TestJavaTLCTerseView(t *testing.T) {
	for _, tc := range []struct{ name, view, want string }{
		{"tuple", "<<s, x>>", "/\\ s = SUBSET {1, 2}\n/\\ x = 0\n"},
		{"record", "[a |-> x, b |-> s]", "[a |-> 2, b |-> SUBSET {1, 2}]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, terseViewModel+"V == "+tc.view, "INIT Init\nNEXT Next\nINVARIANT Inv\nVIEW V\n")
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-view", "-terse", "M.tla"}
			messages, code, stderr := runGoToolInDir(t, dir, args)
			if traces := strings.Join(traceStateMessages(messages), "\n"); code != 12 || stderr != "" || !strings.Contains(traces, tc.want) {
				t.Fatalf("exit %d; traces %q; stderr %s", code, traces, stderr)
			}
			compareJavaTraceStates(t, dir, args, messages, code)
		})
	}
}

// Java TLC's state writers print states with TLCStateMut.toString, so -terse
// applies to dumped states and DOT labels.
func TestJavaTLCDumpTerse(t *testing.T) {
	for _, tc := range []struct {
		format, want string
		extra        []string
	}{
		{"", "State 1:\n/\\ s = SUBSET {1, 2}\n/\\ x = 0\n\n", []string{"-terse"}},
		{"dot", `[label="/\\ s = SUBSET {1, 2}\n/\\ x = 0",style = filled]`, []string{"-terse"}},
		{"", "State 1:\n[a |-> 0, b |-> SUBSET {1, 2}]\n", []string{"-view", "-terse"}},
		{"dot", `[label="[a |-> 0, b |-> SUBSET {1, 2}]",style = filled]`, []string{"-view", "-terse"}},
	} {
		t.Run(tc.format+strings.Join(tc.extra, ""), func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, terseViewModel+"V == [a |-> x, b |-> s]", "INIT Init\nNEXT Next\nVIEW V\n")
			output := filepath.Join(dir, "go.dump")
			if tc.format != "" {
				output = filepath.Join(dir, "go.dot")
			}
			got := runDumpFormat(t, dir, "M", tc.format, output, 0, tc.extra...)
			if !strings.Contains(string(got), tc.want) {
				t.Fatalf("dump missing %q:\n%s", tc.want, got)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaDumpFormat(t, jar, dir, "M", tc.format, got, 0, tc.extra...)
			}
		})
	}
}

// SimulationWorker writes trace files with TLCStateMut.toString, which
// follows -view and -terse.
func TestJavaTLCSimulationTraceFileStyles(t *testing.T) {
	for _, tc := range []struct {
		name, view, want string
		extra            []string
	}{
		{"view", "<<x>>", "STATE_1 == \n/\\ x = 0\n\n\n", []string{"-view"}},
		{"terse", "<<x>>", "STATE_1 == \n/\\ s = SUBSET {1, 2}\n/\\ x = 0\n\n\n", []string{"-terse"}},
		{"terse view", "[a |-> x, b |-> s]", "STATE_1 == \n[a |-> 0, b |-> SUBSET {1, 2}]\n\n", []string{"-view", "-terse"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, terseViewModel+"V == "+tc.view, "INIT Init\nNEXT Next\nVIEW V\n")
			prefix := filepath.Join(dir, "go")
			common := append([]string{"-tool", "-workers", "1", "-seed", "1", "-deadlock"}, tc.extra...)
			args := append(append([]string{}, common...), "-simulate", "file="+prefix+",num=2", filepath.Join(dir, "M.tla"))
			var out, errout bytes.Buffer
			if code := run(context.Background(), args, &out, &errout); code != 0 {
				t.Fatalf("exit %d: %s\n%s", code, out.String(), errout.String())
			}
			files, err := filepath.Glob(prefix + "_0_*")
			if err != nil || len(files) != 2 {
				t.Fatalf("files %v, %v; want 2", files, err)
			}
			if data, err := os.ReadFile(files[0]); err != nil || !strings.Contains(string(data), tc.want) {
				t.Fatalf("trace file %s, %v; missing %q", data, err, tc.want)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareSimulationFiles(t, jar, dir, common, 2, 0, prefix, files, false)
			}
		})
	}
}

// LiveCheck1 prints the counterexample it recovers before an exception with
// StatePrinter, so -view, -terse, and -difftrace apply to it as well.
func TestJavaTLCSimulationTemporalTraceStyles(t *testing.T) {
	model := "VARIABLES x, y\nInit == x = 0 /\\ y = SUBSET {1, 2}\n" +
		"Next == /\\ \\/ TLCGet(\"level\") < 5 /\\ x' = (x + 1) % 6\n           \\/ TLCGet(\"level\") >= 5 /\\ x' = (x + 3) % 6\n        /\\ UNCHANGED y\n" +
		"Spec == Init /\\ [][Next]_<<x, y>>\nP == <>[](x = 0)\nV == <<x>>"
	for _, tc := range []struct {
		name  string
		extra []string
		want  string
	}{
		{"difftrace terse", []string{"-difftrace", "-terse"}, "1: <Initial predicate>\n/\\ x = 0\n/\\ y = SUBSET {1, 2}|2: <Next line 5, col 9 to line 7, col 22 of module M>\n/\\ x = 1|"},
		{"view", []string{"-view"}, "1: <Initial predicate>\n/\\ x = 0|2: <Next line 5, col 9 to line 7, col 22 of module M>\n/\\ x = 1|"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, model, "SPECIFICATION Spec\nPROPERTY P\nVIEW V\n")
			args := append(append([]string{"-tool", "-workers", "1", "-fp", "0", "-simulate", "num=1", "-seed", "1", "-depth", "10"}, tc.extra...), "M.tla")
			messages, code, stderr := runGoToolInDir(t, dir, args)
			if prefix := temporalPrefix(messages); code != 255 || stderr != "" || prefix != tc.want {
				t.Fatalf("exit %d; temporal prefix %q, want %q; stderr %s", code, prefix, tc.want, stderr)
			}
			compareJavaTraceStates(t, dir, args, messages, code)
		})
	}
}

// temporalPrefix joins the trace states printed before the first exception.
func temporalPrefix(messages []toolMessage) string {
	var prefix strings.Builder
	for _, message := range messages {
		if message.code == 1000 {
			break
		}
		if message.code == 2217 {
			prefix.WriteString(message.body + "|")
		}
	}
	return prefix.String()
}
