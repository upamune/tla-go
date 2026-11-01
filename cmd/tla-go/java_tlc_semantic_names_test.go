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

// These models are unchanged from the Java TLC v1.7.4 test suite.
func TestJavaTLCSemanticNameErrors(t *testing.T) {
	for _, tc := range []struct{ name, message string }{
		{"etest1", "operator Foo expects 1 arguments, got 2"},
		{"etest2", "operator Foo expects 2 arguments, got 1"},
		{"etest5", "undefined symbol M!Init"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, ext := range []string{".tla", ".cfg"} {
				name := tc.name + ext
				data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", name))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, tc.name+".tla")}, &out, &errout)
			if code != 150 || !strings.Contains(errout.String(), tc.message) {
				t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runJavaTLCForExit(t, jar, dir, tc.name, 150)
				if !strings.Contains(java, "Semantic errors:") {
					t.Fatalf("Java TLC:\n%s", java)
				}
			}
		})
	}
}

// These original models compare INSTANCE reference order with TLC 1.7.4.
func TestJavaTLCInstanceReferenceOrder(t *testing.T) {
	for _, separator := range []string{" ", "\n"} {
		t.Run(fmt.Sprintf("separator %q", separator), func(t *testing.T) {
			checkJavaTLCInstanceOrderError(t, "Bad == I!F(1)"+separator+"I == INSTANCE A", "F(x) == x")
		})
	}
}

// Original examples compared with Java TLC 1.7.4, not upstream fixtures.
func TestJavaTLCAnonymousInstanceReferenceOrder(t *testing.T) {
	for _, tc := range []struct{ name, root, auxiliary string }{
		{"operator", "Bad == F(1)\nINSTANCE A", "F(x) == x"},
		{"operator argument", "Use(Op(_)) == Op(1)\nBad == Use(F)\nINSTANCE A", "F(x) == x"},
		{"selector", "Bad == F!lab\nINSTANCE A", "F == lab :: 1"},
		{"implicit constant", "INSTANCE A\nCONSTANT C", "CONSTANT C\nF == C"},
		{"implicit variable", "INSTANCE A\nVARIABLE v", "VARIABLE v\nF == v"},
		{"anonymous theorem", "THEOREM I!F(1) = 1\nI == INSTANCE A", "F(x) == x"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checkJavaTLCInstanceOrderError(t, tc.root, tc.auxiliary)
		})
	}
}

func checkJavaTLCInstanceOrderError(t *testing.T, root, auxiliary string) {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"M.tla": "---- MODULE M ----\n" + root + "\n====\n",
		"A.tla": "---- MODULE A ----\n" + auxiliary + "\n====\n",
		"M.cfg": "",
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout)
	if code != 150 || errout.Len() != 0 || !hasToolMessage(readToolMessages(out.String()), 3002) {
		t.Fatalf("Go exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
	}
	if jar := pinnedFairnessEffectJar(t); jar != "" {
		java, javaCode := runJavaTLCArgs(t, jar, dir, []string{"-tool", "-workers", "1", "-fp", "0", "M.tla"})
		if javaCode != 150 || !hasToolMessage(readToolMessages(java), 3002) {
			t.Fatalf("Java TLC exit %d:\n%s", javaCode, java)
		}
	}
}

func TestJavaTLCAnonymousInstanceErrorBeforeEvaluation(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"M.tla": "---- MODULE M ----\nEXTENDS TLC\nVARIABLE v\nInit == PrintT(\"side_effect_marker\") /\\ v = 0\nNext == UNCHANGED v\nBad == F\nINSTANCE A\n====\n",
		"A.tla": "---- MODULE A ----\nF == 1\n====\n",
		"M.cfg": "INIT Init\nNEXT Next\n",
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout)
	if code != 150 || errout.Len() != 0 || strings.Contains(out.String(), "side_effect_marker") || !hasToolMessage(readToolMessages(out.String()), 3002) {
		t.Fatalf("Go exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
	}
	if jar := pinnedFairnessEffectJar(t); jar != "" {
		java, javaCode := runJavaTLCArgs(t, jar, dir, []string{"-tool", "-workers", "1", "-fp", "0", "M.tla"})
		if javaCode != 150 || strings.Contains(java, "side_effect_marker") || !hasToolMessage(readToolMessages(java), 3002) {
			t.Fatalf("Java TLC exit %d:\n%s", javaCode, java)
		}
	}
}

// Java accepts this shadow with a warning; tla-go still reports a duplicate.
func TestJavaTLCInstanceShadowKnownGap(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"M.tla": "---- MODULE M ----\nEXTENDS Naturals\nI == INSTANCE A\nGood == LET X == I!F(1) I == INSTANCE A IN X\nVARIABLE v\nInit == v = 0\nNext == UNCHANGED v\n====\n",
		"A.tla": "---- MODULE A ----\nF(x) == x\n====\n",
		"M.cfg": "INIT Init\nNEXT Next\n",
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout)
	if code != 255 || !strings.Contains(out.String(), "Multiply-defined symbol 'I!F'") {
		t.Fatalf("Go exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
	}
	if jar := pinnedFairnessEffectJar(t); jar != "" {
		java, javaCode := runJavaTLCArgs(t, jar, dir, []string{"-tool", "-workers", "1", "-fp", "0", "M.tla"})
		if javaCode != 0 || !strings.Contains(java, "Multiple declarations or definitions for symbol I") {
			t.Fatalf("Java TLC exit %d:\n%s", javaCode, java)
		}
	}
}
