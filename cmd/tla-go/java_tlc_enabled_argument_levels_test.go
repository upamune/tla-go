package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// test57 and test57a are unchanged models from Java TLC v1.7.4.
func TestJavaTLCEnabledArgumentInstance(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"test57.tla", "test57a.tla", "test57.cfg"} {
		data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want := "Error: Invariant Invariant1 is violated by the initial state:\nx = 1"
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{filepath.Join(dir, "test57.tla")}, &out, &errout)
	if code != 12 || errout.Len() != 0 || !strings.Contains(out.String(), want) {
		t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java := runJavaTLCForExit(t, jar, dir, "test57", 12)
		if !strings.Contains(java, want) {
			t.Fatalf("Java TLC:\n%s", java)
		}
	}
}

func TestJavaTLCEnabledArgumentLevels(t *testing.T) {
	for _, tc := range []struct{ name, definition, invariant string }{
		{"enabled", "B(d) == ENABLED d", "B(Act)"},
		{"ignored", "B(d) == TRUE", "B(Act)"},
		{"conjunction", `B(d) == /\ ENABLED d /\ x = 0`, "B(Act)"},
		{"two arguments", `B(a, b) == ENABLED a /\ b`, "B(Act, x = 0)"},
		{"forwarded", "C(e) == ENABLED e\nB(d) == C(d)", "B(Act)"},
		{"operator argument", "A(i) == x' = i\nB(F(_)) == ENABLED F(0)", "B(A)"},
		{"operator and value", "A(i) == x' = i\nB(F(_), n) == ENABLED F(n)", "B(A, x)"},
		{"forwarded operator", "A(i) == x' = i\nB(F(_)) == ENABLED F(0)\nC(G(_)) == B(G)", "C(A)"},
		{"lambda operator", "B(F(_)) == ENABLED F(0)", `B(LAMBDA i : x' = i)`},
		{"lambda and value", "B(F(_), n) == ENABLED F(n)", `B(LAMBDA i : x' = i, x)`},
		{"bound name collision", `B(d) == \E n \in {1} : ENABLED d`, `\A n \in {1} : B(x' = n)`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\nAct == x' = x\n" + tc.definition + "\nInv == " + tc.invariant + "\n====\n"
			for name, contents := range map[string]string{"M.tla": source, "M.cfg": "INIT Init\nNEXT Next\nINVARIANT Inv\n"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			stats := "2 states generated, 1 distinct states found, 0 states left on queue."
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != 0 || errout.Len() != 0 || !strings.Contains(out.String(), stats) {
				t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runJavaTLCForExit(t, jar, dir, "M", 0)
				if !strings.Contains(java, stats) {
					t.Fatalf("Java TLC:\n%s", java)
				}
			}
		})
	}
}
