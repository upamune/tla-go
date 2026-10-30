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

// These unchanged models are from Java TLC v1.7.4, commit 5a47802.
func TestJavaTLCOperatorArguments(t *testing.T) {
	for _, tc := range []struct {
		name, stats string
	}{
		{"test51", "0 states generated, 0 distinct states found, 0 states left on queue."},
		{"test52", "3 states generated, 2 distinct states found, 0 states left on queue."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, name := range []string{"test51.tla", "test51a.tla", "test51b.tla", "test51.cfg", "test52.tla", "test52.cfg"} {
				data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", name))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, tc.name+".tla")}, &out, &errout)
			checkOperatorArgumentsOutput(t, tc.name, tc.stats, code, out.String(), errout.String())
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runJavaTLCForExit(t, jar, dir, tc.name, 0)
				checkOperatorArgumentsOutput(t, tc.name, tc.stats, 0, java, "")
			}
		})
	}
}

func checkOperatorArgumentsOutput(t *testing.T, name, stats string, code int, stdout, stderr string) {
	t.Helper()
	if code != 0 || stderr != "" || !strings.Contains(stdout, stats) {
		t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if name == "test52" {
		if !strings.Contains(stdout, `"Test1 Finished"  TRUE`) {
			t.Fatalf("missing output:\n%s", stdout)
		}
		return
	}
	for i := 1; i <= 95; i++ {
		message := fmt.Sprintf("\"Test %d OK\"  TRUE", i)
		if !strings.Contains(stdout, message) {
			t.Fatalf("missing %s in output:\n%s", message, stdout)
		}
	}
}

func TestJavaTLCSpecialOperatorArguments(t *testing.T) {
	dir := t.TempDir()
	source := `---- MODULE M ----
EXTENDS Naturals
VARIABLE x
Apply(F(_), a) == F(a)
Ignore(F(_, _)) == TRUE
ASSUME /\ Ignore(-+->) /\ Ignore(~>) /\ Ignore(\cdot)
Witness(n) == Apply(ENABLED, \E i \in {n} : x' = i)
Init == x = 0
Next == /\ x' = x /\ Apply(', x) = x /\ Apply(UNCHANGED, x) /\ Apply(ENABLED, x' = x) /\ Witness(x)
====
`
	for name, contents := range map[string]string{"M.tla": source, "M.cfg": "INIT Init\nNEXT Next\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	stats := "2 states generated, 1 distinct states found, 0 states left on queue."
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
	if code != 0 || errout.Len() != 0 || !strings.Contains(out.String(), stats) {
		t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java := runJavaTLCForExit(t, jar, dir, "M", 0)
		if !strings.Contains(java, stats) {
			t.Fatalf("Java TLC:\n%s", java)
		}
	}
}
