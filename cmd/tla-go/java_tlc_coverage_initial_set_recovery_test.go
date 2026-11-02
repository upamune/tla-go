package main

import (
	"strings"
	"testing"
)

func TestCoverageRejectsInitialLiteralSetWithTraceRecovery(t *testing.T) {
	const body = `EXTENDS Naturals
VARIABLES x, y
Init == /\ x = {}
        /\ y = 0
A == /\ y < 2
     /\ x' = x
     /\ y' = y + 1
B == /\ y < 2
     /\ x' = x
     /\ y' = y + 1
Next == A \/ B
Inv == y < 2
Always == TRUE`
	for _, tc := range []struct {
		name, config string
		exit, event  int
		numeric      bool
	}{
		{"invariant", "INVARIANTS Inv Always\nCHECK_DEADLOCK FALSE\n", 12, 2110, false},
		{"deadlock", "INVARIANT Always\n", 11, 2114, false},
		{"numeric-invariant", "INVARIANTS Inv Always\nCHECK_DEADLOCK FALSE\n", 12, 2110, true},
		{"numeric-deadlock", "INVARIANT Always\n", 11, 2114, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			model := body
			if tc.numeric {
				model = strings.Replace(body, "x = {}", "x = 0", 1)
			}
			writeCoverageModule(t, dir, "M", model)
			writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\n"+tc.config)
			args := []string{"-tool", "-fp", "0", "-workers", "1", "-coverage", "9999", "-checkpoint", "0", "M.tla"}
			out, code, stderr := runGoInDir(t, dir, args)
			if code == 0 || !strings.Contains(out+stderr, "-coverage does not yet support") || len(coverageMessages(readToolMessages(out))) != 0 {
				t.Fatalf("expected rejection without coverage: exit %d stderr %s\n%s", code, stderr, out)
			}
			if jar := pinnedFairnessEffectJar(t); jar != "" {
				java, exit := runJavaTLCArgs(t, jar, dir, args)
				if exit != tc.exit || !hasToolMessage(readToolMessages(java), tc.event) || !strings.Contains(java, "<Init line 4, col 1 to line 4, col 4 of module M>: 2:2") || !strings.Contains(java, "<A line 6, col 1 to line 6, col 1 of module M>: 2:4") {
					t.Fatalf("Java exit %d\n%s", exit, java)
				}
			}
		})
	}
}

func TestCoverageRejectsInitialLiteralSetOutsideUnchanged(t *testing.T) {
	for _, tc := range []struct{ name, body, config string }{
		{"parameterized", "VARIABLE x\nInit == x = {}\nA(n) == x' = n\nNext == A(1) \\/ A(2)", ""},
		{"parameterized-range", "EXTENDS Naturals\nVARIABLE x\nInit == x = {}\nA(n) == x' \\in 0..n\nNext == A(1) \\/ A(2)", ""},
		{"invariant", "VARIABLE x\nInit == x = {}\nNext == UNCHANGED x\nInv == x = {}", "INVARIANT Inv\n"},
		{"assumption", "ASSUME TRUE\nVARIABLE x\nInit == x = {}\nNext == UNCHANGED x", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeCoverageModule(t, dir, "M", tc.body)
			writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\n"+tc.config)
			out, code, stderr := runGoInDir(t, dir, coverageAArgs("M.tla", true))
			if code == 0 || !strings.Contains(out+stderr, "-coverage does not yet support") {
				t.Fatalf("expected rejection: exit %d stderr %s\n%s", code, stderr, out)
			}
		})
	}
}

func TestCoverageRejectsInitialLiteralSetWithGraph(t *testing.T) {
	dir := t.TempDir()
	writeCoverageModule(t, dir, "M", "VARIABLE x\nInit == x = {}\nNext == UNCHANGED x")
	writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\n")
	args := []string{"-coverage", "9999", "-dump", "dot", "graph.dot", "M.tla"}
	out, code, stderr := runGoInDir(t, dir, args)
	if code == 0 || !strings.Contains(out+stderr, "-coverage does not yet support") {
		t.Fatalf("expected rejection: exit %d stderr %s\n%s", code, stderr, out)
	}
}

func TestJavaTLCCoverageLiteralTrueInvariantWithUnchanged(t *testing.T) {
	dir := t.TempDir()
	writeCoverageModule(t, dir, "M", "VARIABLE x\nInit == x = 0\nNext == UNCHANGED x\nAlways == TRUE")
	writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\nINVARIANT Always\n")
	compareParameterizedCoverage(t, dir)
}

func TestJavaTLCCoverageStateInvariantWithUnchanged(t *testing.T) {
	dir := t.TempDir()
	writeCoverageModule(t, dir, "M", "VARIABLE x\nInit == x = 0\nNext == UNCHANGED x\nInv == x = 0")
	writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\nINVARIANT Inv\n")
	compareParameterizedCoverage(t, dir)
}

func TestCoverageRejectsLiteralTrueInvariantMembershipReplay(t *testing.T) {
	dir := t.TempDir()
	model := "EXTENDS Naturals\nVARIABLE x\nInit == x \\in {0,1}\nA == /\\ x < 2\n     /\\ x' = x + 1\nB == /\\ x < 2\n     /\\ x' = x + 1\nNext == A \\/ B\nAlways == TRUE"
	writeCoverageModule(t, dir, "M", model)
	writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\nINVARIANT Always\n")
	args := []string{"-tool", "-fp", "0", "-workers", "1", "-coverage", "9999", "-checkpoint", "0", "M.tla"}
	out, code, stderr := runGoInDir(t, dir, args)
	if code == 0 || !strings.Contains(out+stderr, "-coverage does not yet support") || len(coverageMessages(readToolMessages(out))) != 0 {
		t.Fatalf("expected rejection without coverage: exit %d stderr %s\n%s", code, stderr, out)
	}
	if jar := pinnedFairnessEffectJar(t); jar != "" {
		java, exit := runJavaTLCArgs(t, jar, dir, args)
		if exit != 11 || !hasToolMessage(readToolMessages(java), 2114) || !strings.Contains(java, "<Init line 4, col 1 to line 4, col 4 of module M>: 4:4") || !strings.Contains(java, "<A line 5, col 1 to line 5, col 1 of module M>: 1:3") {
			t.Fatalf("Java exit %d\n%s", exit, java)
		}
	}
}
