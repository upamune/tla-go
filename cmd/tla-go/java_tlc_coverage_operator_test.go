package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Complete Java TLC 1.7.4 reports are retained even for rejected call trees.
func TestJavaTLCCoverageOperators(t *testing.T) {
	for _, tc := range []struct {
		name    string
		reject  bool
		failure bool
	}{
		{name: "parameter"},
		{name: "empty"},
		{name: "nullary"},
		{name: "contexts"},
		{name: "short_circuit"},
		{name: "nested_short_circuit"},
		{name: "failure", failure: true},
		{name: "repeated", reject: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := filepath.Join("testdata", "java-tlc", "operator-coverage", tc.name)
			dir := t.TempDir()
			for _, name := range []string{"M.tla", "M.cfg"} {
				data, err := os.ReadFile(filepath.Join(fixture, name))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var golden []byte
			if !tc.failure {
				var err error
				golden, err = os.ReadFile(filepath.Join(fixture, "M.coverage"))
				if err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"-workers", "1", "-coverage", "0", "M.tla"}
			out, code, stderr := runGoInDir(t, dir, args)
			if tc.reject {
				if code == 0 || !strings.Contains(stderr, "-coverage does not yet support") {
					t.Fatalf("exit %d stderr %s", code, stderr)
				}
			} else {
				if (code != 0) != tc.failure || stderr != "" {
					t.Fatalf("exit %d stderr %s output %s", code, stderr, out)
				}
				if tc.failure {
					if strings.Contains(out, "The coverage statistics at ") {
						t.Errorf("unexpected Go coverage: %s", out)
					}
				} else if got := coverageReport(t, out); got != string(golden) {
					t.Errorf("Go coverage:\n%s\nwant:\n%s", got, golden)
				}
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				if (javaCode != 0) != tc.failure {
					t.Fatalf("Java exit %d: %s", javaCode, java)
				}
				if tc.failure {
					if strings.Contains(java, "The coverage statistics at ") {
						t.Errorf("unexpected Java coverage: %s", java)
					}
				} else if got := coverageReport(t, java); got != string(golden) {
					t.Errorf("Java coverage:\n%s\nwant:\n%s", got, golden)
				}
				if tc.name == "nested_short_circuit" {
					goTool, _, _ := runGoInDir(t, dir, append([]string{"-tool"}, args...))
					javaTool, _ := runJavaTLCArgs(t, jar, dir, append([]string{"-tool"}, args...))
					if got, want := coverageMessages(readToolMessages(goTool)), coverageMessages(readToolMessages(javaTool)); fmt.Sprint(got) != fmt.Sprint(want) {
						t.Errorf("tool messages differ:\n%#v\n%#v", got, want)
					}
				}
			}
		})
	}
}

func TestCoverageRejectsUnmodeledOperatorCalls(t *testing.T) {
	for _, tc := range []struct{ name, definitions, invariant, action string }{
		{"state argument", `F(n) == n < 3`, `F(x)`, `x < 0 /\ UNCHANGED x`},
		{"repeated nested", "F(n) == n < 3\nG(n) == F(1)", `/\ F(2) /\ G(1)`, `x < 0 /\ UNCHANGED x`},
		{"constant nullary", `H == 1 < 2`, `/\ x < 2 /\ H`, `x < 0 /\ UNCHANGED x`},
		{"action call", `F(n) == n < 3`, `x < 2`, `/\ x < 2 /\ F(1) /\ UNCHANGED x`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			src := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nA == " + tc.action + "\nB == x < 0 /\\ UNCHANGED x\nNext == A \\/ B\n" + tc.definitions + "\nInv == " + tc.invariant + "\n====\n"
			if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(src), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\nINVARIANT Inv\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			_, code, stderr := runGoInDir(t, dir, []string{"-coverage", "0", "M.tla"})
			if code == 0 || !strings.Contains(stderr, "-coverage does not yet support") {
				t.Fatalf("exit %d stderr %s", code, stderr)
			}
		})
	}
}
