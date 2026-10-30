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

// Java TLC reports a typed evaluation failure, including the offending
// successor state, when a non-member has the wrong type for an infinite set.
func TestJavaTLCInfiniteSetMembershipFailure(t *testing.T) {
	for _, tc := range []struct {
		name, extends, initial, next, set, bad, statistics string
	}{
		{"nat", "Naturals", "0", "FALSE", "Nat", "FALSE", "2 states generated, 2 distinct states found, 0 states left on queue."},
		{"int", "Integers", "0", `"bad"`, "Int", `"bad"`, "2 states generated, 2 distinct states found, 0 states left on queue."},
		{"string", "TLC", `"a"`, "42", "STRING", "42", "2 states generated, 2 distinct states found, 0 states left on queue."},
		{"nat after completed state", "Naturals", "0", "IF x = 0 THEN 1 ELSE FALSE", "Nat", "FALSE", "3 states generated, 3 distinct states found, 0 states left on queue."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := fmt.Sprintf("---- MODULE M ----\nEXTENDS %s\nVARIABLE x\nInit == x = %s\nNext == x' = %s\nInv == x \\in %s\n====\n", tc.extends, tc.initial, tc.next, tc.set)
			for name, content := range map[string]string{"M.tla": source, "M.cfg": "INIT Init\nNEXT Next\nINVARIANT Inv\n"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
			message := "Error: Evaluating invariant Inv failed.\nAttempted to check if the value:\n" + tc.bad + "\nis an element of " + tc.set + "."
			if code != 76 || errout.Len() != 0 || !strings.Contains(out.String(), message) || !strings.Contains(out.String(), tc.statistics) {
				t.Fatalf("exit %d, stdout %q, stderr %q", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runIncompleteJava(t, jar, dir, "M", 76)
				if got, want := incompleteOutput(out.String(), "Error: Evaluating invariant"), incompleteOutput(java, "Error: Evaluating invariant"); got != want {
					t.Fatalf("Go output:\n%s\nJava output:\n%s", got, want)
				}
			}
		})
	}
}

func TestJavaTLCInfiniteMembershipEvaluationContexts(t *testing.T) {
	for _, tc := range []struct {
		name, definitions, config string
		exit                      int
	}{
		{"assumption", "ASSUME FALSE \\in Nat\nInit == x = 0\nNext == x' = x", "INIT Init\nNEXT Next\n", 10},
		{"initial state", "Init == x = (FALSE \\in Nat)\nNext == x' = x", "INIT Init\nNEXT Next\n", 75},
		{"next state", "Init == x = 0\nNext == x' = (FALSE \\in Nat)", "INIT Init\nNEXT Next\n", 255},
		{"initial invariant", "Init == x = FALSE\nNext == x' = x\nInv == x \\in Nat", "INIT Init\nNEXT Next\nINVARIANT Inv\n", 75},
		{"initial invariant in conjunction", "Init == /\\ x = FALSE /\\ TRUE\nNext == x' = x\nInv == x \\in Nat", "INIT Init\nNEXT Next\nINVARIANT Inv\n", 75},
		{"initial invariant in existential", "Init == \\E n \\in {FALSE} : x = n\nNext == x' = x\nInv == x \\in Nat", "INIT Init\nNEXT Next\nINVARIANT Inv\n", 75},
		{"initial invariant in disjunction", "Init == \\/ x = FALSE \\/ x = TRUE\nNext == x' = x\nInv == x \\in Nat", "INIT Init\nNEXT Next\nINVARIANT Inv\n", 75},
		{"initial invariant in membership", "Init == x \\in {FALSE}\nNext == x' = x\nInv == x \\in Nat", "INIT Init\nNEXT Next\nINVARIANT Inv\n", 75},
		{"initial invariant in operator", "Choice == x = FALSE\nInit == Choice\nNext == x' = x\nInv == x \\in Nat", "INIT Init\nNEXT Next\nINVARIANT Inv\n", 75},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\n" + tc.definitions + "\n====\n"
			for name, content := range map[string]string{"M.tla": source, "M.cfg": tc.config} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
			message := "Attempted to check if the value:\nFALSE\nis an element of Nat."
			if code != tc.exit || errout.Len() != 0 || !strings.Contains(out.String(), message) || strings.Contains(out.String(), "While working on the initial state:") {
				t.Fatalf("exit %d, stdout %q, stderr %q", code, out.String(), errout.String())
			}
			if tc.name != "assumption" && (!strings.Contains(out.String(), "The error occurred when TLC was evaluating the nested\nexpressions at the following positions:") || strings.Contains(out.String(), "The depth of the complete state graph")) {
				t.Fatalf("incorrect evaluation diagnostics:\n%s", out.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runIncompleteJava(t, jar, dir, "M", tc.exit)
				if got, want := incompleteOutput(out.String(), "Error:"), incompleteOutput(java, "Error:"); got != want {
					t.Fatalf("Go output:\n%s\nJava output:\n%s", got, want)
				}
			}
		})
	}
}

func TestJavaTLCInitialInvariantFailureMultipleVariables(t *testing.T) {
	for _, init := range []string{`/\ x = FALSE /\ y = 0`, `/\ y = 0 /\ x = FALSE`} {
		t.Run(init, func(t *testing.T) {
			dir := t.TempDir()
			source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLES x, y\nInit == " + init + "\nNext == UNCHANGED <<x, y>>\nInv == x \\in Nat\n====\n"
			for name, content := range map[string]string{"M.tla": source, "M.cfg": "INIT Init\nNEXT Next\nINVARIANT Inv\n"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != 75 || errout.Len() != 0 || !strings.Contains(out.String(), "Attempted to check if the value:\nFALSE\nis an element of Nat.") || !strings.Contains(out.String(), "0. Line 4, column 9") || strings.Contains(out.String(), "The depth of the complete state graph") {
				t.Fatalf("exit %d, stdout %q, stderr %q", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runIncompleteJava(t, jar, dir, "M", 75)
				if got, want := incompleteOutput(out.String(), "Error:"), incompleteOutput(java, "Error:"); got != want {
					t.Fatalf("Go output:\n%s\nJava output:\n%s", got, want)
				}
			}
		})
	}
}
