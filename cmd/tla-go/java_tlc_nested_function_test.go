package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// test45 is unchanged from Java TLC v1.7.4, commit 5a47802.
func TestJavaTLCNestedRecursiveFunctions(t *testing.T) {
	dir := t.TempDir()
	for _, ext := range []string{".tla", ".cfg"} {
		data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", "test45"+ext))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "test45"+ext), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{filepath.Join(dir, "test45.tla")}, &out, &errout)
	checkNestedFunctionOutput(t, code, out.String(), errout.String())
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		output := runNestedFunctionJava(t, jar, dir, "test45", 0)
		checkNestedFunctionOutput(t, 0, output, "")
	}
}

func checkNestedFunctionOutput(t *testing.T, code int, stdout, stderr string) {
	t.Helper()
	stats := "2 states generated, 1 distinct states found, 0 states left on queue."
	if code != 0 || stderr != "" || !strings.Contains(stdout, stats) {
		t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	for i := 1; i <= 12; i++ {
		message := fmt.Sprintf("\"Test %d OK\"  TRUE", i)
		if i == 3 {
			message = "\"Test 3  OK\"  TRUE"
		}
		if !strings.Contains(stdout, message) {
			t.Fatalf("missing %s in output:\n%s", message, stdout)
		}
	}
}

func TestJavaTLCFunctionApplicationEvaluation(t *testing.T) {
	for _, tc := range []struct {
		name, expression string
		code             int
	}{
		{"nested indexed", "f[0][0] = 1", 0},
		{"nested let", "(LET G == f[0] IN G[0]) = 1", 0},
		{"literal indexed", "([i \\in 0..1 |-> IF i=0 THEN 1 ELSE 1 \\div 0])[0] = 1", 0},
		{"whole value", "DOMAIN f[0] = 0..1", 10},
		{"cache and whole value", "/\\ f[0][0] = 1 /\\ DOMAIN f[0] = 0..1", 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := "---- MODULE M ----\nEXTENDS Naturals\nf[n \\in 0..1] == [j \\in 0..1 |-> IF j = 0 THEN 1 ELSE 1 \\div 0]\nASSUME " + tc.expression + "\n====\n"
			for name, content := range map[string]string{"M.tla": source, "M.cfg": ""} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != tc.code || code == 10 && !strings.Contains(out.String()+errout.String(), "\\div") {
				t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				runNestedFunctionJava(t, jar, dir, "M", tc.code)
			}
		})
	}
}

func runNestedFunctionJava(t *testing.T, jar, dir, name string, want int) string {
	t.Helper()
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-workers", "1", "-fp", "0", name+".tla")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	code := 0
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatalf("Java TLC: %v\n%s", err, out)
	}
	if code != want {
		t.Fatalf("Java TLC exit %d, want %d:\n%s", code, want, out)
	}
	return string(out)
}
