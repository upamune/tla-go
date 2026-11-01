package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TLC 1.7.4 at 5a47802b5c391f59ecdd44117981f4ff8c0656ba.
func TestJavaTLCCoverageSubsetInvariant(t *testing.T) {
	for _, tc := range []struct {
		name       string
		n          int
		predicate  string
		quantifier string
		fail       bool
	}{
		{"empty", 0, `f \in 1..N`, `\A`, false},
		{"two", 2, `f \in 1..N`, `\A`, false},
		{"three", 3, `f \in 1..N`, `\A`, false},
		{"failure", 2, `f < 1`, `\A`, true},
		{"short-circuit", 3, `f \in 1..N`, `\E`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := fmt.Sprintf("---- MODULE M ----\nEXTENDS Naturals, FiniteSets\nVARIABLE x\nInit == x \\in 0..1\nA == x' = x\nB == x' = x\nNext == A \\/ B\nExp(N) == %s e \\in SUBSET (1..N) : %s f \\in e : %s\nInv == /\\ x \\in Nat\n       /\\ Exp(%d)\n====\n", tc.quantifier, tc.quantifier, tc.predicate, tc.n)
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\nINVARIANT Inv\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			var golden []byte
			var err error
			if !tc.fail {
				golden, err = os.ReadFile(filepath.Join("testdata", "java-tlc", "upstream-coverage", "subset-"+tc.name+".coverage"))
			}
			if err != nil {
				t.Fatal(err)
			}
			args := []string{"-workers", "1", "-coverage", "0", "M.tla"}
			output, code, stderr := runGoInDir(t, dir, args)
			if (code != 0) != tc.fail || stderr != "" {
				t.Fatalf("Go exit %d stderr %s output %s", code, stderr, output)
			}
			if tc.fail && strings.Contains(output, "The coverage statistics at ") {
				t.Errorf("unexpected Go coverage: %s", output)
			}
			if !tc.fail {
				if got := coverageReport(t, output); got != string(golden) {
					t.Errorf("Go coverage:\n%s\nwant:\n%s", got, golden)
				}
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java, jcode := runJavaTLCArgs(t, jar, dir, args)
				if (jcode != 0) != tc.fail {
					t.Fatalf("Java exit %d: %s", jcode, java)
				}
				if tc.fail && strings.Contains(java, "The coverage statistics at ") {
					t.Errorf("unexpected Java coverage: %s", java)
				}
				if !tc.fail {
					if got := coverageReport(t, java); got != string(golden) {
						t.Errorf("Java coverage:\n%s\nwant:\n%s", got, golden)
					}
				}
				toolArgs := append([]string{"-tool"}, args...)
				goTool, _, _ := runGoInDir(t, dir, toolArgs)
				javaTool, _ := runJavaTLCArgs(t, jar, dir, toolArgs)
				if got, want := coverageMessages(readToolMessages(goTool)), coverageMessages(readToolMessages(javaTool)); fmt.Sprint(got) != fmt.Sprint(want) {
					t.Errorf("tool messages differ:\n%#v\n%#v", got, want)
				}
			}
		})
	}
}

func TestCoverageRejectsUnmodeledSubsetCosts(t *testing.T) {
	for _, body := range []string{
		`\A e \in SUBSET SUBSET (1..2) : e = e`,
		`\A e \in SUBSET (1..2) : Cardinality(e) = 0`,
		`\A e \in SUBSET {1,2} : e = e`,
		`\A e \in SUBSET (1..2) : e = e`,
	} {
		t.Run(body, func(t *testing.T) {
			dir := t.TempDir()
			source := fmt.Sprintf("---- MODULE M ----\nEXTENDS Naturals, FiniteSets\nVARIABLE x\nInit == x = 0\nA == x' = x\nB == x' = x\nNext == A \\/ B\nInv == /\\ x \\in Nat\n       /\\ %s\n====\n", body)
			if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(source), 0o600); err != nil {
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
