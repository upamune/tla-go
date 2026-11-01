package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Constant arguments and simple total actions exercise shared predicate roots
// without depending on the unimplemented nested cost model.
func TestJavaTLCCoverageParameterizedValues(t *testing.T) {
	for _, tc := range []struct{ name, init, definitions string }{
		{"zero-repeat", "x=0", "A(n) == x'=n\nNext == A(0) \\/ A(0)"},
		{"different", "x=5", "A(n) == x'=n\nNext == A(1) \\/ A(2) \\/ A(3)"},
		{"boolean", "x=TRUE", "A(n) == x'=n\nB(n) == x'=n\nNext == A(FALSE) \\/ B(TRUE)"},
		{"mixed-types", "x=0", "A(n) == x'=n\nNext == A(1) \\/ A(TRUE)"},
		{"unused", "x=0", "A(n) == UNCHANGED x\nB(n) == x'=n\nNext == A(3) \\/ B(1)"},
		{"literal-body", "x=0", "A(n) == x'=1\nB(n) == x'=0\nNext == A(3) \\/ B(4)"},
		{"empty-init", "x \\in 1..0", "A(n) == x'=n\nNext == A(1) \\/ A(2)"},
		{"range-init", "x \\in 0..2", "A(n) == x'=n\nNext == A(1) \\/ A(2)"},
		{"set-init", "x \\in {0,1,1,2}", "A(n) == x'=n\nNext == A(1) \\/ A(2)"},
		{"empty-set-init", "x \\in {}", "A(n) == x'=n\nNext == A(1) \\/ A(2)"},
		{"set-value-init", "x = {1,2}", "A(n) == x'=n\nNext == A(1) \\/ A(2)"},
		{"set-value-range", "x = {}", "A(n) == x' \\in 0..n\nNext == A(1) \\/ A(2)"},
		{"range-values", "x=0", "A(n) == x' \\in 0..n\nB(n,m) == x' \\in n..m\nNext == A(2) \\/ B(1,3)"},
		{"range-unused-bool", "x=0", "A(n,b) == x' \\in n..2\nNext == A(0,TRUE) \\/ A(1,FALSE)"},
		{"range-singleton", "x=0", "A(n) == x' \\in n..n\nNext == A(0) \\/ A(1)"},
		{"range-empty", "x=0", "A(n) == x' \\in n..0\nNext == A(1) \\/ A(2)"},
		{"range-repeat", "x \\in 0..2", "A(n) == x' \\in 0..n\nNext == A(2) \\/ A(1) \\/ A(2)"},
		{"range-empty-init", "x \\in 1..0", "A(n) == x' \\in 0..n\nNext == A(1) \\/ A(2)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeCoverageModule(t, dir, "M", "EXTENDS Naturals\nVARIABLE x\nInit == "+tc.init+"\n"+tc.definitions)
			writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\n")
			compareParameterizedCoverage(t, dir)
		})
	}
}

func TestJavaTLCCoverageParameterizedImportedActions(t *testing.T) {
	dir := t.TempDir()
	writeCoverageModule(t, dir, "Aux", "VARIABLE x\nA(n) == x'=n\nB(n) == x'=n")
	writeCoverageModule(t, dir, "M", "EXTENDS Aux\nInit == x=0\nNext == B(2) \\/ A(1)")
	writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\n")
	compareParameterizedCoverage(t, dir)
}

func compareParameterizedCoverage(t *testing.T, dir string) {
	t.Helper()
	args := coverageAArgs("M.tla", true)
	out, exit, stderr := runGoInDir(t, dir, args)
	if exit != 0 || stderr != "" {
		t.Fatalf("Go exit %d stderr %s output %s", exit, stderr, out)
	}
	if jar := pinnedFairnessEffectJar(t); jar != "" {
		java, javaExit := runJavaTLCArgs(t, jar, dir, args)
		if javaExit != exit || coverageReport(t, out) != coverageReport(t, java) {
			t.Fatalf("Java exit %d\nGo output %s\nJava output %s", javaExit, out, java)
		}
	}
	compareParameterizedToolCoverage(t, dir, args)
}

func writeParameterizedConfig(t *testing.T, dir, cfg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCoverageRejectsUnmodeledParameterizedCalls(t *testing.T) {
	for _, tc := range []struct{ name, body, config string }{
		{"state-argument", "A(n) == x'=n\nNext == A(x) \\/ A(1)", ""},
		{"set-argument", "A(n) == x'=n\nNext == A({1}) \\/ A(1)", ""},
		{"argument-error", "A(n) == x'=n\nNext == A(1 \\div 0) \\/ A(1)", ""},
		{"body-error", "A(n) == x'=(1 \\div 0)\nNext == A(1) \\/ A(2)", ""},
		{"guard", "A(n) == /\\ x<n\n        /\\ x'=n\nNext == A(1) \\/ A(2)", ""},
		{"range-state", "A(n) == x' \\in x..n\nNext == A(1) \\/ A(2)", ""},
		{"range-boolean", "A(n) == x' \\in 0..n\nNext == A(TRUE) \\/ A(2)", ""},
		{"range-wrap", "A(n) == x' \\in 2147483647..n\nNext == A(2147483647) \\/ A(2)", ""},
		{"range-wrap-literal", "A(n) == x' \\in n..2147483647\nNext == A(1) \\/ A(2)", ""},
		{"mixed-action-shapes", "A(n) == x'=n\nB == x'=1\nNext == A(1) \\/ B", ""},
		{"constraint", "A(n) == x'=n\nNext == A(1) \\/ A(2)\nLimit == x < 2", "CONSTRAINT Limit\n"},
		{"invariant", "A(n) == x'=n\nNext == A(1) \\/ A(2)\nInv == x < 3", "INVARIANT Inv\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeCoverageModule(t, dir, "M", "EXTENDS Integers\nVARIABLE x\nInit == x=0\n"+tc.body)
			writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\n"+tc.config)
			out, exit, stderr := runGoInDir(t, dir, coverageAArgs("M.tla", true))
			if exit == 0 || !strings.Contains(out+stderr, "-coverage does not yet support") {
				t.Fatalf("expected rejection: exit %d stderr %s output %s", exit, stderr, out)
			}
			compareUnsupportedParameterizedJava(t, dir, tc.name)
		})
	}
}

func compareUnsupportedParameterizedJava(t *testing.T, dir, name string) {
	t.Helper()
	jar := pinnedFairnessEffectJar(t)
	if jar == "" || name != "argument-error" && name != "body-error" && name != "constraint" {
		return
	}
	args := append([]string{"-tool"}, coverageAArgs("M.tla", true)...)
	java, exit := runJavaTLCArgs(t, jar, dir, args)
	messages := readToolMessages(java)
	if name == "constraint" {
		if exit != 0 || !hasToolMessage(messages, 2201) || !hasToolMessage(messages, 2199) {
			t.Fatalf("Java constraint exit %d output %s", exit, java)
		}
	} else if exit == 0 || !hasToolMessage(messages, 2103) {
		t.Fatalf("Java error exit %d output %s", exit, java)
	}
}

func TestCoverageParameterizedActionsRejectCrossModuleOrdering(t *testing.T) {
	dir := t.TempDir()
	writeCoverageModule(t, dir, "Aux", "VARIABLE x\nA(n) == x'=n")
	writeCoverageModule(t, dir, "M", "EXTENDS Aux\nInit == x=0\nB(n) == x'=n\nNext == B(2) \\/ A(1)")
	writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\n")
	out, exit, stderr := runGoInDir(t, dir, coverageAArgs("M.tla", true))
	if exit == 0 || !strings.Contains(out+stderr, "-coverage does not yet support") {
		t.Fatalf("expected rejection: exit %d stderr %s output %s", exit, stderr, out)
	}
}

func TestCoverageParameterizedCallsRejectUnverifiedModes(t *testing.T) {
	for _, opts := range [][]string{{"-workers", "2"}, {"-maxSetSize", "2"}, {"-dump", "dot", "graph.dot"}} {
		t.Run(opts[0], func(t *testing.T) {
			dir := t.TempDir()
			writeCoverageModule(t, dir, "M", "VARIABLE x\nInit == x=0\nA(n) == x'=n\nNext == A(1) \\/ A(2)")
			writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\n")
			args := append([]string{"-coverage", "9999"}, opts...)
			args = append(args, "M.tla")
			out, exit, stderr := runGoInDir(t, dir, args)
			if exit == 0 || !strings.Contains(out+stderr, "-coverage does not yet support") {
				t.Fatalf("expected rejection: exit %d stderr %s output %s", exit, stderr, out)
			}
		})
	}
}

func TestCoverageParameterizedCallsUseOneSharedRoot(t *testing.T) {
	// A tool-output control also verifies that the root is reported once.
	dir := t.TempDir()
	writeCoverageModule(t, dir, "M", "VARIABLE x\nInit == x=0\nA(n) == x'=n\nNext == A(1) \\/ A(2)")
	writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\n")
	args := append([]string{"-tool"}, coverageAArgs("M.tla", true)...)
	out, exit, stderr := runGoInDir(t, dir, args)
	if exit != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %s output %s", exit, stderr, out)
	}
	var roots []toolMessage
	for _, message := range readToolMessages(out) {
		if message.code == 2772 {
			roots = append(roots, message)
		}
	}
	if len(roots) != 1 || !strings.Contains(roots[0].body, ": 2:6") {
		t.Fatalf("unexpected roots %+v", roots)
	}
	if jar := pinnedFairnessEffectJar(t); jar != "" {
		java, javaExit := runJavaTLCArgs(t, jar, dir, args)
		if javaExit != 0 || !reflect.DeepEqual(coverageMessages(readToolMessages(out)), coverageMessages(readToolMessages(java))) {
			t.Fatalf("Java exit %d\nGo output %s\nJava output %s", javaExit, out, java)
		}
	}
}
