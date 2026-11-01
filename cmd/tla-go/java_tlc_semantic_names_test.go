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

// These original models compare INSTANCE reference order with TLC 1.7.4.
func TestJavaTLCInstanceReferenceOrder(t *testing.T) {
	for _, separator := range []string{" ", "\n"} {
		t.Run(fmt.Sprintf("separator %q", separator), func(t *testing.T) {
			dir := t.TempDir()
			files := map[string]string{
				"M.tla": "---- MODULE M ----\nBad == I!F(1)" + separator + "I == INSTANCE A\n====\n",
				"A.tla": "---- MODULE A ----\nF(x) == x\n====\n",
				"M.cfg": "",
			}
			for name, data := range files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != 150 || errout.Len() != 0 || !hasToolMessage(readToolMessages(out.String()), 3002) {
				t.Fatalf("Go exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, []string{"-tool", "-workers", "1", "-fp", "0", "M.tla"})
				if javaCode != 150 || !hasToolMessage(readToolMessages(java), 3002) {
					t.Fatalf("Java TLC exit %d:\n%s", javaCode, java)
				}
			}
		})
	}
}
