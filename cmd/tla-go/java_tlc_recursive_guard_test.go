package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJavaTLCRecursiveFunctionInActionGuard(t *testing.T) {
	for _, tc := range []struct{ name, body, stats string }{
		{"single witness", "Next == /\\ (LET F[n \\in 0..2] == IF n = 0 THEN TRUE ELSE F[n-1] IN F[x])\n        /\\ x' = IF x < 2 THEN x + 1 ELSE x", "4 states generated, 3 distinct states found"},
		{"multiple witnesses", "Next == /\\ (LET F[n \\in 0..2] == IF n = 0 THEN TRUE ELSE (F[n-1] \\/ F[n-1]) IN F[2])\n        /\\ UNCHANGED x", "5 states generated, 1 distinct states found"},
		{"recursive action", "Next == LET F[n \\in 0..2] == IF n = 0 THEN x' = 1 ELSE (F[n-1] \\/ F[n-1]) IN F[2]", "9 states generated, 2 distinct states found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\n"+tc.body, "INIT Init\nNEXT Next\n")
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != 0 || errout.Len() != 0 || !strings.Contains(out.String(), tc.stats) {
				t.Fatalf("exit %d; stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaToolMessages(t, jar, dir, code, readToolMessages(out.String()))
			}
		})
	}
}
