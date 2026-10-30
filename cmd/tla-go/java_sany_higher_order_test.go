package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJavaSANYHigherOrderMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "M.tla")
	source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nCONSTANT COp(_)\nVARIABLE x\n" +
		"Apply(Op(_), v) == Op(v)\nRelay(Op(_),v) == Apply(Op,v)\nIgnoreOp(Op(_),v) == TRUE\nCallConst(Op(_),v) == Op(0)\n" +
		"Ignore(n) == TRUE\nIdentity(n) == n\nState(n) == x = 0\nAction(n) == x' = n\nTemporal(n) == [] (x = n)\nShield(n) == ENABLED n\n" +
		"KeepFirst(a,b) == a\nBinary(Op(_,_), a, b) == Op(a, b)\n" +
		"A == Apply(Ignore,x)\nB == Apply(Action,0)\nC == Apply(Temporal,0)\n" +
		"D == Apply(LAMBDA n: TRUE, x)\nE == Binary(KeepFirst,0,x)\nF == Binary(+,x,1)\n" +
		"G == LET Local(n) == n + x IN Apply(Local,1)\nH == Apply(PrintT,x)\n" +
		"I == IgnoreOp(State,x)\nJ == CallConst(Ignore,x)\nK == CallConst(State,x)\n" +
		"L == Apply(Ignore,x' = x)\nM == Apply(Shield,x' = x)\nN == Apply(LAMBDA n: ENABLED n,x' = x)\nO == Apply(LAMBDA n: TRUE,[] (x = x))\n" +
		"P == Apply(PrintT,[] (x = x))\nQ == Binary(Print,[] (x = x),1)\nR == Binary(Assert,TRUE,[] (x = x))\nS == Binary(/\\,TRUE,[] (x = x))\nT == Relay(Identity,x)\n" +
		"U == Apply(ENABLED,TRUE)\nV == Apply(UNCHANGED,TRUE)\nW == Apply([],TRUE)\nX == Binary(~>,TRUE,TRUE)\nY == Binary(\\cdot,TRUE,TRUE)\nZ == Apply(COp,[] (x = x))\n====\n"
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errout bytes.Buffer
	if code := run(context.Background(), []string{"-sany-metadata", path}, &out, &errout); code != 0 {
		t.Fatalf("metadata exit %d: %s", code, errout.String())
	}
	got := summarizeSANY(t, out.Bytes())
	for name, level := range map[string]int{"A": 1, "B": 2, "C": 3, "D": 1, "E": 1, "F": 1, "G": 1, "H": 1, "I": 0, "J": 0, "K": 1, "L": 2, "M": 2, "N": 2, "O": 3, "P": 3, "Q": 3, "R": 3, "S": 3, "T": 1, "U": 1, "V": 2, "W": 3, "X": 3, "Y": 2, "Z": 3} {
		if got.definitions[name] != level {
			t.Errorf("%s: got level %d, want %d", name, got.definitions[name], level)
		}
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareSANYMetadata(t, jar, path)
	}
}

func TestJavaSANYHigherOrderLevelMatrix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "M.tla")
	var source strings.Builder
	source.WriteString("---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nUse(Op(_),a) == Op(a)\n")
	for i, body := range []string{"TRUE", "x = x", "x' = x", "[] (x = x)"} {
		fmt.Fprintf(&source, "Op%d(n) == %s\nV%d == %s\n", i, body, i, body)
	}
	for i := range 4 {
		for j := range 4 {
			fmt.Fprintf(&source, "C%d%d == Use(Op%d,V%d)\n", i, j, i, j)
		}
	}
	source.WriteString("====\n")
	if err := os.WriteFile(path, []byte(source.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errout bytes.Buffer
	if code := run(context.Background(), []string{"-sany-metadata", path}, &out, &errout); code != 0 {
		t.Fatalf("metadata exit %d: %s", code, errout.String())
	}
	got := summarizeSANY(t, out.Bytes())
	for i := range 4 {
		for j := range 4 {
			name := fmt.Sprintf("C%d%d", i, j)
			if level := got.definitions[name]; level != max(i, j) {
				t.Errorf("%s: got level %d, want %d", name, level, max(i, j))
			}
		}
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareSANYMetadata(t, jar, path)
	}
}

func TestJavaSANYHigherOrderInstances(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"A.tla": "---- MODULE A ----\nCONSTANT F(_)\nValue(n) == F(n)\nUnused == TRUE\n====\n",
		"M.tla": "---- MODULE M ----\nVARIABLE x\nIgnore(n) == TRUE\nState(n) == x = n\nP(Op(_)) == INSTANCE A WITH F <- Op\n" +
			"First == P(Ignore)!Value(x)\nSecond == P(State)!Value(0)\nThird == P(State)!Unused\n" +
			"T == INSTANCE TLC\nApply(Op(_),a) == Op(a)\nFourth == Apply(T!PrintT,[] (x = x))\n====\n",
	}
	for name, source := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(dir, "M.tla")
	var out, errout bytes.Buffer
	if code := run(context.Background(), []string{"-sany-metadata", path}, &out, &errout); code != 0 {
		t.Fatalf("metadata exit %d: %s", code, errout.String())
	}
	got := summarizeSANY(t, out.Bytes())
	for name, level := range map[string]int{"First": 1, "Second": 1, "Third": 0, "Fourth": 3} {
		if got.definitions[name] != level {
			t.Errorf("%s: got level %d, want %d", name, got.definitions[name], level)
		}
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareSANYMetadata(t, jar, path)
	}
}

func TestJavaSANYHigherOrderLevelConstraints(t *testing.T) {
	for _, tc := range []struct{ name, expression string }{
		{"named operator", "Use(Inc, [] (x = x))"},
		{"forwarded operator", "Relay(Inc, [] (x = x))"},
		{"lambda operator", "Use(LAMBDA n: n + 1, [] (x = x))"},
		{"lambda local operator", "Use(LAMBDA n: LET AddOne(m) == m + 1 IN AddOne(n), [] (x = x))"},
		{"binary builtin", "Binary(+, [] (x = x), 1)"},
		{"enabled operator", "Use(Shield, [] (x = x))"},
		{"assert condition", "Binary(Assert, [] (x = x), 1)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "M.tla")
			source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\n" +
				"Use(Op(_),a) == Op(a)\nRelay(Op(_),a) == Use(Op,a)\nBinary(Op(_,_),a,b) == Op(a,b)\n" +
				"Inc(n) == n + 1\nShield(n) == ENABLED n\nBad == " + tc.expression + "\n====\n"
			if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				if err := os.WriteFile(filepath.Join(dir, "M.cfg"), nil, 0o600); err != nil {
					t.Fatal(err)
				}
				runJavaTLCForExit(t, jar, dir, "M", 150)
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-sany-metadata", path}, &out, &errout)
			if code == 0 || out.Len() != 0 {
				t.Fatalf("accepted invalid level: exit %d, stdout %s, stderr %s", code, out.String(), errout.String())
			}
		})
	}
}
