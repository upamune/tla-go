package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Github179cTest and its model are from Java TLC v1.7.4, commit 5a47802.
func TestJavaTLCGithub179c(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"Github179c.tla", "Github179c.cfg"} {
		data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{filepath.Join(dir, "Github179c.tla")}, &out, &errout)
	message := "Attempted to check equality of integer 1 with non-integer:\n{1}"
	if code != 75 || errout.Len() != 0 || !strings.Contains(out.String(), message) {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java := runIncompleteJava(t, jar, dir, "Github179c", 75)
		if got, want := incompleteOutput(out.String(), "Error: TLC threw"), incompleteOutput(java, "Error: TLC threw"); got != want {
			t.Fatalf("Go output:\n%s\nJava output:\n%s", got, want)
		}
	}
}

func TestJavaTLCIntegerEqualityFailure(t *testing.T) {
	for _, tc := range []struct {
		name, definitions string
		exit              int
	}{
		{"assumption", "ASSUME 1 = {1}\nInit == x = 0\nNext == x' = x", 10},
		{"next", "Init == x = 0\nNext == /\\ 1 = {1} /\\ x' = x", 75},
		{"print argument", "Init == x = 0\nNext == PrintT(1 = {1}) /\\ x' = x", 75},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := "---- MODULE M ----\nEXTENDS TLC\nVARIABLE x\n" + tc.definitions + "\n====\n"
			for name, content := range map[string]string{"M.tla": source, "M.cfg": "INIT Init\nNEXT Next\n"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
			message := "Attempted to check equality of integer 1 with non-integer:\n{1}"
			if code != tc.exit || errout.Len() != 0 || !strings.Contains(out.String(), message) {
				t.Fatalf("exit %d, stdout %q, stderr %q", code, out.String(), errout.String())
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

func TestJavaTLCPrintDeferredEqualityFailure(t *testing.T) {
	model, err := os.ReadFile(filepath.Join("testdata", "java-tlc", "Github179c.tla"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expr := range []string{"Print(sillyExpr, TRUE)", "Print(TRUE, sillyExpr)"} {
		t.Run(expr, func(t *testing.T) {
			dir := t.TempDir()
			source := strings.Replace(string(model), "PrintT(sillyExpr)", expr, 1)
			for name, content := range map[string]string{"Github179c.tla": source, "Github179c.cfg": "INIT Init\nNEXT Next\n"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "Github179c.tla")}, &out, &errout)
			if code != 75 || errout.Len() != 0 || !strings.Contains(out.String(), "tlc2.module.TLC.Print(tlc2.value.impl.Value,tlc2.value.impl.Value)") {
				t.Fatalf("exit %d, stdout %q, stderr %q", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runIncompleteJava(t, jar, dir, "Github179c", 75)
				if got, want := incompleteOutput(out.String(), "Error: TLC threw"), incompleteOutput(java, "Error: TLC threw"); got != want {
					t.Fatalf("Go output:\n%s\nJava output:\n%s", got, want)
				}
			}
		})
	}
}
