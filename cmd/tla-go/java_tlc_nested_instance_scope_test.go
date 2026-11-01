package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJavaTLCEarlyInstanceBeforeSameNamedNestedModule(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "external module missing", true: "duplicate external module"}[external], func(t *testing.T) {
			dir := t.TempDir()
			files := map[string]string{
				"M.tla": "---- MODULE M ----\nJ == INSTANCE E\n---- MODULE E ---- A == 2 ====\n---- MODULE R ---- V == J!A ====\nI == INSTANCE R\nVARIABLE x\nInit == x = I!V\nNext == UNCHANGED x\n====\n",
				"M.cfg": "INIT Init\nNEXT Next\n",
			}
			if external {
				files["E.tla"] = "---- MODULE E ----\nA == 1\n====\n"
			}
			for name, source := range files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != 150 || errout.Len() != 0 || !hasToolMessage(readToolMessages(out.String()), 3002) {
				t.Fatalf("Go exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if strings.Contains(out.String(), "duplicate nested module E") != external {
				t.Fatalf("Go reported the wrong module error: %s", out.String())
			}
			if jar := pinnedFairnessEffectJar(t); jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, []string{"-tool", "-workers", "1", "-fp", "0", "M.tla"})
				if javaCode != 150 || !hasToolMessage(readToolMessages(java), 3002) {
					t.Fatalf("Java TLC exit %d:\n%s", javaCode, java)
				}
				if strings.Contains(java, "Multiply-defined module 'E'") != external {
					t.Fatalf("Java TLC reported the wrong module error: %s", java)
				}
			}
		})
	}
}
