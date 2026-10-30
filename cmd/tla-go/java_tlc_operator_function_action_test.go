package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestJavaTLCOperatorParameterizedFunctionAction(t *testing.T) {
	dir := t.TempDir()
	for name, source := range map[string]string{
		"A.tla": "---- MODULE A ----\nEXTENDS Naturals\nCONSTANT F(_)\nVARIABLE v\nStep[i \\in 1..2] == /\\ v < 5 /\\ v' = v + F(i)\n====\n",
		"M.tla": "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInc(n) == n + 1\nP(Op(_)) == INSTANCE A WITH F <- Op, v <- x\nInit == x = 0\nNext == P(Inc)!Step[1]\n====\n",
		"M.cfg": "INIT Init\nNEXT Next\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", filepath.Join(dir, "M.tla")}, &out, &errout)
	checkMultipleConfigResult(t, code, out.String(), errout.String(), 11, "4/4/0")
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java := runMultipleConfigJava(t, jar, dir, "M", "M", 11, "4/4/0")
		compareRawToolTrace(t, java, out.String())
	}
}

func TestJavaTLCOperatorFunctionActionScopes(t *testing.T) {
	for _, tc := range []struct {
		name, module, constants, body, instance, next, counts string
	}{
		{"dependent bound", "A", "F(_)", "Step[i \\in 1..F(2)] == /\\ v < 5 /\\ v' = v + F(i)", "P(Op(_)) == INSTANCE A WITH F <- Op, v <- x", "P(Inc)!Step[3]", "3/3/0"},
		{"lambda captures bound", "A", "F(_)", "Step[i \\in 1..F(2)] == /\\ v < 5 /\\ v' = v + F(i)", "P(Op(_)) == INSTANCE A WITH F <- Op, v <- x", "\\E z \\in 1..2 : P(LAMBDA n: n + z)!Step[1]", "9/7/0"},
		{"capture avoidance", "A", "F(_)", "Step[i \\in 1..2] == /\\ v < 5 /\\ \\E z \\in {100}: v' = v + F(i)", "P(Op(_)) == INSTANCE A WITH F <- Op, v <- x", "\\E z \\in 1..2 : P(LAMBDA n: n + z)!Step[1]", "9/7/0"},
		{"binary and value parameters", "A", "F(_,_), Limit", "Step[i \\in 1..2] == /\\ v < Limit /\\ v' = F(v, i)", "P(k, Op(_,_)) == INSTANCE A WITH Limit <- k, F <- Op, v <- x", "P(5, Add)!Step[1]", "6/6/0"},
		{"builtin operator", "A", "F(_,_), Limit", "Step[i \\in 1..2] == /\\ v < Limit /\\ v' = F(v, i)", "P(k, Op(_,_)) == INSTANCE A WITH Limit <- k, F <- Op, v <- x", "P(5, +)!Step[1]", "6/6/0"},
		{"enabled", "A", "F(_)", "Step[i \\in 1..F(2)] == /\\ v < 5 /\\ v' = v + F(i)", "P(Op(_)) == INSTANCE A WITH F <- Op, v <- x", "IF ENABLED P(Inc)!Step[1] THEN P(Inc)!Step[1] ELSE UNCHANGED x", "5/4/0"},
		{"local operator", "A", "F(_)", "Step[i \\in 1..F(2)] == /\\ v < 5 /\\ v' = v + F(i)", "P(Op(_)) == INSTANCE A WITH F <- Op, v <- x", "LET Local(n) == n + 1 IN P(Local)!Step[1]", "4/4/0"},
		{"local operator captures bound", "A", "F(_)", "Step[i \\in 1..F(2)] == /\\ v < 5 /\\ v' = v + F(i)", "P(Op(_)) == INSTANCE A WITH F <- Op, v <- x", "\\E z \\in 1..2 : LET Local(n) == n + z IN P(Local)!Step[1]", "9/7/0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			module := fmt.Sprintf("---- MODULE %s ----\nEXTENDS Naturals\nCONSTANT %s\nVARIABLE v\n%s\n====\n", tc.module, tc.constants, tc.body)
			source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInc(n) == n + 1\nAdd(a,b) == a + b\n" + tc.instance + "\nInit == x = 0\nNext == " + tc.next + "\n====\n"
			for name, content := range map[string]string{"A.tla": module, "M.tla": source, "M.cfg": "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			dot := filepath.Join(dir, "go.dot")
			code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", "-dump", "dot,actionlabels", dot, filepath.Join(dir, "M.tla")}, &out, &errout)
			checkMultipleConfigResult(t, code, out.String(), errout.String(), 0, tc.counts)
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				runMultipleConfigJava(t, jar, dir, "M", "M", 0, tc.counts)
				graph, err := os.ReadFile(dot)
				if err != nil {
					t.Fatal(err)
				}
				compareJavaDOT(t, jar, dir, graph, code)
			}
		})
	}
}

func TestJavaTLCOperatorInstanceArgumentLevels(t *testing.T) {
	for _, tc := range []struct {
		name, dependency, definitions, next, counts string
		code                                        int
	}{
		{"stateful named argument", "VARIABLE v\nStep[i \\in 1..2] == /\\ v < 5 /\\ v' = v + F(i)", "P(Op(_)) == INSTANCE A WITH F <- Op, v <- x", "P(Inc)!Step[1]", "", 150},
		{"stateful lambda argument", "VARIABLE v\nStep[i \\in 1..2] == /\\ v < 5 /\\ v' = v + F(i)", "P(Op(_)) == INSTANCE A WITH F <- Op, v <- x", "P(LAMBDA n: n + x)!Step[1]", "", 150},
		{"stateful local argument", "VARIABLE v\nStep[i \\in 1..2] == /\\ v < 5 /\\ v' = v + F(i)", "P(Op(_)) == INSTANCE A WITH F <- Op, v <- x", "LET Local(n) == n + x IN P(Local)!Step[1]", "", 150},
		{"forwarded stateful argument", "VARIABLE v\nStep[i \\in 1..2] == /\\ v < 5 /\\ v' = v + F(i)", "P(Op(_)) == INSTANCE A WITH F <- Op, v <- x\nWrap(Op(_)) == P(Op)!Step[1]", "Wrap(Inc)", "", 150},
		{"indirect operator use", "VARIABLE v\nG(n) == F(n)\nStep[i \\in 1..2] == /\\ v < 5 /\\ v' = v + G(i)", "P(Op(_)) == INSTANCE A WITH F <- Op, v <- x", "P(Inc)!Step[1]", "", 150},
		{"unused argument", "VARIABLE v\nStep[i \\in 1..2] == /\\ v < 5 /\\ v' = v + i", "P(Op(_)) == INSTANCE A WITH F <- Op, v <- x", "P(Inc)!Step[1]", "6/6/0", 0},
		{"operator used only by another export", "VARIABLE v\nOther == F(1)\nStep[i \\in 1..2] == /\\ v < 5 /\\ v' = v + i", "P(Op(_)) == INSTANCE A WITH F <- Op, v <- x", "P(Inc)!Step[1]", "6/6/0", 0},
		{"direct stateful substitution", "VARIABLE v\nStep[i \\in 1..2] == /\\ v < 5 /\\ v' = v + F(i)", "INSTANCE A WITH F <- Inc, v <- x", "Step[1]", "", 150},
		{"stateful substitution in constant module", "Value == F(1)", "INSTANCE A WITH F <- Inc", "IF x < 5 THEN x' = Value ELSE x' = x", "7/6/0", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			files := map[string]string{
				"A.tla": "---- MODULE A ----\nEXTENDS Naturals\nCONSTANT F(_)\n" + tc.dependency + "\n====\n",
				"M.tla": "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInc(n) == n + x\n" + tc.definitions + "\nInit == x = 0\nNext == " + tc.next + "\n====\n",
				"M.cfg": "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n",
			}
			for name, content := range files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				runMultipleConfigJava(t, jar, dir, "M", "M", tc.code, tc.counts)
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", filepath.Join(dir, "M.tla")}, &out, &errout)
			checkMultipleConfigResult(t, code, out.String(), errout.String(), tc.code, tc.counts)
		})
	}
}
