package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// test206 is unchanged from Java TLC v1.7.4, commit 5a47802.
func TestJavaTLCTheoremSubexpressions(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"test206.tla", "test206a.tla", "test206.cfg"} {
		data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{filepath.Join(dir, "test206.tla")}, &out, &errout)
	stats := "0 states generated, 0 distinct states found, 0 states left on queue."
	if code != 0 || errout.Len() != 0 || !strings.Contains(out.String(), stats) {
		t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java := runJavaTLCForExit(t, jar, dir, "test206", 0)
		if !strings.Contains(java, stats) {
			t.Fatalf("Java TLC:\n%s", java)
		}
	}
}

func TestJavaTLCAssumeProveSelections(t *testing.T) {
	for _, tc := range []struct {
		name, extra      string
		goCode, javaCode int
	}{
		{"unused", "", 0, 0},
		{"labeled conclusion", "ASSUME T!lab", 0, 0},
		{"first assumption", "ASSUME T!1 = FALSE /\\ T!<< = FALSE", 0, 0},
		{"conclusion", "ASSUME T!2 = TRUE /\\ T!>> = TRUE", 0, 0},
		{"not an expression", "ASSUME T", 255, 150},
		{"colon on theorem", "ASSUME T!:!lab", 255, 150},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := "---- MODULE M ----\nTHEOREM T == ASSUME FALSE PROVE lab :: TRUE\n" + tc.extra + "\n====\n"
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

func TestJavaTLCInstanceUnboundedSelectionScope(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"A.tla": "---- MODULE A ----\nVARIABLE y\nP == \\A x : y = x\n====\n",
		"M.tla": "---- MODULE M ----\nVARIABLE x\nI == INSTANCE A WITH y <- x\nInit == x = 0\nNext == UNCHANGED x\nInv == /\\ I!P!(2)!<< = x /\\ I!P!(2)!>> = 2\n====\n",
		"M.cfg": "INIT Init\nNEXT Next\nINVARIANT Inv\n",
	}
	for name, contents := range files {
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
