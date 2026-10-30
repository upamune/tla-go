package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// These cases compare Java TLC v1.7.4's LazyValue and operator evaluation.
func TestJavaTLCLazyOperatorArguments(t *testing.T) {
	for _, tc := range []struct {
		name, definitions, statistics string
		prints                        int
	}{
		{"unused", "F(a) == TRUE\nInit == x = 0\nNext == /\\ x' = 1 /\\ F(1 \\div 0)", "3 states generated, 2 distinct states found, 0 states left on queue.", 0},
		{"primed", "F(a) == a'\nInit == x = 1\nNext == /\\ x' = 2 /\\ F(IF x = 1 THEN 1 \\div 0 ELSE x) = 2", "3 states generated, 2 distinct states found, 0 states left on queue.", 0},
		{"assumption cached", "F(a) == a /\\ a\nASSUME F(PrintT(\"probe\"))\nInit == x = 0\nNext == x' = x", "2 states generated, 1 distinct states found, 0 states left on queue.", 1},
		{"action cached", "F(a) == a /\\ a\nInit == x = 0\nNext == /\\ x' = x /\\ F(PrintT(\"probe\"))", "2 states generated, 1 distinct states found, 0 states left on queue.", 1},
		{"current and primed", "F(a) == /\\ a = 1 /\\ a' = 2\nInit == x = 1\nNext == /\\ x' = 2 /\\ F(x)", "2 states generated, 2 distinct states found, 0 states left on queue.", 0},
		{"recursive", "RECURSIVE F(_)\nF(a) == IF a = 0 THEN TRUE ELSE F(a - 1)\nASSUME F(3)\nInit == x = 0\nNext == x' = x", "2 states generated, 1 distinct states found, 0 states left on queue.", 0},
		{"recursive unused", "RECURSIVE F(_)\nF(a) == TRUE\nInit == x = 0\nNext == /\\ x' = x /\\ F(1 \\div 0)", "2 states generated, 1 distinct states found, 0 states left on queue.", 0},
		{"higher unused", "Apply(F(_), a) == TRUE\nInit == x = 0\nNext == /\\ x' = x /\\ Apply(LAMBDA n : n, 1 \\div 0)", "2 states generated, 1 distinct states found, 0 states left on queue.", 0},
		{"higher named prime", "D(n) == n'\nApply(F(_), a) == F(a)\nInit == x = 1\nNext == /\\ x' = 2 /\\ Apply(D, x) = 2", "3 states generated, 2 distinct states found, 0 states left on queue.", 0},
		{"higher lambda prime", "Apply(F(_), a) == F(a)\nInit == x = 1\nNext == /\\ x' = 2 /\\ Apply(LAMBDA n : n', x) = 2", "3 states generated, 2 distinct states found, 0 states left on queue.", 0},
		{"nested calls", "F(a) == a + 1\nInit == x = 0\nNext == /\\ x' = x /\\ F(F(1)) = 3", "2 states generated, 1 distinct states found, 0 states left on queue.", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\n" + tc.definitions + "\n====\n"
			for name, content := range map[string]string{"M.tla": source, "M.cfg": "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
			checkLazyArgumentsOutput(t, out.String(), errout.String(), code, tc.statistics, tc.prints)
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				jar, err := filepath.Abs(jar)
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, "java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-workers", "1", "-fp", "0", "M.tla")
				cmd.Dir = dir
				java, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("Java TLC: %v\n%s", err, java)
				}
				checkLazyArgumentsOutput(t, string(java), "", 0, tc.statistics, tc.prints)
			}
		})
	}
}

func checkLazyArgumentsOutput(t *testing.T, stdout, stderr string, code int, statistics string, prints int) {
	t.Helper()
	if code != 0 || stderr != "" || !strings.Contains(stdout, statistics) || strings.Count(stdout, "\"probe\"") != prints {
		t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
}

func TestJavaTLCLazyArgumentsInsideEnabled(t *testing.T) {
	dir := t.TempDir()
	source := `---- MODULE M ----
EXTENDS Naturals, TLC
VARIABLE x
F(a) == /\ a = 0 /\ TLCSet(0,1) /\ a = 1
Init == /\ x = 0 /\ TLCSet(0,0)
Next == x' = x
Inv == ENABLED (x' = x /\ F(TLCGet(0)))
====
`
	for name, content := range map[string]string{"M.tla": source, "M.cfg": "INIT Init\nNEXT Next\nINVARIANT Inv\nCHECK_DEADLOCK FALSE\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
	stats := "2 states generated, 1 distinct states found, 0 states left on queue."
	checkLazyArgumentsOutput(t, out.String(), errout.String(), code, stats, 0)
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		jar, err := filepath.Abs(jar)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-workers", "1", "-fp", "0", "M.tla")
		cmd.Dir = dir
		java, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Java TLC: %v\n%s", err, java)
		}
		checkLazyArgumentsOutput(t, string(java), "", 0, stats, 0)
	}
}
