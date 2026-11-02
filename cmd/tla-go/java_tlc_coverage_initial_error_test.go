package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Initial exceptions cause TLC 1.7.4 to rerun initialization with its
// CallStackTool. Coverage records both runs; state statistics do not double.
func TestJavaTLCCoverageInitialErrorReplay(t *testing.T) {
	for _, tc := range []struct {
		name, init, extra, cfg string
	}{
		{"invariant-first", "x = TRUE", "Inv == x < 2", "INVARIANT Inv\n"},
		{"invariant-second", `x \in 0..2`, `Inv == x = 0 \/ x < Nat`, "INVARIANT Inv\n"},
		{"constraint-second", `x \in 0..1`, `Limit == IF x = 0 THEN TRUE ELSE x < TRUE`, "CONSTRAINT Limit\n"},
		{"constraint-print", `x \in 0..1`, `Limit == IF PrintT(x) THEN IF x = 0 THEN TRUE ELSE x < TRUE ELSE FALSE`, "CONSTRAINT Limit\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "EXTENDS Naturals, TLC\nVARIABLE x\nInit == " + tc.init + "\nA == UNCHANGED x\nB == UNCHANGED x\nNext == A \\/ B\n" + tc.extra
			writeCoverageModule(t, dir, "M", body)
			cfg := "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n" + tc.cfg
			if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte(cfg), 0o600); err != nil {
				t.Fatal(err)
			}
			args := []string{"-tool", "-fp", "0", "-workers", "1", "-checkpoint", "0", "-coverage", "9999", "M.tla"}
			out, code, stderr := runGoInDir(t, dir, args)
			if code != 75 || stderr != "" {
				t.Fatalf("Go exit %d stderr %s\n%s", code, stderr, out)
			}
			if jar := pinnedFairnessEffectJar(t); jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				got, want := coverageMessages(readToolMessages(out)), coverageMessages(readToolMessages(java))
				if javaCode != code || !reflect.DeepEqual(got, want) || rawToolMessage(out, 2199) != rawToolMessage(java, 2199) || rawToolMessage(out, 2103) != rawToolMessage(java, 2103) || strings.Count(out, "@!@!@STARTMSG 2103:") != strings.Count(java, "@!@!@STARTMSG 2103:") || tc.name == "constraint-print" && !reflect.DeepEqual(initialErrorEventOrder(out), initialErrorEventOrder(java)) {
					t.Fatalf("Java exit %d Go exit %d\nJava: %#v\nGo: %#v\nJava output: %s\nGo output: %s", javaCode, code, want, got, java, out)
				}
			}
		})
	}
}

func initialErrorEventOrder(output string) []string {
	var events []string
	for line := range strings.SplitSeq(output, "\n") {
		if line == "0" || line == "1" || strings.HasPrefix(line, "@!@!@STARTMSG 1000:") || strings.HasPrefix(line, "@!@!@STARTMSG 2103:") {
			events = append(events, line)
		}
	}
	return events
}

// The original failure and its diagnostic replay need not fail at the same
// expression or even at the same initial candidate when TLC registers mutate.
func TestJavaTLCCoverageInitialErrorMutableReplayBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, constraint, wantStack, wantStats string
	}{
		{"different-expression", `IF x = 2 THEN IF TLCGet(0) = 0 THEN TLCSet(0,1) /\ x < TRUE ELSE x + TRUE = 0 ELSE TRUE`, "Line 10, column 75 to line 10, column 82", "3 states generated, 2 distinct states found"},
		{"earlier-candidate", `IF TLCGet(0) = 0 THEN IF x = 2 THEN TLCSet(0,1) /\ x < TRUE ELSE TRUE ELSE IF x = 1 THEN x + TRUE = 0 ELSE TRUE`, "Line 10, column 99 to line 10, column 106", "2 states generated, 2 distinct states found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nASSUME TLCSet(0,0)\nInit == x \\in 0..2\nA == UNCHANGED x\nB == UNCHANGED x\nNext == A \\/ B\nInv == TRUE\nLimit == " + tc.constraint + "\n====\n"
			if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\nCONSTRAINT Limit\nCHECK_DEADLOCK FALSE\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			args := []string{"-tool", "-fp", "0", "-workers", "1", "-checkpoint", "0", "-coverage", "9999", "M.tla"}
			out, code, _ := runGoInDir(t, dir, args)
			if code == 0 || !strings.Contains(out, "-coverage cannot report initial error replay with mutable TLC or random operators") || hasToolMessage(readToolMessages(out), 2201) || hasToolMessage(readToolMessages(out), 2103) {
				t.Fatalf("unsupported replay reported as supported: exit %d\n%s", code, out)
			}
			if jar := pinnedFairnessEffectJar(t); jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				if javaCode != 75 || !strings.Contains(rawToolMessage(java, 2103), tc.wantStack) || !strings.Contains(rawToolMessage(java, 2199), tc.wantStats) {
					t.Fatalf("unexpected Java replay: exit %d\n%s", javaCode, java)
				}
			}
		})
	}
}
