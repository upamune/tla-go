package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJavaTLCInvariantLevel(t *testing.T) {
	for _, tc := range []struct {
		name, body, cfg string
		code, message   int
	}{
		{"action", "Inv == x' = x", "INIT Init\nNEXT Next\nINVARIANT Inv\n", 76, 2146},
		{"action without level parameter", "Inv == x' = 0", "INIT Init\nNEXT Next\nINVARIANT Inv\n", 76, 2146},
		{"temporal with level parameter", "Inv == [](x = 0) /\\ x = 0", "INIT Init\nNEXT Next\nINVARIANT Inv\n", 76, 2146},
		{"operator ignores argument", "F(y) == TRUE\nInv == (x' = 0) /\\ F(x)", "INIT Init\nNEXT Next\nINVARIANT Inv\n", 76, 2146},
		{"operator uses argument", "F(y) == y = 0\nInv == (x' = 0) /\\ F(x)", "INIT Init\nNEXT Next\nINVARIANT Inv\n", 76, 2146},
		{"temporal", "Inv == [](x = 0)", "INIT Init\nNEXT Next\nINVARIANT Inv\n", 76, 2146},
		{"action alias", "A == x' = x\nInv == A", "INIT Init\nNEXT Next\nINVARIANT Inv\n", 76, 2146},
		{"before missing property", "Inv == x' = x", "INIT Init\nNEXT Next\nPROPERTY Missing\nINVARIANT Inv\n", 76, 2146},
		{"before missing init", "Inv == x' = x", "INIT Missing\nNEXT Next\nINVARIANT Inv\n", 76, 2146},
		{"missing invariant", "", "INIT Init\nNEXT Next\nINVARIANT Missing\n", 151, 2229},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == UNCHANGED x\n"+tc.body, tc.cfg)
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", filepath.Join(dir, "M.tla")}, &out, &errout)
			messages := readToolMessages(out.String())
			if code != tc.code || errout.Len() != 0 || !hasToolMessage(messages, tc.message) || hasToolMessage(messages, 2199) {
				t.Fatalf("exit %d, want %d; stdout: %s; stderr: %s", code, tc.code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaToolMessages(t, jar, dir, code, messages)
			}
		})
	}
}

// TestInvalidInvariant is an upstream Java TLC test-model fixture from v1.7.4.
func TestJavaTLCUpstreamInvalidInvariant(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"testinvalidinvariant.tla", "testinvalidinvariant.cfg"} {
		data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-tool", filepath.Join(dir, "testinvalidinvariant.tla")}, &out, &errout)
	if code != 76 || errout.Len() != 0 || !hasToolMessage(readToolMessages(out.String()), 2146) {
		t.Fatalf("exit %d: %s %s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java := runJavaTLCForExit(t, jar, dir, "testinvalidinvariant", 76)
		if !strings.Contains(java, "The invariant Invariant is not a state predicate") {
			t.Fatalf("Java output: %s", java)
		}
	}
}
