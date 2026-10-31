package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Java TLC's AbstractChecker warns before assumptions are checked when a model
// with implied temporal properties declares symmetry (2279) or constraints (2284).
func TestJavaTLCLivenessWarnings(t *testing.T) {
	for _, tc := range []struct {
		name, property, extra string
		want                  []int
	}{
		{"symmetry and constraints", "Prop", "SYMMETRY Sym CONSTRAINT Con", []int{2279, 2284}},
		{"action constraint", "Prop", "ACTION_CONSTRAINT Act", []int{2284}},
		{"true constraint", "Prop", "CONSTRAINT Yes", nil},
		{"conjunction", "Both", "SYMMETRY Sym", []int{2279}},
		{"quantified always", "AllAlways", "SYMMETRY Sym", []int{2279}},
		{"invariant property", "Safe", "SYMMETRY Sym CONSTRAINT Con", nil},
		{"action property", "Steps", "SYMMETRY Sym CONSTRAINT Con", nil},
		{"fairness only", "", "SYMMETRY Sym CONSTRAINT Con", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "CONSTANTS P, a, b\nVARIABLE x\nInit == x \\in P\nNext == x' \\in P\n" +
				"Spec == Init /\\ [][Next]_x /\\ WF_x(Next)\nSym == Permutations(P)\nCon == x \\in P\nYes == TRUE\n" +
				"Act == x' \\in P\nProp == <>(x \\in P)\nSafe == [](x \\in P)\nSteps == [][x' \\in P]_x\n" +
				"Both == Safe /\\ Prop\nAllAlways == \\A p \\in P : [](x \\in P)"
			cfg := "CONSTANTS P = {a, b} a = a b = b\nSPECIFICATION Spec\n" + tc.extra + "\n"
			if tc.property != "" {
				cfg += "PROPERTY " + tc.property + "\n"
			}
			writeDOTFixture(t, dir, body, cfg)
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final", filepath.Join(dir, "M.tla")}, &out, &errout)
			messages := readToolMessages(out.String())
			if got := warningsBeforeInitialStates(messages); code != 0 || errout.Len() != 0 || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("exit %d, warnings %v, want %v; stdout:\n%s\nstderr:\n%s", code, got, tc.want, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaToolMessages(t, jar, dir, code, messages)
			}
		})
	}
}

// Java TLC warns in AbstractChecker before LiveCheck rejects a temporal
// tautology with message 2253.
func TestJavaTLCLivenessWarningBeforeTautology(t *testing.T) {
	dir := t.TempDir()
	body := "CONSTANTS P, a, b\nVARIABLE x\nInit == x \\in P\nNext == x' \\in P\nSpec == Init /\\ [][Next]_x /\\ WF_x(Next)\n" +
		"Sym == Permutations(P)\nQuant == \\A p \\in P : <>(x = p) \\/ TRUE"
	writeDOTFixture(t, dir, body, "CONSTANTS P = {a, b} a = a b = b\nSPECIFICATION Spec\nSYMMETRY Sym\nPROPERTY Quant\n")
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final", filepath.Join(dir, "M.tla")}, &out, &errout)
	messages := readToolMessages(out.String())
	var codes []int
	for _, message := range messages {
		if message.code == 2279 || message.code == 2253 {
			codes = append(codes, message.code)
		}
	}
	if code != 77 || errout.Len() != 0 || !reflect.DeepEqual(codes, []int{2279, 2253}) {
		t.Fatalf("exit %d, messages %v; stdout:\n%s\nstderr:\n%s", code, codes, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareJavaToolMessages(t, jar, dir, code, messages)
	}
}

func warningsBeforeInitialStates(messages []toolMessage) []int {
	var warnings []int
	for _, message := range messages {
		if message.code == 2189 {
			return warnings
		}
		if message.level == 3 {
			warnings = append(warnings, message.code)
		}
	}
	return nil
}

func TestJavaTLCLivenessWarningText(t *testing.T) {
	dir := copySymmetryRecurrenceFixtures(t)
	model := filepath.Join(dir, "April25MC.tla")
	var out, errout bytes.Buffer
	if code := run(context.Background(), []string{"-deadlock", model}, &out, &errout); code != 0 || errout.Len() != 0 {
		t.Fatalf("exit %d; stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
	}
	want := "Warning: " + "Declaring symmetry during liveness checking is dangerous. It might cause TLC to miss violations of the stated liveness properties. Please check liveness without symmetry defined.\n" +
		"(Use the -nowarning option to disable this warning.)\n"
	if !strings.Contains(out.String(), want) {
		t.Fatalf("missing warning %q in:\n%s", want, out.String())
	}
	out.Reset()
	if code := run(context.Background(), []string{"-deadlock", "-nowarning", "-tool", model}, &out, &errout); code != 0 || strings.Contains(out.String(), "Warning") || hasToolMessage(readToolMessages(out.String()), 2279) {
		t.Fatalf("-nowarning printed a warning (exit %d):\n%s", code, out.String())
	}
}
