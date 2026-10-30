package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJavaTLCNaturalsInstance(t *testing.T) {
	cases := []struct {
		name, body, text string
		code             int
	}{
		{"operators", "X == INSTANCE Naturals\nASSUME /\\ X!+(1,2)=3 /\\ X!<(1,2) /\\ X!<=(2,2) /\\ X!>(3,2) /\\ X!>=(3,3) /\\ X!*(2,3)=6 /\\ X!\\div(5,2)=2 /\\ X!%(5,2)=1 /\\ X!..(1,2)={1,2}\nASSUME 1 \\in X!Nat", "", 0},
		{"no unqualified import", "X == INSTANCE Naturals\nASSUME 0 \\in Nat", "undefined symbol Nat", 150},
		{"unqualified instance", "INSTANCE Naturals\nASSUME 1 + 2 = 3", "", 0},
		{"unused parameter", "X(p) == INSTANCE Naturals", "", 0},
		{"parameterized call", "X(p) == INSTANCE Naturals\nASSUME X(1)!<(1,2)", "array is not of length 2", 10},
		{"invalid substitution", "X == INSTANCE Naturals WITH Nat <- {}", "invalid instance substitution", 150},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			files := map[string]string{
				"M.tla": "---- MODULE M ----\n" + tc.body + "\nVARIABLE x\nInit == x=0\nNext == UNCHANGED x\n====\n",
				"M.cfg": "INIT Init\nNEXT Next\n",
			}
			for name, content := range files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != tc.code || tc.text != "" && !strings.Contains(out.String()+errout.String(), tc.text) {
				t.Fatalf("exit %d, want %d; stdout %s; stderr %s", code, tc.code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaInstance(t, jar, dir, code)
			}
		})
	}
}
