package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLivenessDOTConfigOverridesPreserveExisting(t *testing.T) {
	for _, tc := range []struct{ name, declarations, assignments string }{
		{"substitution", "CONSTANT N\nThree == 3", "N <- Three"},
		{"operator table", "CONSTANT F(_)", "F(0) = 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := tc.declarations + "\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\nP == <>[](x = 0)"
			cfg := "CONSTANT " + tc.assignments + "\nINIT Init\nNEXT Next\nPROPERTY P\nCHECK_DEADLOCK FALSE\n"
			writeDOTFixture(t, dir, body, cfg)
			path := filepath.Join(dir, "out_liveness.dot")
			if err := os.WriteFile(path, []byte("existing"), 0o600); err != nil {
				t.Fatal(err)
			}
			out, code, stderr := runGoInDir(t, dir, []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final", "-dump", "dot", "out.dot", "M.tla"})
			if code != 0 || stderr != "" {
				t.Fatalf("exit %d stderr %q: %s", code, stderr, out)
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != "existing" {
				t.Fatalf("config override changed output: %q, %v", got, err)
			}
		})
	}
}
