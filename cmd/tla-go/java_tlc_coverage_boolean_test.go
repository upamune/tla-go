package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// BCoverageTest at upstream commit 5a47802b asserts the full coverage report,
// search depth 2, and 5 generated / 2 distinct states with coverage enabled.
func TestJavaTLCUpstreamBooleanCoverage(t *testing.T) {
	base := filepath.Join("testdata", "java-tlc", "upstream-coverage")
	source, err := os.ReadFile(filepath.Join(base, "B.tla"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join(base, "B.coverage"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := os.ReadFile(filepath.Join(base, "B.cfg"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, old, replacement string }{
		{"upstream", "", ""},
		{"true-initial", "Init == x = FALSE", "Init == x = TRUE"},
		{"reversed-actions", "Next == A \\/ B", "Next == B \\/ A"},
		{"different-operators", "B == x' = Switch(x)", "Flip(v) == ~v\nB == x' = Flip(x)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text := string(source)
			if tc.old != "" {
				if !strings.Contains(text, tc.old) {
					t.Fatal("fixture to replace is missing")
				}
				text = strings.Replace(text, tc.old, tc.replacement, 1)
			}
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "B.tla"), []byte(text), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "B.cfg"), cfg, 0o600); err != nil {
				t.Fatal(err)
			}
			args := []string{"-workers", "1", "-coverage", "9999", "B.tla"}
			out, code, stderr := runGoInDir(t, dir, args)
			if code != 0 || stderr != "" || !strings.Contains(out, "5 states generated, 2 distinct states found, 0 states left on queue.") || !strings.Contains(out, "The depth of the complete state graph search is 2.") {
				t.Fatalf("Go exit %d stderr %s output %s", code, stderr, out)
			}
			if tc.name == "upstream" && coverageReport(t, out) != string(want) {
				t.Fatalf("coverage differs:\n%s", coverageReport(t, out))
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareBooleanCoverageOracle(t, jar, dir, args, out)
			}
		})
	}
}

func compareBooleanCoverageOracle(t *testing.T, jar, dir string, args []string, output string) {
	t.Helper()
	java, code := runJavaTLCArgs(t, jar, dir, args)
	if code != 0 || coverageReport(t, output) != coverageReport(t, java) {
		t.Fatalf("Java exit %d; Go/Java coverage:\n%s\n%s", code, output, java)
	}
	toolArgs := append([]string{"-tool"}, args...)
	goTool, goCode, stderr := runGoInDir(t, dir, toolArgs)
	javaTool, javaCode := runJavaTLCArgs(t, jar, dir, toolArgs)
	if goCode != 0 || javaCode != 0 || stderr != "" || !reflect.DeepEqual(coverageMessages(readToolMessages(goTool)), coverageMessages(readToolMessages(javaTool))) {
		t.Fatalf("tool coverage differs; Go exit %d Java exit %d stderr %s\n%s\n%s", goCode, javaCode, stderr, goTool, javaTool)
	}
}

func TestJavaTLCCoverageReportsActionsInSourceOrder(t *testing.T) {
	base := filepath.Join("testdata", "java-tlc", "upstream-coverage")
	source, err := os.ReadFile(filepath.Join(base, "CoverageStatistics.tla"))
	if err != nil {
		t.Fatal(err)
	}
	model := strings.Replace(string(source), "Next == A \\/ B", "Next == B \\/ A", 1)
	model = strings.Replace(model, "=============================================================================", "Inv == x \\in Nat\n=============================================================================", 1)
	if model == string(source) || !strings.Contains(model, "Next == B \\/ A") {
		t.Fatal("upstream fixture changed")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "CoverageStatistics.tla"), []byte(model), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "CoverageStatistics.cfg"), []byte("SPECIFICATION Spec\nCONSTRAINT Constraint\nINVARIANT Inv\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"-workers", "1", "-coverage", "9999", "CoverageStatistics.tla"}
	out, code, stderr := runGoInDir(t, dir, args)
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %s output %s", code, stderr, out)
	}
	report := coverageReport(t, out)
	a, b, invariant := strings.Index(report, "<A "), strings.Index(report, "<B "), strings.Index(report, "<Inv ")
	if a < 0 || b <= a || invariant <= b {
		t.Fatalf("action or property order: %s", report)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareBooleanCoverageOracle(t, jar, dir, args, out)
	}
}

func TestCoverageRejectsBooleanActionsAcrossExtendedModules(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"Base.tla": "---- MODULE Base ----\nVARIABLE x\nFlip(v) == ~v\nZ == x' = Flip(x)\n====\n",
		"M.tla":    "---- MODULE M ----\nEXTENDS Base\nInit == x = FALSE\nA == x' = Flip(x)\nNext == A \\/ Z\n====\n",
		"M.cfg":    "INIT Init\nNEXT Next\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	args := []string{"-workers", "1", "-coverage", "9999", "M.tla"}
	out, code, stderr := runGoInDir(t, dir, args)
	if code == 0 || !strings.Contains(stderr, "-coverage does not yet support") {
		t.Fatalf("Go exit %d stderr %s output %s", code, stderr, out)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java, code := runJavaTLCArgs(t, jar, dir, args)
		if code != 0 || !strings.Contains(java, "5 states generated, 2 distinct states found") {
			t.Fatalf("Java exit %d: %s", code, java)
		}
	}
}

func TestCoverageRejectsBooleanActionsFromInstances(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"Base.tla": "---- MODULE Base ----\nVARIABLE x\nFlip(v) == ~v\nZ == x' = Flip(x)\n====\n",
		"M.tla":    "---- MODULE M ----\nVARIABLE x\nI == INSTANCE Base WITH x <- x\nJ == INSTANCE Base WITH x <- x\nInit == x = FALSE\nNext == I!Z \\/ J!Z\n====\n",
		"M.cfg":    "INIT Init\nNEXT Next\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	args := []string{"-workers", "1", "-coverage", "9999", "M.tla"}
	out, code, stderr := runGoInDir(t, dir, args)
	if code == 0 || !strings.Contains(stderr, "-coverage does not yet support") {
		t.Fatalf("Go exit %d stderr %s output %s", code, stderr, out)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java, code := runJavaTLCArgs(t, jar, dir, args)
		if code != 0 || !strings.Contains(java, "5 states generated, 2 distinct states found") || !strings.Contains(java, "<I!Z ") || !strings.Contains(java, "<J!Z ") {
			t.Fatalf("Java exit %d: %s", code, java)
		}
	}
}

func TestCoverageRejectsUnprovenBooleanToggleCosts(t *testing.T) {
	for _, tc := range []struct{ name, initial, definition, action, config string }{
		{"nonboolean init", "0", "~v", "x' = Switch(x)", ""},
		{"operator body", "FALSE", "~~v", "x' = Switch(x)", ""},
		{"different action", "FALSE", "~v", "x' = TRUE", ""},
		{"extra guard", "FALSE", "~v", "x = FALSE /\\ x' = Switch(x)", ""},
		{"call inside conjunction", "FALSE", "~v", `/\ x' = Switch(x) /\ UNCHANGED x`, ""},
		{"call inside conjunction with guard", "FALSE", "~v", `/\ x' = Switch(x) /\ x < 0`, ""},
		{"nested call", "FALSE", "~v", "x' = Switch(Switch(x))", ""},
		{"call with literal argument", "FALSE", "~v", "x' = Switch(TRUE)", ""},
		{"direct negation", "FALSE", "~v", "x' = ~x", ""},
		{"conditional operator body", "FALSE", "IF v THEN FALSE ELSE TRUE", "x' = Switch(x)", ""},
		{"invariant", "FALSE", "~v", "x' = Switch(x)", "INVARIANT Inv\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = " + tc.initial + "\nSwitch(v) == " + tc.definition + "\nA == x' = Switch(x)\nB == " + tc.action + "\nNext == A \\/ B\nInv == TRUE\n====\n"
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\n"+tc.config), 0o600); err != nil {
				t.Fatal(err)
			}
			_, code, stderr := runGoInDir(t, dir, []string{"-coverage", "0", "M.tla"})
			if code == 0 || !strings.Contains(stderr, "-coverage does not yet support") {
				t.Fatalf("exit %d stderr %s", code, stderr)
			}
		})
	}
}
