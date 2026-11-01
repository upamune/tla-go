package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These cases exercise TLC 1.7.4's getNextStates bounded-existential
// continuation, whose witnesses are all explored (unlike boolean evaluation).
func TestJavaTLCCoverageExistentialActions(t *testing.T) {
	for _, domain := range []string{"1..0", "0..0", "0..1", "1..3"} {
		for _, guard := range []string{"x < 0", "x < 1", "x < 2", "x < 3"} {
			name := strings.ReplaceAll(domain, "..", "-") + "_" + strings.ReplaceAll(strings.ReplaceAll(guard, " ", ""), "<", "lt")
			t.Run(name, func(t *testing.T) {
				source := fmt.Sprintf(`---- MODULE M ----
EXTENDS Naturals
VARIABLE x
Init == x \in 0..1
A == /\ %s
     /\ \E i \in %s : x' = i
B == x < 0 /\ UNCHANGED x
Limit == x < 3
Next == A \/ B
Inv == x \in Nat
====
`, guard, domain)
				path := filepath.Join("testdata", "java-tlc", "upstream-coverage", "exists-"+name+".coverage")
				golden, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				dir := t.TempDir()
				if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(source), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\nCONSTRAINT Limit\nINVARIANT Inv\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				args := []string{"-workers", "1", "-deadlock", "-coverage", "0", "M.tla"}
				out, code, stderr := runGoInDir(t, dir, args)
				if code != 0 || stderr != "" {
					t.Fatalf("exit %d stderr %s output %s", code, stderr, out)
				}
				if got := coverageReport(t, out); got != string(golden) {
					t.Errorf("Go coverage:\n%s\nwant:\n%s", got, golden)
				}
				if jar := os.Getenv("TLC_JAR"); jar != "" {
					java, javaCode := runJavaTLCArgs(t, jar, dir, args)
					if javaCode != 0 {
						t.Fatalf("Java exit %d: %s", javaCode, java)
					}
					if got := coverageReport(t, java); got != string(golden) {
						t.Errorf("Java coverage:\n%s\nwant:\n%s", got, golden)
					}
				}
			})
		}
	}
}

func TestCoverageRejectsUnmodeledExistentialActions(t *testing.T) {
	for _, action := range []string{
		`\E i \in 0..1 : x' = i`,
		`/\ x < 2 /\ \E i \in 0..1 : x' = i /\ x < 2`,
		`/\ x < 2 /\ \E i \in 0..1, j \in 0..1 : x' = i`,
		`/\ x < 2 /\ \E i \in 0..1 : /\ i < 1 /\ x' = i`,
		`/\ x < 2 /\ \E i \in 0..x : x' = i`,
	} {
		t.Run(action, func(t *testing.T) {
			dir := t.TempDir()
			source := fmt.Sprintf("---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nA == %s\nB == x < 0 /\\ UNCHANGED x\nNext == A \\/ B\n====\n", action)
			if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			_, code, stderr := runGoInDir(t, dir, []string{"-coverage", "0", "M.tla"})
			if code == 0 || !strings.Contains(stderr, "-coverage does not yet support") {
				t.Fatalf("exit %d stderr %s", code, stderr)
			}
		})
	}
}

func TestCoverageRejectsExistentialActionWithLiveness(t *testing.T) {
	dir := t.TempDir()
	source := `---- MODULE M ----
EXTENDS Naturals
VARIABLE x
Init == x = 0
A == /\ x < 2 /\ \E i \in 0..1 : x' = i
B == x < 0 /\ UNCHANGED x
Next == A \/ B
Live == []<>(x = 0)
====
`
	if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\nPROPERTY Live\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, code, stderr := runGoInDir(t, dir, []string{"-coverage", "0", "M.tla"})
	if code == 0 || !strings.Contains(stderr, "-coverage does not yet support") {
		t.Fatalf("exit %d stderr %s", code, stderr)
	}
}

func TestCoverageExistentialActionFailureReplay(t *testing.T) {
	for _, tc := range []struct {
		name, invariant, guard string
		omit, otherAction      bool
	}{
		{"invariant", "INVARIANT Inv\n", "x < 2", true, false},
		{"deadlock", "", "x < 2", true, false},
		{"deadlock before existential", "", "x < 0", false, false},
		{"deadlock after other action", "", "x < 0", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := fmt.Sprintf(`---- MODULE M ----
EXTENDS Naturals
VARIABLE x
Init == x \in 0..1
A == /\ %s
     /\ \E i \in 1..2 : x' = i
B == x < 0 /\ UNCHANGED x
Next == A \/ B
Inv == x < 2
====
`, tc.guard)
			if tc.otherAction {
				source = strings.Replace(source, `Init == x \in 0..1`, `Init == x \in 0..0`, 1)
				source = strings.Replace(source, `B == x < 0 /\ UNCHANGED x`, `B == x < 1 /\ x' = x + 1`, 1)
			}
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\n"+tc.invariant), 0o600); err != nil {
				t.Fatal(err)
			}
			out, code, stderr := runGoInDir(t, dir, []string{"-workers", "1", "-coverage", "0", "M.tla"})
			if code == 0 || stderr != "" || strings.Contains(out, "End of statistics.") == tc.omit {
				t.Fatalf("exit %d stderr %s output %s", code, stderr, out)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, []string{"-workers", "1", "-coverage", "0", "M.tla"})
				if javaCode == 0 {
					t.Fatalf("Java unexpectedly succeeded: %s", java)
				}
				javaCoverage := coverageReport(t, java)
				if !tc.omit && coverageReport(t, out) != javaCoverage {
					t.Errorf("Go and Java coverage differ:\n%s\n%s", out, java)
				}
				if code != javaCode {
					t.Fatalf("Go exit %d, Java exit %d:\n%s", code, javaCode, java)
				}
				compareCoverageFailureOutput(t, out, java)
			}
		})
	}
}

func TestJavaTLCCoverageExistentialEarlyFailures(t *testing.T) {
	for _, tc := range []struct {
		name, assumption, init, invariant string
		code                              int
		diagnostic                        string
	}{
		{"false assumption", "ASSUME FALSE", `x \in 0..1`, "", 10, "is false."},
		{"assumption error", `ASSUME 1 < TRUE`, `x \in 0..1`, "", 10, "Evaluating assumption"},
		{"initial invariant", "", `x \in 0..1`, `x < 0`, 12, "violated by the initial state"},
		{"initial invariant error", "", `x = C`, `x < 0`, 75, "first argument"},
		{"initial error", "", `/\ x = C
        /\ x < 1`, "", 75, "first argument"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := fmt.Sprintf("---- MODULE M ----\nEXTENDS Naturals\nCONSTANT C\nVARIABLE x\n%s\nInit == %s\nA == /\\ x < 2\n     /\\ \\E i \\in 1..2 : x' = i\nB == x < 0 /\\ UNCHANGED x\nNext == A \\/ B\nInv == %s\n====\n", tc.assumption, tc.init, tc.invariant)
			if tc.invariant == "" {
				source = strings.Replace(source, "Inv == \n", "Inv == TRUE\n", 1)
			}
			cfg := "CONSTANT C = TRUE\nINIT Init\nNEXT Next\n"
			if tc.invariant != "" {
				cfg += "INVARIANT Inv\n"
			}
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte(cfg), 0o600); err != nil {
				t.Fatal(err)
			}
			args := []string{"-workers", "1", "-coverage", "0", "M.tla"}
			out, code, stderr := runGoInDir(t, dir, args)
			if code != tc.code || stderr != "" || !strings.Contains(out, tc.diagnostic) || strings.Contains(out, "End of statistics.") {
				t.Fatalf("exit %d stderr %s output:\n%s", code, stderr, out)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				if javaCode != code || !strings.Contains(java, tc.diagnostic) {
					t.Fatalf("Java exit %d:\n%s", javaCode, java)
				}
				javaReport := strings.Contains(java, "End of statistics.")
				wantReport := tc.name == "initial error" || tc.name == "initial invariant error"
				if javaReport != wantReport {
					t.Fatalf("Unexpected coverage behavior:\nGo:\n%s\nJava:\n%s", out, java)
				}
				compareCoverageFailureOutput(t, out, java)
			}
		})
	}
}

// Compare the diagnostic and statistics after excluding Java's runtime banner,
// clock-dependent lines, and the coverage report when replay is unsupported.
func compareCoverageFailureOutput(t *testing.T, goOutput, javaOutput string) {
	t.Helper()
	clean := func(output string) string {
		start := strings.Index(output, "Error:")
		if start < 0 {
			t.Fatalf("missing diagnostic: %s", output)
		}
		output = output[start:]
		if end := strings.Index(output, "Finished in "); end >= 0 {
			output = output[:end]
		}
		if start := strings.Index(output, "The coverage statistics at "); start >= 0 {
			end := strings.Index(output[start:], "End of statistics.")
			if end < 0 {
				t.Fatalf("incomplete coverage report: %s", output)
			}
			output = output[:start] + output[start+end+len("End of statistics.\n"):]
		}
		return strings.TrimSpace(output)
	}
	if got, want := clean(goOutput), clean(javaOutput); got != want {
		t.Errorf("Go output:\n%s\nJava output:\n%s", got, want)
	}
}
