package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The diagnostics and original fixtures come from Java TLC v1.7.4's
// EmptySubsetEqTest, FingerprintExceptionInitTest and FingerprintExceptionNextTest.
func TestJavaTLCFingerprintFailures(t *testing.T) {
	cases := []struct {
		name, cause, stack, states, evidence string
	}{
		{"EmptySubsetEq", "Attempted to check if the value:\n{}\nis in the integer interval 1..4", "0) line 8, col 18 to line 8, col 30 of module EmptySubsetEq", "1 states generated, 1 distinct states found, 0 states left on queue.", "State 1: <Initial predicate>\nb = TRUE"},
		{"FingerprintExceptionInit", "Overflow when computing the number of elements in:\nSUBSET 1..36", "1) line 7, col 20 to line 7, col 32 of module FingerprintExceptionInit\n0) line 7, col 13 to line 7, col 33 of module FingerprintExceptionInit", "1 states generated, 0 distinct states found, 0 states left on queue.", "While working on the initial state:\nx = SUBSET SUBSET 1..36"},
		{"FingerprintExceptionNext", "Overflow when computing the number of elements in:\nSUBSET 1..32", "1) line 8, col 51 to line 8, col 63 of module FingerprintExceptionNext\n0) line 8, col 44 to line 8, col 64 of module FingerprintExceptionNext", "2 states generated, 1 distinct states found, 0 states left on queue.", "TLC threw an unexpected exception."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := copyIncompleteFixture(t, tc.name)
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, tc.name+".tla")}, &out, &errout)
			for _, want := range []string{"Error: TLC was unable to fingerprint.", "Fingerprint Stack Trace:\n" + tc.stack, "Reason:\n" + tc.cause, tc.states, tc.evidence} {
				if !strings.Contains(out.String(), want) {
					t.Errorf("missing %q in:\n%s", want, out.String())
				}
			}
			if code != 75 || errout.Len() != 0 {
				t.Fatalf("exit %d, stderr %q", code, errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runIncompleteJava(t, jar, dir, tc.name, 75)
				if got, want := incompleteOutput(out.String(), "Error:"), incompleteOutput(java, "Error:"); got != want {
					t.Fatalf("Go output:\n%s\nJava output:\n%s", got, want)
				}
			}
		})
	}
}

func TestJavaTLCInvalidIntervalMembership(t *testing.T) {
	cases := []struct {
		name, constant, init, next, config, cause string
	}{
		{"InvalidInterval", "", "x = 0", `x' = ({} \in 1..4)`, "", "{}"},
		{"InvalidInit", "", `x = ({} \in 1..4)`, "UNCHANGED x", "", "{}"},
		{"TypedInterval", "CONSTANT a_1\n", "x = 0", `x' = (a_1 \in 1..4)`, "CONSTANT a_1 = a_1\n", "a_1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := fmt.Sprintf("---- MODULE %s ----\nEXTENDS Integers\n%sVARIABLE x\nInit == %s\nNext == %s\n====\n", tc.name, tc.constant, tc.init, tc.next)
			for name, content := range map[string]string{tc.name + ".tla": source, tc.name + ".cfg": tc.config + "INIT Init\nNEXT Next\n"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, tc.name+".tla")}, &out, &errout)
			message := "Attempted to check if the value:\n" + tc.cause + "\nis in the integer interval 1..4"
			if code != 75 || errout.Len() != 0 || !strings.Contains(out.String(), message) || !strings.Contains(out.String(), "expressions at the following positions:") {
				t.Fatalf("exit %d, stdout %q, stderr %q", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runIncompleteJava(t, jar, dir, tc.name, 75)
				if got, want := incompleteOutput(out.String(), "Error:"), incompleteOutput(java, "Error:"); got != want {
					t.Fatalf("Go output:\n%s\nJava output:\n%s", got, want)
				}
			}
		})
	}
}
