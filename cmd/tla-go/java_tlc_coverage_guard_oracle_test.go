package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// These are Java CLI observations, not execution of the JUnit lifecycle.
// Go must keep rejecting guard cost models until their event tree is modeled.
func TestJavaTLCCoverageParameterizedGuardBoundary(t *testing.T) {
	fixture := coverageAFixtures(t)
	for _, tc := range []struct {
		name, body, cfg, inner, stats string
		fails                         bool
	}{
		{"false", "EXTENDS Naturals\nVARIABLE x\nInit == x=0\nA(n) == x<n /\\ x'=n\nB(n) == x'=n\nNext == A(0) \\/ B(1)", "", "", "3 states generated, 2 distinct states found", false},
		{"shared", "EXTENDS Naturals\nVARIABLE x\nInit == x=0\nA(n) == x<n /\\ x'=n\nNext == A(1) \\/ A(2)", "", "", "4 states generated, 3 distinct states found", false},
		{"repeat", "EXTENDS Naturals\nVARIABLE x\nInit == x=0\nA(n) == n=n /\\ x'=n\nNext == A(1) \\/ A(2)", "", "", "7 states generated, 3 distinct states found", false},
		{"multiline", "EXTENDS Naturals\nVARIABLE x\nInit == x=0\nA( n,\n   unused ) == x<n /\\ x'=n\nB(n) == x'=n\nNext == A(2,1) \\/ B(3)", "", "", "5 states generated, 3 distinct states found", false},
		{"error", "EXTENDS Naturals\nVARIABLE x\nInit == x=0\nA(n) == x<n /\\ x'=n\nB(n) == x'=n\nNext == A(TRUE) \\/ B(1)", "", "", "1 states generated, 1 distinct states found", true},
		{"constraint", "EXTENDS Naturals\nVARIABLE x\nInit == x=0\nA(n) == x<n /\\ x'=n\nNext == A(1) \\/ A(2)\nLimit == x<2", "CONSTRAINT Limit\n", "", "4 states generated, 2 distinct states found", false},
		{"instances", "VARIABLE x\nInit == x=0\nI == INSTANCE Inner WITH c <- 1, x <- x\nJ == INSTANCE Inner WITH c <- 2, x <- x\nNext == I!Act \\/ J!Act", "", "EXTENDS Naturals\nCONSTANT c\nVARIABLE x\nAct == x<c /\\ x'=c", "4 states generated, 3 distinct states found", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeCoverageModule(t, dir, "M", tc.body)
			if tc.inner != "" {
				writeCoverageModule(t, dir, "Inner", tc.inner)
			}
			writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\n"+tc.cfg)
			args := coverageAArgs("M.tla", true)
			out, code, stderr := runGoInDir(t, dir, args)
			if code == 0 || !strings.Contains(out+stderr, "-coverage does not yet support") {
				t.Fatalf("coverage should remain rejected: exit %d stderr %s output %s", code, stderr, out)
			}
			if !tc.fails {
				out, code, stderr = runGoInDir(t, dir, coverageAArgs("M.tla", false))
				if code != 0 || stderr != "" || !strings.Contains(out, tc.stats) {
					t.Fatalf("default: exit %d stderr %s output %s", code, stderr, out)
				}
			}
			if jar := pinnedFairnessEffectJar(t); jar != "" {
				checkGuardJavaOracle(t, jar, dir, args, filepath.Join(fixture, "guard-"+tc.name+".coverage"), tc.stats, tc.fails)
			}
		})
	}
}

func checkGuardJavaOracle(t *testing.T, jar, dir string, args []string, snapshot, stats string, fails bool) {
	t.Helper()
	golden, err := os.ReadFile(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	plain, exit := runJavaTLCArgs(t, jar, dir, guardJavaArgs(t, args, false))
	if (exit != 0) != fails || !strings.Contains(plain, stats) || coverageReport(t, plain) != string(golden) {
		t.Fatalf("Java normal exit %d output %s", exit, plain)
	}
	tool, toolExit := runJavaTLCArgs(t, jar, dir, guardJavaArgs(t, args, true))
	messages := readToolMessages(tool)
	if (toolExit != 0) != fails || !hasToolMessage(messages, 2201) || !hasToolMessage(messages, 2202) || hasToolMessage(messages, 2776) {
		t.Fatalf("Java tool exit %d output %s", toolExit, tool)
	}
	if fails && (!hasToolMessage(messages, 2266) || !hasToolMessage(messages, 2103) || !hasToolMessage(messages, 2121)) {
		t.Fatalf("missing Java error messages: %s", tool)
	}
	if got, want := guardCoverageMessageBodies(messages), guardSnapshotBodies(string(golden)); !reflect.DeepEqual(got, want) {
		t.Fatalf("Java tool coverage %q, normal snapshot %q", got, want)
	}
}

func guardJavaArgs(t *testing.T, args []string, tool bool) []string {
	t.Helper()
	out := append([]string{}, args[:len(args)-1]...)
	out = append(out, "-metadir", filepath.Join(t.TempDir(), "states"), args[len(args)-1])
	if tool {
		out = append([]string{"-tool"}, out...)
	}
	return out
}

func guardCoverageMessageBodies(messages []toolMessage) []string {
	var result []string
	for _, message := range messages {
		switch message.code {
		case 2221, 2772, 2773:
			result = append(result, message.body)
		}
	}
	return result
}

func guardSnapshotBodies(report string) []string {
	var result []string
	for line := range strings.SplitSeq(report, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "<") || strings.HasPrefix(line, "line ") || strings.HasPrefix(line, "|") {
			result = append(result, line)
		}
	}
	return result
}
