package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestJavaTLCProofLevelDeclarations(t *testing.T) {
	for _, tc := range []struct {
		name, statement string
		code            int
	}{
		{"state action temporal", "ASSUME STATE P, ACTION A, TEMPORAL F PROVE P = P", 0},
		{"new temporal", "ASSUME NEW TEMPORAL F PROVE TRUE", 0},
		{"bounded constant", "ASSUME CONSTANT x \\in {1} PROVE x = 1", 0},
		{"bounded state", "ASSUME STATE x \\in {1} PROVE x = 1", 150},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, contents := range map[string]string{"M.tla": "---- MODULE M ----\nTHEOREM T == " + tc.statement + "\n====\n", "M.cfg": ""} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != tc.code {
				t.Fatalf("exit %d, want %d: %s %s", code, tc.code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				runJavaTLCForExit(t, jar, dir, "M", tc.code)
			}
		})
	}
}
