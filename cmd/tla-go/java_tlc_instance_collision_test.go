package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Original examples compared with TLC 1.7.4; the hidden conflicting action
// must not replace the first visible symbol or leak its internal alias.
func TestJavaTLCInstanceCollisionDOT(t *testing.T) {
	for _, tc := range []struct{ name, setup, next, label string }{
		{"anonymous", "INSTANCE A\nINSTANCE B", "Other", "Other"},
		{"let", "I == INSTANCE A", "LET I == INSTANCE B IN I!Other", "I!Other"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			files := map[string]string{
				"M.tla": "---- MODULE M ----\nVARIABLE x\n" + tc.setup + "\nInit == x = 0\nNext == " + tc.next + "\n====\n",
				"M.cfg": "INIT Init\nNEXT Next\n",
				"A.tla": "---- MODULE A ----\nVARIABLE x\nStep == x' = 1\n====\n",
				"B.tla": "---- MODULE B ----\nVARIABLE x\nStep == x' = 2\nOther == Step\n====\n",
			}
			for name, data := range files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(dir, "go.dot")
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-dump", "dot,actionlabels", path, filepath.Join(dir, "M.tla")}
			var out, errout bytes.Buffer
			if code := run(context.Background(), args, &out, &errout); code != 0 || errout.Len() != 0 {
				t.Fatalf("Go exit %d stdout %s stderr %s", code, &out, &errout)
			}
			graph, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(graph), `label="`+tc.label+`"`) || !strings.Contains(string(graph), `x = 2`) || strings.Contains(string(graph), "!instance-binding!") {
				t.Fatalf("unexpected graph: %s", graph)
			}
			if jar := pinnedFairnessEffectJar(t); jar != "" {
				javaPath := filepath.Join(dir, "java.dot")
				java, code := runJavaTLCArgs(t, jar, dir, []string{"-tool", "-workers", "1", "-fp", "0", "-dump", "dot,actionlabels", javaPath, "M.tla"})
				if code != 0 {
					t.Fatalf("Java exit %d: %s", code, java)
				}
				want, err := os.ReadFile(javaPath)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(graph, want) {
					t.Fatalf("Java graph:\n%s\nGo graph:\n%s", want, graph)
				}
			}
		})
	}
}

// The existing effectful WITH substitution gap remains: Java evaluates this
// expression once and tla-go twice. A collision must not add another replay.
func TestJavaTLCInstanceCollisionWithEffects(t *testing.T) {
	const auxiliary = "---- MODULE B ----\nEXTENDS Naturals\nCONSTANT C\nF(n) == IF C THEN 2 ELSE 3\nG(n) == F(n) + 10\n====\n"
	counts := make([]int, 0, 2)
	for _, tc := range []struct{ name, imports, expression, expected string }{
		{"no collision", "INSTANCE B WITH C <- PrintT(\"collision-effect\")", "G(1)", "12"},
		{"collision", "INSTANCE A\nINSTANCE B WITH C <- PrintT(\"collision-effect\")", "<<F(1), G(1)>>", "<<1, 12>>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			files := map[string]string{
				"M.tla": "---- MODULE M ----\nEXTENDS Naturals, TLC\n" + tc.imports + "\nVARIABLE x\nInit == x = " + tc.expression + "\nNext == UNCHANGED x\nInv == x = " + tc.expected + "\n====\n",
				"M.cfg": "INIT Init\nNEXT Next\nINVARIANT Inv\n",
				"A.tla": "---- MODULE A ----\nF(n) == 1\n====\n",
				"B.tla": auxiliary,
			}
			for name, data := range files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			if code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", filepath.Join(dir, "M.tla")}, &out, &errout); code != 0 || errout.Len() != 0 {
				t.Fatalf("Go exit %d stdout %s stderr %s", code, &out, &errout)
			}
			counts = append(counts, strings.Count(out.String(), `"collision-effect"`))
			if jar := pinnedFairnessEffectJar(t); jar != "" {
				java, code := runJavaTLCArgs(t, jar, dir, []string{"-tool", "-workers", "1", "-fp", "0", "M.tla"})
				if code != 0 || strings.Count(java, `"collision-effect"`) != 1 {
					t.Fatalf("Java exit %d output %s", code, java)
				}
			}
		})
	}
	if len(counts) != 2 || counts[0] == 0 || counts[0] != counts[1] {
		t.Fatalf("effect count changed with collision: %v", counts)
	}
}
