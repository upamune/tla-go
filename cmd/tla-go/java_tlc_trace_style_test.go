package main

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

const diffTraceModel = "VARIABLES x, y, z\nvars == <<x, y, z>>\nInit == x = 0 /\\ y = 0 /\\ z = <<1, 2>>\n" +
	"A == x' = x + 1 /\\ UNCHANGED <<y, z>>\nB == y' = y + 1 /\\ z' = <<y, 2>> /\\ UNCHANGED x\nNext == A \\/ B\n" +
	"Stop == x < 2 /\\ Next\nCycle == x' = (x + 1) % 3 /\\ UNCHANGED <<y, z>>\nLive == Init /\\ [][Cycle]_vars /\\ WF_vars(Cycle)\n" +
	"Inv4 == x + y < 3\nInv2 == x + y < 1\nInv3 == x + y < 2\nAct == [][x' = x]_vars\nNever == <>(x = 5)\nBad == x < 2 \\/ (1 \\div 0 = 0)"

// Java TLC's -difftrace prints only changed variables. TLCTrace prints the
// last state in full unless it is a checked successor of an initial state,
// while liveness and simulation traces print every later state as a
// difference.
func TestJavaTLCDiffTrace(t *testing.T) {
	for _, tc := range []struct {
		name, cfg string
		extra     []string
		code      int
		want      []string
	}{
		{"invariant", "INIT Init\nNEXT Next\nINVARIANT Inv4\n", nil, 12, []string{"2: <A", "/\\ x = 1\n", "4: <A", "/\\ x = 3\n/\\ y = 0\n/\\ z = <<1, 2>>"}},
		{"successor of initial", "INIT Init\nNEXT Next\nINVARIANT Inv2\n", nil, 12, []string{"2: <A line 6, col 6 to line 6, col 37 of module M>\n/\\ x = 1"}},
		{"depth three", "INIT Init\nNEXT Next\nINVARIANT Inv3\n", nil, 12, []string{"/\\ x = 2\n/\\ y = 0"}},
		{"action property", "INIT Init\nNEXT Next\nPROPERTY Act\n", nil, 13, []string{"2: <A line 6, col 6 to line 6, col 37 of module M>\n/\\ x = 1"}},
		{"deadlock", "INIT Init\nNEXT Stop\n", nil, 11, []string{"/\\ x = 2\n/\\ y = 0"}},
		{"invariant evaluation", "INIT Init\nNEXT Next\nINVARIANT Bad\n", nil, 76, []string{"/\\ x = 1\n"}},
		{"liveness", "SPECIFICATION Live\nPROPERTY Never\n", nil, 13, []string{"3: <Cycle line 10, col 10 to line 10, col 47 of module M>\n/\\ x = 2"}},
		{"simulation", "INIT Init\nNEXT Next\nINVARIANT Inv4\n", []string{"-simulate", "num=1", "-depth", "10", "-seed", "1"}, 12, []string{"4: <A line 6, col 6 to line 6, col 37 of module M>\n/\\ x = 2"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, diffTraceModel, tc.cfg)
			args := append(append([]string{"-tool", "-workers", "1", "-fp", "0", "-difftrace"}, tc.extra...), "M.tla")
			messages, code, stderr := runGoToolInDir(t, dir, args)
			traces := traceStateMessages(messages)
			if code != tc.code || stderr != "" || !containsAll(strings.Join(traces, "\n"), tc.want) {
				t.Fatalf("exit %d, want %d; traces %q; stderr %s", code, tc.code, traces, stderr)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				if javaTraces := traceStateMessages(readToolMessages(java)); javaCode != code || !reflect.DeepEqual(javaTraces, traces) {
					t.Fatalf("Java exit %d, Go exit %d\nJava: %q\nGo: %q", javaCode, code, javaTraces, traces)
				}
			}
		})
	}
}

// Java TLC's -terse prints SUBSET, function-set, record-set, and product
// values as formulas in Print output and traces.
func TestJavaTLCTerseValues(t *testing.T) {
	dir := t.TempDir()
	body := "S == {1, 2}\nASSUME PrintT(SUBSET S)\nASSUME PrintT(<<[S -> S], [a : S], S \\X {3}, {SUBSET {1}}>>)\n" +
		"ASSUME Print([x |-> SUBSET S], TRUE)\nASSUME PrintT({n \\in S : n > 1} \\cup {3})\n" +
		"VARIABLE x\nInit == x = SUBSET S\nNext == x' = [S -> {0}]\nInv == x = SUBSET S"
	writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nINVARIANT Inv\n")
	args := []string{"-workers", "1", "-fp", "0", "-terse", "M.tla"}
	want := []string{
		"SUBSET {1, 2}\n",
		"<<[{1, 2} -> {1, 2}], [a: {1, 2}], ({1, 2} \\X {3}), {SUBSET {1}}>>\n",
		"[x |-> SUBSET {1, 2}]  TRUE\n",
		"{2, 3}\n",
		"x = SUBSET {1, 2}\n",
		"x = [{1, 2} -> {0}]\n",
	}
	output, code, stderr := runGoInDir(t, dir, args)
	if code != 12 || stderr != "" || !containsAll(output, want) {
		t.Fatalf("exit %d; stdout:\n%s\nstderr:\n%s", code, output, stderr)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java, javaCode := runJavaTLCArgs(t, jar, dir, args)
		if javaCode != code || !containsAll(java, want) {
			t.Fatalf("Java exit %d:\n%s", javaCode, java)
		}
	}
}

func traceStateMessages(messages []toolMessage) []string {
	var traces []string
	for _, message := range messages {
		if message.code == 2217 || message.code == 2218 || message.code == 2122 {
			traces = append(traces, message.body)
		}
	}
	return traces
}

func containsAll(text string, parts []string) bool {
	for _, part := range parts {
		if !strings.Contains(text, part) {
			return false
		}
	}
	return true
}
