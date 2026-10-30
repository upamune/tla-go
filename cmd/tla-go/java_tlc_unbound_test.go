package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// etest6 is unchanged from the Java TLC v1.7.4 test suite.
func TestJavaTLCUnboundNextVariable(t *testing.T) {
	dir := t.TempDir()
	for _, ext := range []string{".tla", ".cfg"} {
		name := "etest6" + ext
		data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{filepath.Join(dir, "etest6.tla")}, &out, &errout)
	for _, want := range []string{
		"Error: TLC threw an unexpected exception.",
		"In evaluation, the identifier x is either undefined or not an operator.\nline 16, col 23 to line 16, col 23 of module etest6",
		"State 1: <Initial predicate>\nx = <<1, 2>>",
		"The error occurred when TLC was evaluating the nested",
		"1 states generated, 1 distinct states found, 0 states left on queue.",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in stdout:\n%s\nstderr:\n%s", want, out.String(), errout.String())
		}
	}
	if code != 75 || errout.Len() != 0 {
		t.Fatalf("exit %d, stderr %s", code, errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		runJavaTLCForExit(t, jar, dir, "etest6", 75)
	}
}

func TestJavaTLCUnboundInitialVariable(t *testing.T) {
	for _, tc := range []struct{ name, declarations, init, next, missing string }{
		{"self", "x", "x = x + 1", "UNCHANGED x", "x"},
		{"later assignment", "x, y", "/\\ x = y /\\ y = 1", "UNCHANGED <<x,y>>", "y"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLES " + tc.declarations + "\nInit == " + tc.init + "\nNext == " + tc.next + "\n====\n"
			for name, contents := range map[string]string{"M.tla": source, "M.cfg": "INIT Init\nNEXT Next\n"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
			want := "In evaluation, the identifier " + tc.missing + " is either undefined or not an operator."
			if code != 75 || errout.Len() != 0 || !strings.Contains(out.String(), want) || !strings.Contains(out.String(), "0 states generated, 0 distinct states found, 0 states left on queue.") {
				t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				runJavaTLCForExit(t, jar, dir, "M", 75)
			}
		})
	}
}

func TestJavaTLCPrimedVariableEvaluationOrder(t *testing.T) {
	for _, tc := range []struct {
		name, next string
		code       int
	}{
		{"read before assignment", `x' = x' + 1`, 75},
		{"read after assignment", `/\ x' = 1 /\ x' = x' + 1`, 11},
		{"valid unchanged state", `x' = x`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == " + tc.next + "\n====\n"
			for name, contents := range map[string]string{"M.tla": source, "M.cfg": "INIT Init\nNEXT Next\n"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != tc.code || errout.Len() != 0 {
				t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				runJavaTLCForExit(t, jar, dir, "M", tc.code)
			}
		})
	}
}
