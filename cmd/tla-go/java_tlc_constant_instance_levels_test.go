package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestJavaTLCConstantModuleInstanceLevels(t *testing.T) {
	for _, tc := range []struct {
		name, body, substitution string
		code                     int
	}{
		{"plain constant", "F == C", "TRUE", 0},
		{"plain state", "F == C", "x", 0},
		{"plain action", "F == C", "x'", 0},
		{"plain temporal", "F == C", "[](x = 0)", 0},
		{"assumption state", "ASSUME C = {}", "x", 0},
		{"assumption action", "ASSUME C = {}", "x'", 0},
		{"assumption temporal", "ASSUME C = {}", "[](x = 0)", 150},
		{"theorem assumption action", "THEOREM ASSUME C PROVE TRUE", "x'", 0},
		{"theorem assumption temporal", "THEOREM ASSUME C PROVE TRUE", "[](x = 0)", 150},
		{"theorem goal temporal", "THEOREM ASSUME TRUE PROVE C", "[](x = 0)", 150},
		{"temporal comparison", "F == C = {}", "[](x = 0)", 150},
		{"unconstrained theorem", "THEOREM C", "[](x = 0)", 0},
		{"unused argument", "F(p) == TRUE\nG == F(C)", "[](x = 0)", 0},
		{"used argument", "F(p) == p = {}\nG == F(C)", "[](x = 0)", 150},
		{"logical temporal", "F == C /\\ TRUE", "[](x = 0)", 0},
		{"negated temporal", "F == ~C", "[](x = 0)", 0},
		{"tuple temporal", "F == <<C>>", "[](x = 0)", 150},
		{"set temporal", "F == {C}", "[](x = 0)", 150},
		{"record value temporal", "F == [a |-> C]", "[](x = 0)", 0},
		{"record set temporal", "F == [a : C]", "[](x = 0)", 150},
		{"quantifier body temporal", "F == \\A i \\in {1} : C", "[](x = 0)", 0},
		{"quantifier bound temporal", "F == \\A i \\in C : TRUE", "[](x = 0)", 0},
		{"choose bound temporal", "F == CHOOSE i \\in C : TRUE", "[](x = 0)", 0},
		{"choose body temporal", "F == CHOOSE i \\in {1} : C", "[](x = 0)", 150},
		{"set bound temporal", "F == {i : i \\in C}", "[](x = 0)", 0},
		{"set body temporal", "F == {C : i \\in {1}}", "[](x = 0)", 150},
		{"function bound temporal", "F == [i \\in C |-> TRUE]", "[](x = 0)", 0},
		{"nested temporal bound", "F == <<{i : i \\in C}>>", "[](x = 0)", 150},
		{"function value temporal", "F == [i \\in {1} |-> C]", "[](x = 0)", 0},
		{"nonconstant module state", "F == ENABLED TRUE", "x", 150},
		{"nonconstant module action", "F == TRUE'", "x'", 150},
		{"nonconstant temporal quantifier", "F == \\EE y : TRUE", "x", 150},
		{"nonconstant temporal theorem", "THEOREM [](TRUE)", "x", 150},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			files := map[string]string{
				"A.tla": "---- MODULE A ----\nCONSTANT C\n" + tc.body + "\n====\n",
				"M.tla": "---- MODULE M ----\nVARIABLE x\nI == INSTANCE A WITH C <- " + tc.substitution + "\nInit == x = 0\nNext == UNCHANGED x\n====\n",
				"M.cfg": "INIT Init\nNEXT Next\n",
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

// test213 and test213b are upstream TLC v1.7.4 suite fixtures.
func TestJavaTLCUpstreamInstanceTemporalLevels(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"test213.tla", "test213b.tla", "test213.cfg"} {
		data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-tool", filepath.Join(dir, "test213.tla")}, &out, &errout)
	if code != 150 || errout.Len() != 0 || !hasToolMessage(readToolMessages(out.String()), 3002) {
		t.Fatalf("exit %d: %s %s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		runJavaTLCForExit(t, jar, dir, "test213", 150)
	}
}

func TestJavaTLCParameterizedConstantModuleLevels(t *testing.T) {
	for _, tc := range []struct {
		name, body, expression string
		code                   int
	}{
		{"unrestricted state", "F == C", "P(x)!F", 0},
		{"unrestricted action", "F == C", "P(x')!F", 0},
		{"unrestricted temporal", "F == C", "P([](x = 0))!F", 0},
		{"comparison state", "F == C = 0\nU == TRUE", "P(x)!F", 0},
		{"comparison action", "F == C = 0\nU == TRUE", "P(x')!F", 0},
		{"comparison temporal", "F == C = 0\nU == TRUE", "P([](x = 0))!F", 150},
		{"unused temporal", "F == C = 0\nU == TRUE", "P([](x = 0))!U", 0},
		{"unconstrained alias temporal", "F == C = 0\nU == C", "P([](x = 0))!U", 0},
		{"assumption does not constrain alias", "ASSUME C = {}\nF == C", "P([](x = 0))!F", 0},
		{"local wrapper state", "F == C = 0", "LET H(q) == P(q)!F IN H(x)", 0},
		{"local wrapper temporal", "F == C = 0", "LET H(q) == P(q)!F IN H([](x = 0))", 150},
		{"local wrapper action", "F == C = 0", "LET H(q) == P(q)!F IN H(x')", 0},
		{"local chain temporal", "F == C = 0", "LET H(q) == P(q)!F J(r) == H(r) IN J([](x = 0))", 150},
		{"local unrelated temporal", "F == C = 0", "LET H(q) == TRUE IN H([](x = 0))", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			files := map[string]string{
				"A.tla": "---- MODULE A ----\nCONSTANT C\n" + tc.body + "\n====\n",
				"M.tla": "---- MODULE M ----\nVARIABLE x\nP(p) == INSTANCE A WITH C <- p\nG == " + tc.expression + "\nInit == x = 0\nNext == UNCHANGED x\n====\n",
				"M.cfg": "INIT Init\nNEXT Next\n",
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

func TestJavaTLCLocalParameterizedStateInstance(t *testing.T) {
	for _, tc := range []struct {
		name, body, argument string
		code                 int
	}{
		{"state", "F == y", "x", 0},
		{"unconstrained action", "F == y", "x'", 0},
		{"unconstrained temporal", "F == y", "[](x = 0)", 0},
		{"local alias temporal", "F == LET K == y IN K", "[](x = 0)", 0},
		{"comparison state", "F == y = 0", "x", 0},
		{"comparison action", "F == y = 0", "x'", 150},
		{"comparison temporal", "F == y = 0", "[](x = 0)", 150},
		{"negation action", "F == ~y", "x'", 150},
		{"quantifier bound action", "F == \\A i \\in y : TRUE", "x'", 0},
		{"unused operator argument", "Ignore(q) == TRUE\nF == Ignore(y)", "x'", 150},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			files := map[string]string{
				"A.tla": "---- MODULE A ----\nVARIABLE y\n" + tc.body + "\n====\n",
				"M.tla": "---- MODULE M ----\nVARIABLE x\nP(p) == INSTANCE A WITH y <- p\nG == LET H(q) == P(q)!F IN H(" + tc.argument + ")\nInit == x = 0\nNext == UNCHANGED x\n====\n",
				"M.cfg": "INIT Init\nNEXT Next\n",
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

func TestJavaTLCParameterizedNonconstantModuleConstants(t *testing.T) {
	for _, tc := range []struct {
		name, body, expression string
		code                   int
	}{
		{"plain state", "F == C\nU == TRUE", "P(x)!F", 0},
		{"plain action", "F == C\nU == TRUE", "P(x')!F", 0},
		{"plain temporal", "F == C\nU == TRUE", "P([](x = 0))!F", 0},
		{"unused temporal", "F == C\nU == TRUE", "P([](x = 0))!U", 0},
		{"comparison state", "F == C = y\nU == C", "P(x)!F", 150},
		{"comparison action", "F == C = y\nU == C", "P(x')!F", 150},
		{"comparison temporal", "F == C = y\nU == C", "P([](x = 0))!F", 150},
		{"separate alias state", "F == C = y\nU == C", "P(x)!U", 0},
		{"separate alias temporal", "F == C = y\nU == C", "P([](x = 0))!U", 0},
		{"local wrapper constant", "F == C = y", "LET H(q) == P(q)!F IN H(0)", 0},
		{"local wrapper state", "F == C = y", "LET H(q) == P(q)!F IN H(x)", 150},
		{"local chain state", "F == C = y", "LET H(q) == P(q)!F J(r) == H(r) IN J(x)", 150},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			files := map[string]string{
				"A.tla": "---- MODULE A ----\nCONSTANT C\nVARIABLE y\n" + tc.body + "\n====\n",
				"M.tla": "---- MODULE M ----\nVARIABLE x\nP(p) == INSTANCE A WITH C <- p, y <- x\nG == " + tc.expression + "\nInit == x = 0\nNext == UNCHANGED x\n====\n",
				"M.cfg": "INIT Init\nNEXT Next\n",
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

func TestJavaTLCInstanceConstantAliasInFunctionDomain(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"A.tla": "---- MODULE A ----\nCONSTANT C\nVARIABLE y\nPublic == C\n====\n",
		"M.tla": "---- MODULE M ----\nVARIABLE x\nP(p) == INSTANCE A WITH C <- p, y <- x\nf[i \\in {P(x)!Public}] == i\nInit == x = 0\nNext == UNCHANGED x\n====\n",
		"M.cfg": "INIT Init\nNEXT Next\n",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout)
	if code != 0 {
		t.Fatalf("Go exit %d: %s %s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		runJavaTLCForExit(t, jar, dir, "M", 0)
	}
}
