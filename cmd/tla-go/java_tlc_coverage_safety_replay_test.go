package main

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestJavaTLCCoverageSafetyTraceReplay(t *testing.T) {
	for _, tc := range []struct {
		name, body, config string
		exit               int
	}{
		{"violation-deep", "EXTENDS Naturals\nVARIABLES x,y\nInit == x=0 /\\ y=0\nA == /\\ y < 2\n     /\\ x' = x\n     /\\ y' = y + 1\nB == /\\ y < 2\n     /\\ x' = x\n     /\\ y' = y + 1\nNext == A \\/ B\nInv == y < 2", "INVARIANT Inv\nCHECK_DEADLOCK FALSE\n", 12},
		{"deadlock-deep", "EXTENDS Naturals\nVARIABLES x,y\nInit == x=0 /\\ y=0\nA == /\\ y < 2\n     /\\ x' = x\n     /\\ y' = y + 1\nB == /\\ y < 2\n     /\\ x' = x\n     /\\ y' = y + 1\nNext == A \\/ B", "", 11},
		{"violation-immediate", "EXTENDS Naturals\nVARIABLE x\nInit == x=0\nA == x'=1\nB == x'=2\nNext == A \\/ B\nInv == x < 1", "INVARIANT Inv\n", 12},
		{"later-action", "EXTENDS Naturals\nVARIABLE x\nInit == x=0\nA == x'=0\nB == x'=1\nC == x'=2\nNext == A \\/ B \\/ C\nInv == x < 1", "INVARIANT Inv\n", 12},
		{"initial-range", "EXTENDS Naturals\nVARIABLE x\nInit == x \\in 0..1\nA == /\\ x < 3\n     /\\ x' = x + 1\nB == /\\ x < 3\n     /\\ x' = x + 1\nNext == A \\/ B\nInv == x < 3", "INVARIANT Inv\n", 12},
		{"next-range", "EXTENDS Naturals\nVARIABLE x\nInit == x=0\nA == x' \\in 0..3\nB == x'=x\nNext == A \\/ B\nInv == x < 2", "INVARIANT Inv\n", 12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeCoverageModule(t, dir, "M", tc.body)
			writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\n"+tc.config)
			args := []string{"-tool", "-fp", "0", "-workers", "1", "-coverage", "9999", "-checkpoint", "0", "M.tla"}
			out, code, stderr := runGoInDir(t, dir, args)
			if code != tc.exit || stderr != "" {
				t.Fatalf("Go exit %d stderr %s\n%s", code, stderr, out)
			}
			if jar := pinnedFairnessEffectJar(t); jar != "" {
				javaArgs := append([]string{}, args[:len(args)-1]...)
				javaArgs = append(javaArgs, "-metadir", filepath.Join(t.TempDir(), "states"), "M.tla")
				java, exit := runJavaTLCArgs(t, jar, dir, javaArgs)
				got, want := coverageMessages(readToolMessages(out)), coverageMessages(readToolMessages(java))
				if exit != code || !reflect.DeepEqual(got, want) || rawToolMessage(out, 2199) != rawToolMessage(java, 2199) {
					t.Fatalf("Java exit %d\nGo coverage %+v\nJava coverage %+v\nGo:\n%s\nJava:\n%s", exit, got, want, out, java)
				}
			}
		})
	}
}

// Java's state equality can itself fail while recovering a trace. Do not emit
// a partial coverage report as if that separate recovery path were supported.
func TestCoverageSafetyTraceReplayMixedTypeBoundary(t *testing.T) {
	dir := t.TempDir()
	body := "EXTENDS Naturals\nVARIABLES x,y\nInit == x=0 /\\ y=0\nA == /\\ x'=0\n     /\\ y'=0\nB == /\\ x'=TRUE\n     /\\ y'=1\nNext == A \\/ B\nInv == y < 1"
	writeCoverageModule(t, dir, "M", body)
	writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\nINVARIANT Inv\n")
	args := []string{"-tool", "-fp", "0", "-workers", "1", "-coverage", "9999", "-checkpoint", "0", "M.tla"}
	out, code, stderr := runGoInDir(t, dir, args)
	if code == 0 || len(coverageMessages(readToolMessages(out))) != 0 || !strings.Contains(out+stderr, "Attempted to check equality") {
		t.Fatalf("expected rejection without coverage: exit %d stderr %s\n%s", code, stderr, out)
	}
	if jar := pinnedFairnessEffectJar(t); jar != "" {
		javaArgs := append([]string{}, args[:len(args)-1]...)
		javaArgs = append(javaArgs, "-metadir", filepath.Join(t.TempDir(), "states"), "M.tla")
		java, exit := runJavaTLCArgs(t, jar, dir, javaArgs)
		if exit != 12 || !hasToolMessage(readToolMessages(java), 2110) || !strings.Contains(java, "<A line 5, col 1 to line 5, col 1 of module M>: 0:2") {
			t.Fatalf("Java exit %d\n%s", exit, java)
		}
	}
}

// An evaluation exception also invokes TLC's separate error-stack replay.
func TestJavaTLCCoverageSafetyEvaluationErrorReplay(t *testing.T) {
	for _, tc := range []struct {
		name, body, config string
		exit               int
		stack              bool
	}{
		{"immediate", "EXTENDS Naturals\nVARIABLE x\nInit == x = 0\nA == x' = TRUE\nB == x' = FALSE\nNext == A \\/ B\nInv == x < 2", "", 255, true},
		{"deep", "EXTENDS Naturals\nVARIABLES x,y\nInit == x = 0 /\\ y = 0\nB == /\\ y = 1\n     /\\ x' = TRUE\n     /\\ y' = y\nA == /\\ y < 1\n     /\\ x' = x\n     /\\ y' = y + 1\nNext == B \\/ A\nInv == x < 2", "", 76, false},
		{"multiple-successors", "EXTENDS Naturals\nVARIABLES x,y\nInit == x = 0 /\\ y = 0\nA == /\\ x' = TRUE\n     /\\ y' \\in 0..2\nB == /\\ x' = FALSE\n     /\\ y' = 0\nNext == A \\/ B\nInv == x < 2", "", 255, true},
		{"excluded-successor", "EXTENDS Naturals\nVARIABLES x,y\nInit == x = 0 /\\ y = 0\nA == /\\ x' = TRUE\n     /\\ y' = 1\nB == /\\ x' = FALSE\n     /\\ y' = 1\nNext == A \\/ B\nInv == x < 2\nLimit == y < 1", "CONSTRAINT Limit\n", 255, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeCoverageModule(t, dir, "M", tc.body)
			writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\nINVARIANT Inv\n"+tc.config)
			args := []string{"-tool", "-fp", "0", "-workers", "1", "-coverage", "9999", "-checkpoint", "0", "M.tla"}
			out, code, stderr := runGoInDir(t, dir, args)
			if code != tc.exit || stderr != "" || !hasToolMessage(readToolMessages(out), 2111) || hasToolMessage(readToolMessages(out), 2103) != tc.stack {
				t.Fatalf("Go evaluation failure: exit %d stderr %s\n%s", code, stderr, out)
			}
			if jar := pinnedFairnessEffectJar(t); jar != "" {
				javaArgs := append([]string{}, args[:len(args)-1]...)
				javaArgs = append(javaArgs, "-metadir", filepath.Join(t.TempDir(), "states"), "M.tla")
				java, exit := runJavaTLCArgs(t, jar, dir, javaArgs)
				if exit != code || !reflect.DeepEqual(coverageMessages(readToolMessages(out)), coverageMessages(readToolMessages(java))) || !reflect.DeepEqual(coverageSafetyErrorMessages(out), coverageSafetyErrorMessages(java)) {
					t.Fatalf("Java evaluation failure: exit %d\nGo:\n%s\nJava:\n%s", exit, out, java)
				}
				compareCoverageSafetyErrorHuman(t, jar, dir, args[1:], code)
			}
		})
	}
}

func coverageSafetyErrorMessages(output string) []toolMessage {
	var messages []toolMessage
	for _, message := range readToolMessages(output) {
		switch message.code {
		case 2111, 2121, 2217, 2103, 2199:
			messages = append(messages, message)
		}
	}
	return messages
}

func compareCoverageSafetyErrorHuman(t *testing.T, jar, dir string, args []string, want int) {
	t.Helper()
	out, code, stderr := runGoInDir(t, dir, args)
	javaArgs := append([]string{}, args[:len(args)-1]...)
	javaArgs = append(javaArgs, "-metadir", filepath.Join(t.TempDir(), "states"), "M.tla")
	java, exit := runJavaTLCArgs(t, jar, dir, javaArgs)
	if code != want || exit != want || stderr != "" || coverageReport(t, out) != coverageReport(t, java) {
		t.Fatalf("normal output: Go exit %d stderr %s; Java exit %d\nGo:\n%s\nJava:\n%s", code, stderr, exit, out, java)
	}
}

func TestCoverageSafetyEvaluationErrorEffectfulConstraintBoundary(t *testing.T) {
	dir := t.TempDir()
	body := "EXTENDS TLC, Naturals\nVARIABLES x,y\nInit == x = 0 /\\ y = 0\nA == /\\ x' = TRUE\n     /\\ y' = 1\nB == /\\ x' = FALSE\n     /\\ y' = 1\nNext == A \\/ B\nInv == x < 2\nLimit == Print(\"limit\", y < 1)"
	writeCoverageModule(t, dir, "M", body)
	writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\nINVARIANT Inv\nCONSTRAINT Limit\n")
	args := []string{"-tool", "-fp", "0", "-workers", "1", "-coverage", "9999", "-checkpoint", "0", "M.tla"}
	out, code, stderr := runGoInDir(t, dir, args)
	if code == 0 || len(coverageMessages(readToolMessages(out))) != 0 || !strings.Contains(out+stderr, "-coverage cannot report an effectful safety error recovery") {
		t.Fatalf("expected rejection without coverage: exit %d stderr %s\n%s", code, stderr, out)
	}
	if jar := pinnedFairnessEffectJar(t); jar != "" {
		javaArgs := append([]string{}, args[:len(args)-1]...)
		javaArgs = append(javaArgs, "-metadir", filepath.Join(t.TempDir(), "states"), "M.tla")
		java, exit := runJavaTLCArgs(t, jar, dir, javaArgs)
		if exit != 255 || !hasToolMessage(readToolMessages(java), 2111) || !hasToolMessage(readToolMessages(java), 2103) || strings.Count(java, `"limit"  FALSE`) < 2 {
			t.Fatalf("Java exit %d\n%s", exit, java)
		}
	}
}

func TestCoverageSuccessorExpressionErrorBoundary(t *testing.T) {
	dir := t.TempDir()
	body := "EXTENDS Naturals\nVARIABLE x\nInit == x = 2147483647\nA == x' = x + 1\nB == x' = x\nNext == A \\/ B"
	writeCoverageModule(t, dir, "M", body)
	writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\n")
	args := []string{"-tool", "-fp", "0", "-workers", "1", "-coverage", "9999", "-checkpoint", "0", "M.tla"}
	out, code, stderr := runGoInDir(t, dir, args)
	if code == 0 || len(coverageMessages(readToolMessages(out))) != 0 || !strings.Contains(out+stderr, "-coverage cannot report successor expression error replay") {
		t.Fatalf("expected rejection without coverage: exit %d stderr %s\n%s", code, stderr, out)
	}
	if jar := pinnedFairnessEffectJar(t); jar != "" {
		javaArgs := append([]string{}, args[:len(args)-1]...)
		javaArgs = append(javaArgs, "-metadir", filepath.Join(t.TempDir(), "states"), "M.tla")
		java, exit := runJavaTLCArgs(t, jar, dir, javaArgs)
		if exit != 255 || !hasToolMessage(readToolMessages(java), 2103) || !strings.Contains(java, "|line 5, col 11 to line 5, col 15 of module M: 2") {
			t.Fatalf("Java exit %d\n%s", exit, java)
		}
	}
}

func TestJavaTLCCoverageInitialInvariantEvaluationErrorReplay(t *testing.T) {
	dir := t.TempDir()
	body := "EXTENDS Naturals\nVARIABLE x\nInit == x = TRUE\nA == x' = FALSE\nB == x' = TRUE\nNext == A \\/ B\nInv == x < 2"
	writeCoverageModule(t, dir, "M", body)
	writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\nINVARIANT Inv\n")
	args := []string{"-tool", "-fp", "0", "-workers", "1", "-coverage", "9999", "-checkpoint", "0", "M.tla"}
	out, code, stderr := runGoInDir(t, dir, args)
	if code != 75 || stderr != "" || !hasToolMessage(readToolMessages(out), 1000) || !hasToolMessage(readToolMessages(out), 2103) || !strings.Contains(out, "<Init line 4, col 1 to line 4, col 4 of module M>: 2:2") {
		t.Fatalf("Go initial error: exit %d stderr %s\n%s", code, stderr, out)
	}
	if jar := pinnedFairnessEffectJar(t); jar != "" {
		javaArgs := append([]string{}, args[:len(args)-1]...)
		javaArgs = append(javaArgs, "-metadir", filepath.Join(t.TempDir(), "states"), "M.tla")
		java, exit := runJavaTLCArgs(t, jar, dir, javaArgs)
		if exit != code || !hasToolMessage(readToolMessages(java), 1000) || rawToolMessage(out, 2103) != rawToolMessage(java, 2103) || rawToolMessage(out, 2199) != rawToolMessage(java, 2199) || !reflect.DeepEqual(coverageMessages(readToolMessages(out)), coverageMessages(readToolMessages(java))) {
			t.Fatalf("Java exit %d\nGo:\n%s\nJava:\n%s", exit, out, java)
		}
	}
}
