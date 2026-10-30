package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestJavaTLCAxiomsAreNotEvaluated(t *testing.T) {
	for _, tc := range []struct {
		name, declaration, invariant string
		code                         int
	}{
		{"false axiom", "AXIOM FALSE", "TRUE", 0},
		{"unevaluable axiom", "AXIOM \\A n : n = n", "TRUE", 0},
		{"false assumption", "ASSUME FALSE", "TRUE", 10},
		{"named axiom remains an operator", "AXIOM A == FALSE", "A \\/ x = 0", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\n" + tc.declaration + "\nInit == x = 0\nNext == UNCHANGED x\nInv == " + tc.invariant
			writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nINVARIANT Inv\n")
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != tc.code || errout.Len() != 0 {
				t.Fatalf("exit %d, want %d; stdout:\n%s\nstderr:\n%s", code, tc.code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaToolMessages(t, jar, dir, code, readToolMessages(out.String()))
			}
		})
	}
}
