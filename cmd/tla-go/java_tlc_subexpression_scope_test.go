package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// test209 is unchanged from Java TLC v1.7.4, commit 5a47802.
func TestJavaTLCPositionalSubexpressionModel(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"test209.tla", "test209.cfg"} {
		data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{filepath.Join(dir, "test209.tla")}, &out, &errout)
	stats := "0 states generated, 0 distinct states found, 0 states left on queue."
	if code != 0 || errout.Len() != 0 || !strings.Contains(out.String(), stats) {
		t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java := runJavaTLCForExit(t, jar, dir, "test209", 0)
		if !strings.Contains(java, stats) {
			t.Fatalf("Java TLC:\n%s", java)
		}
	}
}

func TestJavaTLCSelectedLetScope(t *testing.T) {
	for _, tc := range []struct {
		name, definitions string
		goCode, javaCode  int
	}{
		{"later global", "M == LET Z == 23 IN Z\nZ == 44\nASSUME M!1 = 23", 0, 0},
		{"nested locals", "M == LET Z == 23 IN LET K == Z+1 IN K\nZ == 44\nK == 99\nASSUME M!1!1 = 24", 0, 0},
		{"local function", "M == LET Z == 23 F(x) == Z+x IN <<F(2), F(3)>>\nZ == 44\nF(x) == 99\nASSUME M!1!<< = 25 /\\ M!1!>> = 26", 0, 0},
		{"earlier global", "Z == 44\nM == LET Z == 23 IN Z\nASSUME M!1 = 23", 255, 150},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := "---- MODULE M ----\nEXTENDS Naturals\n" + tc.definitions + "\n====\n"
			for name, contents := range map[string]string{"M.tla": source, "M.cfg": ""} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != tc.goCode {
				t.Fatalf("exit %d, want %d, stdout:\n%s\nstderr:\n%s", code, tc.goCode, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				runJavaTLCForExit(t, jar, dir, "M", tc.javaCode)
			}
		})
	}
}
