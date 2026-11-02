package main

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestJavaTLCCoverageInitialConstraintCounts(t *testing.T) {
	for _, tc := range []struct {
		name, init, constraint, invariant string
		exit                              int
	}{
		{"some", "x \\in 0..1", "x > 0", "x < 2", 12},
		{"all", "x \\in 0..1", "x >= 0", "x < 2", 12},
		{"none", "x \\in 0..1", "x < 0", "", 0},
		{"empty", "x \\in 1..0", "x > 0", "", 0},
		{"no-constraint", "x \\in 0..1", "", "x < 2", 12},
		{"duplicate-set", "x \\in {0,0,1}", "", "x < 2", 12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "EXTENDS Naturals\nVARIABLE x\nInit == " + tc.init + "\nA == /\\ x < 3\n     /\\ x' = x + 1\nB == /\\ x < 3\n     /\\ x' = x + 1\nNext == A \\/ B"
			cfg := "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n"
			if tc.constraint != "" {
				body += "\nLimit == " + tc.constraint
				cfg += "CONSTRAINT Limit\n"
			}
			if tc.invariant != "" {
				body += "\nInv == " + tc.invariant
				cfg += "INVARIANT Inv\n"
			}
			writeCoverageModule(t, dir, "M", body)
			writeParameterizedConfig(t, dir, cfg)
			compareCoverageInitialConstraint(t, dir, tc.exit)
		})
	}
}

func compareCoverageInitialConstraint(t *testing.T, dir string, want int) {
	t.Helper()
	args := []string{"-tool", "-fp", "0", "-workers", "1", "-coverage", "9999", "-checkpoint", "0", "M.tla"}
	out, code, stderr := runGoInDir(t, dir, args)
	if code != want || stderr != "" {
		t.Fatalf("Go exit %d stderr %s\n%s", code, stderr, out)
	}
	if jar := pinnedFairnessEffectJar(t); jar != "" {
		javaArgs := append([]string{}, args[:len(args)-1]...)
		javaArgs = append(javaArgs, "-metadir", filepath.Join(t.TempDir(), "states"), "M.tla")
		java, exit := runJavaTLCArgs(t, jar, dir, javaArgs)
		if exit != code || !reflect.DeepEqual(coverageMessages(readToolMessages(out)), coverageMessages(readToolMessages(java))) || rawToolMessage(out, 2199) != rawToolMessage(java, 2199) {
			t.Fatalf("Java exit %d\nGo:\n%s\nJava:\n%s", exit, out, java)
		}
	}
}

func TestCoverageRejectsMultipleInitialActions(t *testing.T) {
	dir := t.TempDir()
	body := "VARIABLE x\nI == x = 0\nInit == I \\/ I\nA == x' = 0\nB == x' = 0\nNext == A \\/ B"
	writeCoverageModule(t, dir, "M", body)
	writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\n")
	args := []string{"-tool", "-deadlock", "-fp", "0", "-workers", "1", "-coverage", "9999", "-checkpoint", "0", "M.tla"}
	out, code, stderr := runGoInDir(t, dir, args)
	if code == 0 || !strings.Contains(out+stderr, "-coverage does not yet support") {
		t.Fatalf("expected rejection: exit %d stderr %s\n%s", code, stderr, out)
	}
	if jar := pinnedFairnessEffectJar(t); jar != "" {
		javaArgs := append([]string{}, args[:len(args)-1]...)
		javaArgs = append(javaArgs, "-metadir", filepath.Join(t.TempDir(), "states"), "M.tla")
		java, exit := runJavaTLCArgs(t, jar, dir, javaArgs)
		if exit != 0 || !hasToolMessage(readToolMessages(java), 2201) || !hasToolMessage(readToolMessages(java), 2199) {
			t.Fatalf("Java exit %d\n%s", exit, java)
		}
	}
}

func TestJavaTLCCoverageInitialConstraintEffects(t *testing.T) {
	dir := t.TempDir()
	body := "EXTENDS TLC, Naturals\nVARIABLE x\nInit == x \\in 0..1\nA == /\\ x < 3\n     /\\ x' = x + 1\nB == /\\ x < 3\n     /\\ x' = x + 1\nNext == A \\/ B\nInv == x < 2\nLimit == Print(\"constraint\", x > 0)"
	writeCoverageModule(t, dir, "M", body)
	writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\nINVARIANT Inv\nCONSTRAINT Limit\nCHECK_DEADLOCK FALSE\n")
	args := []string{"-tool", "-fp", "0", "-workers", "1", "-coverage", "9999", "-checkpoint", "0", "M.tla"}
	out, code, stderr := runGoInDir(t, dir, args)
	if code != 12 || stderr != "" {
		t.Fatalf("Go exit %d stderr %s\n%s", code, stderr, out)
	}
	if jar := pinnedFairnessEffectJar(t); jar != "" {
		javaArgs := append([]string{}, args[:len(args)-1]...)
		javaArgs = append(javaArgs, "-metadir", filepath.Join(t.TempDir(), "states"), "M.tla")
		java, exit := runJavaTLCArgs(t, jar, dir, javaArgs)
		if exit != code || !reflect.DeepEqual(coverageMessages(readToolMessages(out)), coverageMessages(readToolMessages(java))) || !reflect.DeepEqual(coverageConstraintEffectLines(out), coverageConstraintEffectLines(java)) || !coverageConstraintEffectsBeforeError(out) || !coverageConstraintEffectsBeforeError(java) {
			t.Fatalf("Java exit %d\nGo:\n%s\nJava:\n%s", exit, out, java)
		}
	}
}

func coverageConstraintEffectsBeforeError(output string) bool {
	lastPrint := strings.LastIndex(output, `"constraint"  `)
	heading := strings.Index(output, "@!@!@STARTMSG 2110:")
	return lastPrint >= 0 && heading > lastPrint
}

func coverageConstraintEffectLines(output string) []string {
	var lines []string
	for line := range strings.SplitSeq(output, "\n") {
		if strings.HasPrefix(line, `"constraint"  `) {
			lines = append(lines, line)
		}
	}
	return lines
}

// Init constraint errors still lack TLC's separate initial-state stack replay.
func TestCoverageInitialConstraintEvaluationErrorBoundary(t *testing.T) {
	dir := t.TempDir()
	body := "EXTENDS Naturals\nVARIABLE x\nInit == x \\in 0..1\nA == x' = 0\nB == x' = 1\nNext == A \\/ B\nLimit == IF x = 0 THEN TRUE ELSE x < TRUE"
	writeCoverageModule(t, dir, "M", body)
	writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\nCONSTRAINT Limit\n")
	args := []string{"-tool", "-fp", "0", "-workers", "1", "-coverage", "9999", "-checkpoint", "0", "M.tla"}
	out, code, stderr := runGoInDir(t, dir, args)
	if code != 75 || stderr != "" || !hasToolMessage(readToolMessages(out), 2103) || !strings.Contains(out, "<Init line 4, col 1 to line 4, col 4 of module M>: 2:2") {
		t.Fatalf("Go exit %d stderr %s\n%s", code, stderr, out)
	}
	if jar := pinnedFairnessEffectJar(t); jar != "" {
		javaArgs := append([]string{}, args[:len(args)-1]...)
		javaArgs = append(javaArgs, "-metadir", filepath.Join(t.TempDir(), "states"), "M.tla")
		java, exit := runJavaTLCArgs(t, jar, dir, javaArgs)
		if exit != code || rawToolMessage(out, 2103) != rawToolMessage(java, 2103) || rawToolMessage(out, 2199) != rawToolMessage(java, 2199) || !strings.Contains(java, "<Init line 4, col 1 to line 4, col 4 of module M>: 4:4") {
			t.Fatalf("Java exit %d\nGo:\n%s\nJava:\n%s", exit, out, java)
		}
	}
}
