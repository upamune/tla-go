package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// I!Act and J!Act have the same expression range and separate action roots
// in Java, including when the substitutions use equal values. These CLI
// comparisons do not execute the upstream JUnit lifecycle.
func TestJavaTLCCoverageDistinctInstanceOriginsBoundary(t *testing.T) {
	for _, tc := range []struct{ name, value, snapshot, stats string }{
		{"different-values", "2", "origin-distinct-instances.coverage", "7 states generated, 3 distinct states found"},
		{"same-value", "1", "origin-identical-instances.coverage", "5 states generated, 2 distinct states found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checkCoverageOriginBoundary(t, map[string]string{
				"Inner.tla": "---- MODULE Inner ----\nCONSTANT c\nVARIABLE x\nAct == x' = c\n====\n",
				"M.tla":     "---- MODULE M ----\nVARIABLE x\nInit == x=0\nI == INSTANCE Inner WITH c <- 1, x <- x\nJ == INSTANCE Inner WITH c <- " + tc.value + ", x <- x\nNext == I!Act \\/ J!Act\n====\n",
			}, tc.snapshot, tc.stats)
		})
	}
}

// Both formal uses expand to the same caller expression pointer today. Java
// still reports separate source children for those uses under one action.
func TestJavaTLCCoverageSharedArgumentOriginsBoundary(t *testing.T) {
	checkCoverageOriginBoundary(t, map[string]string{
		"M.tla": "---- MODULE M ----\nVARIABLE x\nInit == x = 0\nF(n) == /\\ n=n\n        /\\ x' = 1\nB == x' = 0\nNext == F(x) \\/ B\n====\n",
	}, "origin-shared-argument.coverage", "5 states generated, 2 distinct states found")
}

func checkCoverageOriginBoundary(t *testing.T, files map[string]string, snapshot, statistics string) {
	t.Helper()
	fixture := coverageAFixtures(t)
	dir := t.TempDir()
	files["M.cfg"] = "INIT Init\nNEXT Next\n"
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	args := coverageAArgs("M.tla", true)
	out, code, stderr := runGoInDir(t, dir, args)
	if code == 0 || !strings.Contains(out+stderr, "-coverage does not yet support") {
		t.Fatalf("coverage should remain rejected: exit %d stderr %s output %s", code, stderr, out)
	}
	out, code, stderr = runGoInDir(t, dir, coverageAArgs("M.tla", false))
	if code != 0 || stderr != "" || !strings.Contains(out, statistics) {
		t.Fatalf("default: exit %d stderr %s output %s", code, stderr, out)
	}
	checkJavaACoverage(t, fixture, dir, args, snapshot, statistics)
}
