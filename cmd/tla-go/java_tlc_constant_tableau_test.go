package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Java TLC 1.7.4's tableau omits Boolean constants from its state predicates.
// This causes it to report violations for some mathematically true formulas.
// TLC 1.8.0 corrected the omission; the pinned 1.7.4 behavior is the target.
func TestJavaTLCConstantTableau17(t *testing.T) {
	for _, tc := range []struct {
		name, property, init, fairness string
		java17, java18                 int
	}{
		{"eventually true", "<>TRUE", "x = 0", "", 13, 0},
		{"eventually equality", "<>(1 = 1)", "x = 0", "", 13, 0},
		{"eventually comparison", "<>(2 > 1)", "x = 0", "", 13, 0},
		{"eventually not false", "<>~FALSE", "x = 0", "", 13, 0},
		{"eventually conjunction false", `<>(TRUE /\ FALSE)`, "x = 0", "", 13, 13},
		{"not eventually false", "~<>FALSE", "x = 0", "", 13, 0},
		{"not always false", "~[]FALSE", "x = 0", "", 13, 0},
		{"nested always true", "[][]TRUE", "x = 0", "", 13, 0},
		{"nested eventually true", "<><>TRUE", "x = 0", "", 13, 0},
		{"nested recurrence true", "[]<>[]TRUE", "x = 0", "", 13, 0},
		{"nested stability true", "<>[]<>TRUE", "x = 0", "", 13, 0},
		{"tableau conjunction", `(<>TRUE) \/ (<>TRUE)`, "x = 0", "", 13, 0},
		{"recurrence with tableau", `([]<>TRUE) \/ (<>TRUE)`, "x = 0", "", 0, 0},
		{"weak fairness", "<>TRUE", "x = 0", "WF_x(Next)", 13, 0},
		{"strong fairness", "<>TRUE", "x = 0", "SF_x(Next)", 13, 0},
		{"state recurrence fairness", "<>TRUE", "x = 0", "[]<>(x = 0)", 13, 0},
		{"unsatisfiable fairness", "<>TRUE", "x = 0", `SF_x(Next) /\ []<>(x = 4)`, 0, 0},
		{"multiple initial states", "<>TRUE", `x \in {0, 1}`, "", 13, 0},
		{"bound constant", `\A i \in {0, 1} : <>(i = i)`, "x = 0", "", 13, 0},
		{"no initial states", "<>TRUE", `x \in {}`, "", 0, 0},
		{"state-dependent tautology", "<>(x = x)", "x = 0", "", 0, 0},
		{"safety true", "[]TRUE", "x = 0", "", 0, 0},
		{"safety false", "[]FALSE", "x = 0", "", 12, 12},
		{"recurrence true", "[]<>TRUE", "x = 0", "", 0, 0},
		{"stability true", "<>[]TRUE", "x = 0", "", 0, 0},
		{"eventually false", "<>FALSE", "x = 0", "", 13, 13},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == " + tc.init + "\nNext == x' = (x + 1) % 3\nP == " + tc.property + "\n"
			cfg := "INIT Init\nNEXT Next\nPROPERTY P\nCHECK_DEADLOCK FALSE\n"
			if tc.fairness != "" {
				body += "Spec == Init /\\ [][Next]_x /\\ " + tc.fairness + "\n"
				cfg = "SPECIFICATION Spec\nPROPERTY P\nCHECK_DEADLOCK FALSE\n"
			}
			writeDOTFixture(t, dir, body, cfg)
			var out, stderr bytes.Buffer
			code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", filepath.Join(dir, "M.tla")}, &out, &stderr)
			if code != tc.java17 || stderr.Len() != 0 {
				t.Fatalf("Go exit %d, want %d: %s %s", code, tc.java17, &out, &stderr)
			}
			if code == 13 && !strings.Contains(out.String(), "Temporal properties were violated.") {
				t.Fatalf("Go did not report a temporal violation: %s", &out)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runJavaTLCForExit(t, jar, dir, "M", tc.java17)
				if tc.name == "eventually true" {
					for _, fragment := range []string{"1: <Initial predicate>\nx = 0", "2: Stuttering"} {
						if !strings.Contains(java, fragment) || !strings.Contains(out.String(), fragment) {
							t.Fatalf("missing lasso fragment %q; Java: %s; Go: %s", fragment, java, &out)
						}
					}
				}
			}
			if jar := os.Getenv("TLC_MODERN_JAR"); jar != "" {
				runJavaTLCForExit(t, jar, dir, "M", tc.java18)
			}
		})
	}
}

func TestJavaTLCConstantTableauConfiguredConstant(t *testing.T) {
	dir := t.TempDir()
	body := "CONSTANT Flag\nVARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 3\nP == <>Flag"
	cfg := "CONSTANT Flag = TRUE\nINIT Init\nNEXT Next\nPROPERTY P\nCHECK_DEADLOCK FALSE\n"
	writeDOTFixture(t, dir, body, cfg)
	var out, stderr bytes.Buffer
	code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", filepath.Join(dir, "M.tla")}, &out, &stderr)
	if code != 13 || stderr.Len() != 0 || !strings.Contains(out.String(), "Temporal properties were violated.") {
		t.Fatalf("Go exit %d: %s %s", code, &out, &stderr)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		runJavaTLCForExit(t, jar, dir, "M", 13)
	}
	if jar := os.Getenv("TLC_MODERN_JAR"); jar != "" {
		runJavaTLCForExit(t, jar, dir, "M", 0)
	}
}

func TestJavaTLCConstantTableauMultipleProperties(t *testing.T) {
	for _, tc := range []struct {
		name, first, second string
		java17, java18      int
	}{
		{"two eventualities", "<>TRUE", "<>FALSE", 13, 13},
		{"recurrence then eventuality", "[]<>TRUE", "<>TRUE", 13, 0},
		{"safety violation first", "<>TRUE", "[](x = 0)", 12, 12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 3\nP == " + tc.first + "\nQ == " + tc.second
			cfg := "INIT Init\nNEXT Next\nPROPERTY P\nPROPERTY Q\nCHECK_DEADLOCK FALSE\n"
			writeDOTFixture(t, dir, body, cfg)
			var out, stderr bytes.Buffer
			code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", filepath.Join(dir, "M.tla")}, &out, &stderr)
			if code != tc.java17 || stderr.Len() != 0 {
				t.Fatalf("Go exit %d, want %d: %s %s", code, tc.java17, &out, &stderr)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				runJavaTLCForExit(t, jar, dir, "M", tc.java17)
			}
			if jar := os.Getenv("TLC_MODERN_JAR"); jar != "" {
				runJavaTLCForExit(t, jar, dir, "M", tc.java18)
			}
		})
	}
}
