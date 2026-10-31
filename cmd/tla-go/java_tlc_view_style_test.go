package main

import (
	"os"
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
