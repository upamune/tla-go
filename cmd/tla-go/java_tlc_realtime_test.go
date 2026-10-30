package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJavaTLCEmbeddedRealModules(t *testing.T) {
	for _, tc := range []struct {
		name, source, config, localReals, stats string
		code                                    int
	}{
		{"Reals value", "EXTENDS Reals\nASSUME Real = \"Reals\"", "", "", "0 states generated, 0 distinct states found, 0 states left on queue.", 0},
		{"Reals named instance", "R == INSTANCE Reals\nASSUME R!Real = \"Reals\"", "", "", "0 states generated, 0 distinct states found, 0 states left on queue.", 0},
		{"Reals inherited operators", "R == INSTANCE Reals\nASSUME /\\ 1 \\in R!Int /\\ 1 \\in R!Nat /\\ R!+(1, 2) = 3", "", "", "0 states generated, 0 distinct states found, 0 states left on queue.", 0},
		{"Reals unnamed instance", "INSTANCE Reals\nASSUME Real = \"Reals\"", "", "", "0 states generated, 0 distinct states found, 0 states left on queue.", 0},
		{"Reals unnamed operators", "INSTANCE Reals\nASSUME /\\ 1 \\in Int /\\ 1 + 2 = 3", "", "", "0 states generated, 0 distinct states found, 0 states left on queue.", 0},
		{"instance does not import Int", "R == INSTANCE Reals\nASSUME Int = Int", "", "", "", 150},
		{"Reals model", "EXTENDS Reals\nVARIABLE x\nInit == x = 0\nNext == x' = 1 - x\nInv == Real = \"Reals\"", "INIT Init\nNEXT Next\nINVARIANT Inv\n", "", "3 states generated, 2 distinct states found, 0 states left on queue.", 0},
		{"RealTime clock", "EXTENDS RealTime\nInit == now = 0\nNext == now' = 1 - now\nInv == now \\in {0, 1}", "INIT Init\nNEXT Next\nINVARIANT Inv\n", "", "3 states generated, 2 distinct states found, 0 states left on queue.", 0},
		{"RealTime no states", "EXTENDS RealTime\nVARIABLE x\nInit == FALSE /\\ x = 0\nNext == UNCHANGED <<x, now>>", "INIT Init\nNEXT Next\n", "", "0 states generated, 0 distinct states found, 0 states left on queue.", 0},
		{"RealTime missing clock", "EXTENDS RealTime\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED <<x, now>>", "INIT Init\nNEXT Next\n", "", "", 255},
		{"local override", "EXTENDS Reals\nASSUME Real = \"local\"", "", "---- MODULE Reals ----\nReal == \"local\"\n====\n", "0 states generated, 0 distinct states found, 0 states left on queue.", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			files := map[string]string{"M.tla": "---- MODULE M ----\n" + tc.source + "\n====\n", "M.cfg": tc.config}
			if tc.localReals != "" {
				files["Reals.tla"] = tc.localReals
			}
			for name, contents := range files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != tc.code || (tc.stats != "" && !strings.Contains(out.String(), tc.stats)) {
				t.Fatalf("Go exit %d want %d: %s %s", code, tc.code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runJavaTLCForExit(t, jar, dir, "M", tc.code)
				if tc.stats != "" && !strings.Contains(java, tc.stats) {
					t.Fatalf("Java output: %s", java)
				}
			}
		})
	}
}

func TestJavaTLCInstanceStandardImports(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		code       int
	}{
		{"arithmetic in instance", "I == INSTANCE A\nASSUME I!F = 5", 0},
		{"standard name remains local", "I == INSTANCE A\nASSUME Nat = Nat", 150},
		{"root operator does not shadow instance", "a + b == 42\nI == INSTANCE A\nASSUME I!F = 5\nASSUME 2 + 3 = 42", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			files := map[string]string{
				"A.tla": "---- MODULE A ----\nEXTENDS Naturals\nF == 2 + 3\n====\n",
				"M.tla": "---- MODULE M ----\n" + tc.body + "\n====\n",
				"M.cfg": "",
			}
			for name, contents := range files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != tc.code {
				t.Fatalf("Go exit %d want %d: %s %s", code, tc.code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				runJavaTLCForExit(t, jar, dir, "M", tc.code)
			}
		})
	}
}

func TestJavaTLCInstanceStandardBindings(t *testing.T) {
	for _, tc := range []struct {
		name, dependency, root string
	}{
		{"parameterized arithmetic", "EXTENDS Naturals\nCONSTANT y\nNonnegative == y >= 0", "I(z) == INSTANCE A WITH y <- z\nASSUME I(1)!Nonnegative"},
		{"sequences", "EXTENDS Sequences\nF == Len(<<1, 2>>)", "Len(s) == 99\nI == INSTANCE A\nASSUME I!F = 2 /\\ Len(<<>>) = 99"},
		{"finite sets", "EXTENDS FiniteSets\nF == Cardinality({1, 2})", "Cardinality(s) == 99\nI == INSTANCE A\nASSUME I!F = 2 /\\ Cardinality({}) = 99"},
		{"bags", "EXTENDS Bags\nF == EmptyBag", "EmptyBag == \"shadow\"\nI == INSTANCE A\nASSUME I!F = <<>> /\\ EmptyBag = \"shadow\""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			files := map[string]string{
				"A.tla": "---- MODULE A ----\n" + tc.dependency + "\n====\n",
				"M.tla": "---- MODULE M ----\n" + tc.root + "\n====\n",
				"M.cfg": "",
			}
			for name, contents := range files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				runJavaTLCForExit(t, jar, dir, "M", 0)
			}
			var out, errout bytes.Buffer
			if code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout); code != 0 {
				t.Fatalf("Go exit %d: %s %s", code, out.String(), errout.String())
			}
		})
	}
}

func TestJavaTLCHigherOrderInstanceInheritedBuiltin(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"A.tla": "---- MODULE A ----\nEXTENDS Naturals\nCONSTANT G(_)\nF == G(1)\n====\n",
		"M.tla": "---- MODULE M ----\nInc(n) == n\nP(F(_)) == INSTANCE A WITH G <- F\nASSUME P(Inc)!+(1, 2) = 3\n====\n",
		"M.cfg": "",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	if code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout); code != 10 || !strings.Contains(out.String(), "array is not of length 2") {
		t.Fatalf("Go exit %d: %s %s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java := runJavaTLCForExit(t, jar, dir, "M", 10)
		if !strings.Contains(java, "array is not of length 2") {
			t.Fatalf("Java output: %s", java)
		}
	}
}
