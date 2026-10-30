package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// test219 is unchanged from Java TLC v1.7.4, commit 5a47802.
func TestJavaTLCParameterizedInstanceSelections(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"test219.tla", "test219a.tla", "test219.cfg", "test206a.tla"} {
		data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{filepath.Join(dir, "test219.tla")}, &out, &errout)
	stats := "0 states generated, 0 distinct states found, 0 states left on queue."
	if code != 0 || errout.Len() != 0 || !strings.Contains(out.String(), stats) {
		t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java := runJavaTLCForExit(t, jar, dir, "test219", 0)
		if !strings.Contains(java, stats) {
			t.Fatalf("Java TLC:\n%s", java)
		}
	}
}

func TestJavaTLCInstanceOperatorArgumentCapture(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"A.tla": `---- MODULE A ----
EXTENDS Naturals
CONSTANTS C, D
F(a, b) == a + b + C + D
Use(Op(_, _)) == Op(1, 2)
Result == Use(F)
====
`,
		"M.tla": `---- MODULE M ----
EXTENDS Naturals
VARIABLE x
I(p, q) == INSTANCE A WITH C <- p, D <- q
Init == x = 4
Next == UNCHANGED x
Inv == /\ I(4, 5)!Result = 12 /\ \A i \in {x} : I(i, 5)!Result = i + 8
====
`,
		"M.cfg": "INIT Init\nNEXT Next\nINVARIANT Inv\n",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	stats := "2 states generated, 1 distinct states found, 0 states left on queue."
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
	if code != 0 || errout.Len() != 0 || !strings.Contains(out.String(), stats) {
		t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java := runJavaTLCForExit(t, jar, dir, "M", 0)
		if !strings.Contains(java, stats) {
			t.Fatalf("Java TLC:\n%s", java)
		}
	}
}

func TestJavaTLCExplicitInstanceOperatorArgumentRejected(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"A.tla": "---- MODULE A ----\nEXTENDS Naturals\nCONSTANT C\nF(a, b) == a+b+C\n====\n",
		"M.tla": "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nI(p) == INSTANCE A WITH C <- p\nApply(Op(_, _), a, b) == Op(a, b)\nInit == x=0\nNext == UNCHANGED x\nInv == Apply(I(4)!F, 1, 2) = 7\n====\n",
		"M.cfg": "INIT Init\nNEXT Next\nINVARIANT Inv\n",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	if code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout); code != 255 {
		t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		runJavaTLCForExit(t, jar, dir, "M", 150)
	}
}
