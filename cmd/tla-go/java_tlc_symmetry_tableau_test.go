package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestJavaTLCSymmetryTableauRegeneration(t *testing.T) {
	for _, name := range []string{"ChooseTableauSymmetryMC", "ChooseTableauSymmetryMCa"} {
		t.Run(name, func(t *testing.T) {
			dir := copySymmetryRecurrenceFixtures(t)
			checkSymmetryRecurrence(t, dir, name, 13, "13 states generated, 6 distinct states found, 0 states left on queue")
		})
	}
}

func TestJavaTLCSymmetryTableauToolTrace(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("set TLC_JAR to compare Java TLC tool output")
	}
	for _, name := range []string{"ChooseTableauSymmetryMC", "ChooseTableauSymmetryMCa"} {
		t.Run(name, func(t *testing.T) { compareSymmetryTableauToolTrace(t, jar, name) })
	}
}

func compareSymmetryTableauToolTrace(t *testing.T, jar, name string) {
	t.Helper()
	dir := copySymmetryRecurrenceFixtures(t)
	model := name + ".tla"
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final", filepath.Join(dir, model)}, &out, &errout)
	if code != 13 || errout.Len() != 0 {
		t.Fatalf("tla-go exit %d; stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "-lncheck", "final", model)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil || cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code {
		t.Fatalf("Java TLC: %v\n%s", err, output)
	}
	java := toolConsumerMessages(readToolMessages(string(output)))
	got := toolConsumerMessages(readToolMessages(out.String()))
	if !reflect.DeepEqual(java, got) {
		t.Fatalf("consumer messages differ\nJava: %+v\nGo: %+v", java, got)
	}
}

func TestJavaTLCSymmetryLeadsToFairness(t *testing.T) {
	for _, tc := range []struct {
		name, variables, fairness, property string
		constraints                         bool
		code                                int
	}{
		{"no fairness", "x,bit", "", "(x=a) ~> (bit=1)", false, 13},
		{"weak fairness", "x,bit", " /\\ WF_vars(Next)", "(x=a) ~> (bit=1)", false, 0},
		{"strong fairness", "x,bit", " /\\ SF_vars(Next)", "(x=a) ~> (bit=1)", false, 0},
		{"other variable order", "bit,x", " /\\ WF_vars(Next)", "(x=a) ~> (bit=1)", false, 0},
		{"state and action constraints", "x,bit", " /\\ WF_vars(Next)", "(x=a) ~> (bit=1)", true, 0},
		{"other trigger without fairness", "x,bit", "", "(bit=1) ~> (x=a)", false, 13},
		{"other trigger with fairness", "x,bit", " /\\ WF_vars(Next)", "(bit=1) ~> (x=a)", false, 13},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "CONSTANTS P,a,b\nVARIABLES " + tc.variables + "\nvars == <<x,bit>>\n" +
				"Init == x \\in P /\\ bit = 0\nNext == x' \\in P /\\ bit' \\in {0,1}\n" +
				"Spec == Init /\\ [][Next]_vars" + tc.fairness + "\nSym == Permutations(P)\nProp == " + tc.property
			cfg := "CONSTANTS P={a,b} a=a b=b SPECIFICATION Spec SYMMETRY Sym PROPERTY Prop CHECK_DEADLOCK FALSE"
			if tc.constraints {
				body += "\nCon == bit=0 \\/ x=a\nAct == bit'=bit \\/ x'=x"
				cfg += " CONSTRAINT Con ACTION_CONSTRAINT Act"
			}
			writeDOTFixture(t, dir, body, cfg)
			checkSymmetryRecurrence(t, dir, "M", tc.code, "10 states generated, 2 distinct states found, 0 states left on queue")
		})
	}
}

func TestJavaTLCSymmetryLeadsToToolTrace(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("set TLC_JAR to compare Java TLC tool output")
	}
	dir := t.TempDir()
	body := "CONSTANTS P,a,b\nVARIABLES x,bit\nvars == <<x,bit>>\n" +
		"Init == x \\in P /\\ bit = 0\nNext == x' \\in P /\\ bit' \\in {0,1}\n" +
		"Spec == Init /\\ [][Next]_vars\nSym == Permutations(P)\nProp == (x=a) ~> (x=b)"
	cfg := "CONSTANTS P={a,b} a=a b=b SPECIFICATION Spec SYMMETRY Sym PROPERTY Prop CHECK_DEADLOCK FALSE"
	writeDOTFixture(t, dir, body, cfg)
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final", filepath.Join(dir, "M.tla")}, &out, &errout)
	if code != 13 || errout.Len() != 0 {
		t.Fatalf("tla-go exit %d; stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
	}
	compareJavaToolMessages(t, jar, dir, code, readToolMessages(out.String()))
}
