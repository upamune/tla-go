package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJavaTLCUnscopedStandardValueSubstitution(t *testing.T) {
	for _, tc := range []struct{ name, module, target, members string }{
		{"naturals", "Naturals", "Nat", "0..2"},
		{"integers", "Integers", "Int", "-1..1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			contents := "---- MODULE M ----\nEXTENDS " + tc.module + "\nVARIABLE x\nBound == " + tc.members + "\nInit == x = 0\nNext == x' \\in " + tc.target + "\n====\n"
			for name, data := range map[string]string{"M.tla": contents, "M.cfg": "INIT Init\nNEXT Next\nCONSTANT " + tc.target + " <- Bound\n"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != 0 || errout.Len() != 0 || !strings.Contains(out.String(), "10 states generated, 3 distinct states found") {
				t.Fatalf("exit %d; stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaToolMessages(t, jar, dir, code, readToolMessages(out.String()))
			}
		})
	}
}
