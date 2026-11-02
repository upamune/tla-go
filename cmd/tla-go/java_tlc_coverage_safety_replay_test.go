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

// An evaluation exception also invokes TLC's error-stack replay. That path
// is separate from ordinary safety trace recovery and is not yet modeled.
func TestJavaTLCCoverageSafetyEvaluationErrorBoundary(t *testing.T) {
	dir := t.TempDir()
	body := "EXTENDS Naturals\nVARIABLE x\nInit == x = 0\nA == x' = TRUE\nB == x' = FALSE\nNext == A \\/ B\nInv == x < 2"
	writeCoverageModule(t, dir, "M", body)
	writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\nINVARIANT Inv\n")
	args := []string{"-tool", "-fp", "0", "-workers", "1", "-coverage", "9999", "-checkpoint", "0", "M.tla"}
	out, code, stderr := runGoInDir(t, dir, args)
	if code != 255 || stderr != "" || !hasToolMessage(readToolMessages(out), 2111) || !hasToolMessage(readToolMessages(out), 2103) || !strings.Contains(out, "<A line 5, col 1 to line 5, col 1 of module M>: 1:1") {
		t.Fatalf("Go evaluation failure: exit %d stderr %s\n%s", code, stderr, out)
	}
	if jar := pinnedFairnessEffectJar(t); jar != "" {
		javaArgs := append([]string{}, args[:len(args)-1]...)
		javaArgs = append(javaArgs, "-metadir", filepath.Join(t.TempDir(), "states"), "M.tla")
		java, exit := runJavaTLCArgs(t, jar, dir, javaArgs)
		if exit != 255 || !hasToolMessage(readToolMessages(java), 2111) || !hasToolMessage(readToolMessages(java), 2103) || !strings.Contains(java, "<A line 5, col 1 to line 5, col 1 of module M>: 1:3") || !strings.Contains(java, "<B line 6, col 1 to line 6, col 1 of module M>: 1:1") {
			t.Fatalf("Java evaluation failure: exit %d\n%s", exit, java)
		}
	}
}
