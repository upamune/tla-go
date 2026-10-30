package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
)

func TestJavaTLCDuplicateRecordFields(t *testing.T) {
	upstream, err := os.ReadFile(filepath.Join("testdata", "java-tlc", "etest16.tla"))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, source, location string
	}{
		{"upstream", strings.Replace(string(upstream), "MODULE etest16", "MODULE M", 1), "line 5, column 27"},
		{"record set", "---- MODULE M ----\nUnused == [a : {1}, b : {2}, a : {1}]\n====\n", "line 2, column 30"},
		{"nested", "---- MODULE M ----\nUnused == [a |-> [b |-> 1, b |-> 2]]\n====\n", "line 2, column 28"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parser.ParseModule(tc.source); err != nil {
				t.Fatalf("syntax parser rejected semantic error: %v", err)
			}
			dir := t.TempDir()
			path := filepath.Join(dir, "M.tla")
			if err := os.WriteFile(path, []byte(tc.source), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "M.cfg"), nil, 0o600); err != nil {
				t.Fatal(err)
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{path}, &out, &errout)
			if code != 150 || !strings.Contains(errout.String(), "Non-unique fields in constructor") || !strings.Contains(errout.String(), tc.location) {
				t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runJavaTLCForExit(t, jar, dir, "M", 150)
				if !strings.Contains(java, "Non-unique fields in constructor") {
					t.Fatalf("Java TLC:\n%s", java)
				}
			}
		})
	}
}
