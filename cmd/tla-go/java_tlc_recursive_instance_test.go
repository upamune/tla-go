package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These models exercise Java TLC v1.7.4's instance substitution level checks.
func TestJavaTLCRecursiveInstance(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"test203.tla", "test203a.tla", "test203.cfg"} {
		data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{filepath.Join(dir, "test203.tla")}, &out, &errout)
	stats := "0 states generated, 0 distinct states found, 0 states left on queue."
	if code != 0 || errout.Len() != 0 || !strings.Contains(out.String(), stats) || !strings.Contains(out.String(), `"Test1 OK"`) {
		t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java := runJavaTLCForExit(t, jar, dir, "test203", 0)
		if !strings.Contains(java, stats) || !strings.Contains(java, `"Test1 OK"`) {
			t.Fatalf("Java TLC:\n%s", java)
		}
	}
}

func TestJavaTLCInstanceSubstitutionLevels(t *testing.T) {
	const scalar = "---- MODULE A ----\nCONSTANT C\nVARIABLE y\nPublic == C = y\n====\n"
	const scalarConstant = "---- MODULE A ----\nCONSTANT C\nH == C\n====\n"
	const operator = "---- MODULE A ----\nCONSTANT F(_)\nH == F(0)\n====\n"
	for _, tc := range []struct {
		name, module, definitions, invariant string
		goCode, javaCode                     int
	}{
		{"state substitution", scalar, "I == INSTANCE A WITH C <- x, y <- x", "I!Public", 255, 150},
		{"direct state", scalar, "P(p) == INSTANCE A WITH C <- p, y <- x", "P(x)!Public", 255, 150},
		{"nonrecursive state", scalar, "P(p) == INSTANCE A WITH C <- p, y <- x\nF(n) == P(n)!Public", "F(x)", 255, 150},
		{"nonrecursive constant", scalar, "P(p) == INSTANCE A WITH C <- p, y <- x\nF(n) == P(n)!Public", "F(0)", 0, 0},
		{"state bounded variable", scalar, "P(p) == INSTANCE A WITH C <- p, y <- x", "\\A i \\in {x} : P(i)!Public", 0, 0},
		{"recursive state dependency", scalar, "RECURSIVE g(_)\nP(p) == INSTANCE A WITH C <- p, y <- x\ng(n) == IF n = 0 THEN P(n)!Public ELSE g(n-1)\nWrapped(n) == g(n)", "Wrapped(x)", 255, 150},
		{"recursive constant", scalarConstant, "RECURSIVE g(_)\nP(p) == INSTANCE A WITH C <- p\ng(n) == IF n = 0 THEN P(n)!H ELSE g(n-1)\nWrapped(n) == g(n)", "Wrapped(x) = 0", 0, 0},
		{"stateful operator", operator, "G(n) == n + x\nI == INSTANCE A WITH F <- G", "I!H = x", 0, 0},
		{"stateful lambda", operator, "P(p) == INSTANCE A WITH F <- LAMBDA n : n + p", "P(x)!H = x", 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\n" + tc.definitions + "\nInit == x = 0\nNext == UNCHANGED x\nInv == " + tc.invariant + "\n====\n"
			for name, contents := range map[string]string{"M.tla": source, "A.tla": tc.module, "M.cfg": "INIT Init\nNEXT Next\nINVARIANT Inv\n"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != tc.goCode {
				t.Fatalf("exit %d, want %d, stdout:\n%s\nstderr:\n%s", code, tc.goCode, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				runJavaTLCForExit(t, jar, dir, "M", tc.javaCode)
			}
		})
	}
}
