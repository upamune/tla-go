package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJavaTLCImportedModuleErrors(t *testing.T) {
	for _, tc := range []struct {
		name, root, message string
		modules             map[string]string
	}{
		{"extends name mismatch", "EXTENDS A", "File name 'A' does not match the name 'Wrong'", map[string]string{"A": "---- MODULE Wrong ----\n====\n"}},
		{"instance name mismatch", "I == INSTANCE A", "File name 'A' does not match the name 'Wrong'", map[string]string{"A": "---- MODULE Wrong ----\n====\n"}},
		{"nested name mismatch", "EXTENDS A", "File name 'B' does not match the name 'Wrong'", map[string]string{"A": "---- MODULE A ----\nEXTENDS B\n====\n", "B": "---- MODULE Wrong ----\n====\n"}},
		{"missing extension", "EXTENDS A", "load module A", nil},
		{"missing instance", "I == INSTANCE A", "load instance module A", nil},
		{"invalid extension", "EXTENDS A", "module A:", map[string]string{"A": "---- MODULE A ----\nBad == (\n====\n"}},
		{"invalid instance", "I == INSTANCE A", "module A:", map[string]string{"A": "---- MODULE A ----\nBad == (\n====\n"}},
		{"cyclic extension", "EXTENDS A", "cyclic module dependency", map[string]string{"A": "---- MODULE A ----\nEXTENDS M\n====\n"}},
		{"cyclic instance", "I == INSTANCE A", "cyclic instance dependency", map[string]string{"A": "---- MODULE A ----\nJ == INSTANCE M\n====\n"}},
		{"extension and instance cycle", "EXTENDS A", "cyclic instance dependency", map[string]string{"A": "---- MODULE A ----\nJ == INSTANCE M\n====\n"}},
		{"instance and extension cycle", "I == INSTANCE A", "cyclic module dependency", map[string]string{"A": "---- MODULE A ----\nEXTENDS M\n====\n"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			files := map[string]string{"M.tla": "---- MODULE M ----\n" + tc.root + "\n====\n", "M.cfg": ""}
			for name, source := range tc.modules {
				files[name+".tla"] = source
			}
			for name, source := range files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != 150 || !strings.Contains(errout.String(), tc.message) {
				t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				runJavaTLCForExit(t, jar, dir, "M", 150)
			}
		})
	}
}

// testa, its configuration, and test.tla are unchanged Java TLC v1.7.4 fixtures.
func TestJavaTLCImportedModuleNameFixture(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"testa.tla", "testa.cfg", "test.tla"} {
		data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want := "File name 'test' does not match the name 'Foo'"
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{filepath.Join(dir, "testa.tla")}, &out, &errout)
	if code != 150 || !strings.Contains(errout.String(), want) {
		t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		if java := runJavaTLCForExit(t, jar, dir, "testa", 150); !strings.Contains(java, want) {
			t.Fatalf("Java TLC:\n%s", java)
		}
	}
}
