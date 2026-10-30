package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJavaTLCTemporalNextEvaluation(t *testing.T) {
	for _, tc := range []struct {
		name, init, next, message string
		code                      int
	}{
		{"empty initial states", "FALSE /\\ x = 0", "[](x = 0)", "0 states generated, 0 distinct states found", 0},
		{"empty initial states box action", "FALSE /\\ x = 0", "[][FALSE /\\ x' = x]_x", "0 states generated, 0 distinct states found", 0},
		{"always", "x = 0", "[](x = 0)", "temporal formula ([]A)", 75},
		{"eventually", "x = 0", "<>(x = 0)", "temporal formula (<>A)", 75},
		{"leads to", "x = 0", "(x = 0) ~> (x = 1)", "temporal formula (a ~> b)", 75},
		{"guarantees", "x = 0", "(x = 0) -+-> (x = 1)", "temporal formula (a -+-> formula)", 75},
		{"weak fairness", "x = 0", "WF_x(x' = x)", "temporal formula (WF)", 75},
		{"strong fairness", "x = 0", "SF_x(x' = x)", "temporal formula (SF)", 75},
		{"short circuit conjunction", "x = 0", "FALSE /\\ [](x = 0)", "Deadlock reached", 11},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, "VARIABLE x\nInit == "+tc.init+"\nNext == "+tc.next, "INIT Init\nNEXT Next\n")
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != tc.code || !strings.Contains(out.String(), tc.message) {
				t.Fatalf("Go exit %d want %d: %s %s", code, tc.code, out.String(), errout.String())
			}
			if tc.code == 75 && (!strings.Contains(out.String(), "@!@!@STARTMSG 2261:0") || !strings.Contains(out.String(), "@!@!@STARTMSG 2103:1")) {
				t.Fatalf("missing temporal error or evaluation stack: %s", out.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runJavaTLCForExit(t, jar, dir, "M", tc.code)
				if !strings.Contains(java, tc.message) {
					t.Fatalf("Java output: %s", java)
				}
			}
		})
	}
}

func TestJavaTLCTemporalActionLevels(t *testing.T) {
	for _, tc := range []struct {
		name, next string
		code       int
	}{
		{"and", "(x' = x) /\\ [](x = 0)", 150},
		{"or", "(x' = x) \\/ [](x = 0)", 150},
		{"implies", "(x' = x) => [](x = 0)", 150},
		{"reverse implies", "[](x = 0) => (x' = x)", 150},
		{"equivalence", "(x' = x) <=> [](x = 0)", 150},
		{"equality", "(x' = x) = [](x = 0)", 150},
		{"inequality", "(x' = x) # [](x = 0)", 150},
		{"tuple", "<<(x' = x), [](x = 0)>> = <<TRUE, TRUE>>", 150},
		{"set", "{[](x = 0)} = {}", 150},
		{"membership", "[](x = 0) \\in BOOLEAN", 150},
		{"defined operators", "Act /\\ Temp", 150},
		{"action passed to temporal operator", "F(x' = x)", 0},
		{"unused temporal argument", "<<Const([](x = 0))>> = <<TRUE>>", 0},
		{"unused temporal let definition", "<<(LET T == [](x = 0) IN TRUE)>> = <<TRUE>>", 0},
		{"used temporal argument", "<<Id([](x = 0))>> = <<TRUE>>", 150},
		{"used temporal let definition", "<<(LET T == [](x = 0) IN T)>> = <<TRUE>>", 150},
		{"action and state", "(x' = x) /\\ (x = 0)", 0},
		{"temporal and state", "(x = 0) /\\ [](x = 0)", 0},
		{"conditional", "IF [](x = 0) THEN x' = x ELSE TRUE", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nAct == x' = x\nTemp == [](x = 0)\nF(p) == p /\\ Temp\nConst(p) == TRUE\nId(p) == p\nInit == FALSE /\\ x = 0\nNext == " + tc.next
			writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\n")
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

func TestJavaTLCTemporalStatePredicates(t *testing.T) {
	for _, tc := range []struct {
		name, init, config string
		code               int
	}{
		{"temporal initial predicate", "[](x = 0)", "", 75},
		{"unreached temporal initial branch", "IF FALSE THEN []TRUE ELSE x = 0", "", 0},
		{"empty state constraint", "FALSE /\\ x = 0", "CONSTRAINT AC\n", 0},
		{"state constraint", "x = 0", "CONSTRAINT AC\n", 75},
		{"empty action constraint", "FALSE /\\ x = 0", "ACTION_CONSTRAINT AC\n", 0},
		{"action constraint", "x = 0", "ACTION_CONSTRAINT AC\n", 75},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == " + tc.init + "\nNext == x' = x\nAC == [](x = 0)"
			writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\n"+tc.config)
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != tc.code || tc.code == 75 && !strings.Contains(out.String(), "@!@!@STARTMSG 2261:0") {
				t.Fatalf("Go exit %d want %d: %s %s", code, tc.code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				runJavaTLCForExit(t, jar, dir, "M", tc.code)
			}
		})
	}
}

func TestJavaTLCUpstreamStandardModules(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"StandardModules.tla", "StandardModules.cfg"} {
		data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-tool", filepath.Join(dir, "StandardModules.tla")}, &out, &errout)
	stats := "0 states generated, 0 distinct states found, 0 states left on queue."
	if code != 0 || !strings.Contains(out.String(), stats) {
		t.Fatalf("Go exit %d: %s %s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java := runJavaTLCForExit(t, jar, dir, "StandardModules", 0)
		if !strings.Contains(java, stats) {
			t.Fatalf("Java output: %s", java)
		}
	}
}

func TestTemporalFailureHumanReadable(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == [](x = 0)", "INIT Init\nNEXT Next\n")
	var out, errout bytes.Buffer
	if code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout); code != 75 || !strings.Contains(out.String(), "temporal formula ([]A)") {
		t.Fatalf("exit %d: %s %s", code, out.String(), errout.String())
	}
}
