package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJavaTLCInstanceStateArgumentLevels(t *testing.T) {
	for _, tc := range []struct {
		name, body, root string
		code             int
	}{
		{"used state", `Foo == y = 0`, `P == I(x)!Foo`, 0},
		{"used constant", `Foo == y' = 0`, `P == I(1)!Foo`, 0},
		{"used action", `Foo == y = 0`, `P == I(x')!Foo`, 150},
		{"primed action", `Foo == y' = 0`, `P == I(x')!Foo`, 150},
		{"set of action", `Foo == y' = 0`, `P == I({x'})!Foo`, 150},
		{"unused variable", `Foo == TRUE`, `P == I(x')!Foo`, 0},
		{"indirect variable", "F == y = 0\nFoo == F", `P == I(x')!Foo`, 150},
		{"indirect unused", "F == TRUE\nFoo == F", `P == I(x')!Foo`, 0},
		{"unused operator argument", "F(z) == TRUE\nFoo == F(y)", `P == I(x')!Foo`, 150},
		{"enabled inside export", `Foo == ENABLED (y' = y)`, `P == I(x')!Foo`, 150},
		{"enabled argument", `Foo == y' = 0`, `P == I(ENABLED (x' = x))!Foo`, 0},
		{"forwarded action", `Foo == y' = 0`, "G(z) == I(z)!Foo\nP == G(x')", 150},
		{"forwarded state", `Foo == y' = 0`, "G(z) == I(z)!Foo\nP == G(x)", 0},
		{"forwarded unused argument", `Foo == y' = 0`, "K(z) == TRUE\nG(z) == I(K(z))!Foo\nP == G(x')", 0},
		{"forwarded enabled argument", `Foo == y' = 0`, "G(z) == I(ENABLED z)!Foo\nP == G(x' = x)", 0},
		{"forwarded unused let", `Foo == y' = 0`, "G(z) == I(LET H == z IN TRUE)!Foo\nP == G(x')", 0},
		{"forwarded used let", `Foo == y' = 0`, "G(z) == I(LET H == z IN H)!Foo\nP == G(x')", 150},
		{"forwarded unused primed let", `Foo == y' = 0`, "G(z) == I(LET H == z' IN TRUE)!Foo\nP == G(x')", 150},
		{"forwarded unused temporal let", `Foo == y' = 0`, "G(z) == I(LET H == []z IN TRUE)!Foo\nP == G(x')", 0},
		{"forwarded twice", `Foo == y' = 0`, "G(z) == I(z)!Foo\nH(v) == G(v)\nP == H(x')", 150},
		{"quantified action bound", `Foo == y' = 0`, `P == \A i \in {x'} : I(i)!Foo`, 0},
		{"action inside enabled", `Foo == y' = 0`, `P == ENABLED I(x')!Foo`, 150},
		{"unused let", `Foo == LET F == y IN TRUE`, `P == I(x')!Foo`, 0},
		{"used let", `Foo == LET F == y IN F = 0`, `P == I(x')!Foo`, 150},
		{"unused primed let", `Foo == LET F == y' IN TRUE`, `P == I(x')!Foo`, 150},
		{"bound shadows later action", `Foo == y = 0`, "P == I(\\A q \\in {1}: q = 1)!Foo\nq == x'", 0},
		{"bound shadows later temporal", `Foo == y = 0`, "P == I(\\A q \\in {1}: q = 1)!Foo\nq == [](x = 0)", 0},
		{"local parameter shadows later action", `Foo == y = 0`, "P == I(LET Q(q) == q = 1 IN Q(1))!Foo\nq == x'", 0},
		{"unnamed theorem", `Foo == y' = 0`, `THEOREM I(x')!Foo`, 150},
		{"function domain used", `Foo[i \in {y}] == i`, `P == I(x')!Foo[1]`, 150},
		{"function domain unused", `Foo[i \in {1}] == i`, `P == I(x')!Foo[1]`, 0},
		{"function body used", `Foo[i \in {1}] == y`, `P == I(x')!Foo[1]`, 150},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			module := "---- MODULE A ----\nEXTENDS Naturals\nVARIABLE y\n" + tc.body + "\n====\n"
			root := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nI(z) == INSTANCE A WITH y <- z\nInit == x = 0\nNext == UNCHANGED x\n" + tc.root + "\n====\n"
			for name, contents := range map[string]string{"A.tla": module, "M.tla": root, "M.cfg": "INIT Init\nNEXT Next\n"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != tc.code {
				t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				runJavaTLCForExit(t, jar, dir, "M", tc.code)
			}
		})
	}
}

// test217 and test217a are unchanged Java TLC v1.7.4 fixtures.
func TestJavaTLCInstanceStateLevelFixture(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"test217.tla", "test217.cfg", "test217a.tla"} {
		data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want := "Level error in applying operator I!Foo"
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{filepath.Join(dir, "test217.tla")}, &out, &errout)
	if code != 150 || !strings.Contains(errout.String(), want) {
		t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		if java := runJavaTLCForExit(t, jar, dir, "test217", 150); !strings.Contains(java, want) {
			t.Fatalf("Java TLC:\n%s", java)
		}
	}
}

func TestJavaTLCInstanceMultipleStateArguments(t *testing.T) {
	for _, tc := range []struct {
		name, expression string
		code             int
	}{
		{"both state", `I(x, x)!Foo`, 0},
		{"unused action argument", `I(x, x')!Foo`, 0},
		{"used action argument", `I(x', x)!Foo`, 150},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			module := "---- MODULE A ----\nEXTENDS Naturals\nVARIABLES y,w\nFoo == y = 0\n====\n"
			root := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nI(a,b) == INSTANCE A WITH y <- a, w <- b\nInit == x = 0\nNext == UNCHANGED x\nP == " + tc.expression + "\n====\n"
			for name, contents := range map[string]string{"A.tla": module, "M.tla": root, "M.cfg": "INIT Init\nNEXT Next\n"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != tc.code {
				t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				runJavaTLCForExit(t, jar, dir, "M", tc.code)
			}
		})
	}
}
