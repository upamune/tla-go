package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These models are unchanged from the Java TLC v1.7.4 test suite.
func TestJavaTLCSemanticNameErrors(t *testing.T) {
	for _, tc := range []struct{ name, message string }{
		{"etest1", "operator Foo expects 1 arguments, got 2"},
		{"etest2", "operator Foo expects 2 arguments, got 1"},
		{"etest5", "undefined symbol M!Init"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, ext := range []string{".tla", ".cfg"} {
				name := tc.name + ext
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
			if code != 150 || !strings.Contains(errout.String(), tc.message) {
				t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runJavaTLCForExit(t, jar, dir, tc.name, 150)
				if !strings.Contains(java, "Semantic errors:") {
					t.Fatalf("Java TLC:\n%s", java)
				}
			}
		})
	}
}
