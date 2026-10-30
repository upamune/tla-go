package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJavaTLCUpstreamFunctionSetFailure(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"MC.tla", "MC.cfg", "TSnapShot.tla"} {
		data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", "tsnapshot", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final", filepath.Join(dir, "MC.tla")}, &out, &errout)
	want := "Attempted to check if \n0\nwhich is not a TLC function value, is in the set of functions:\n{(w1 :> 0), (w1 :> 1), (w1 :> 2)}"
	counts := "14 states generated, 10 distinct states found, 3 states left on queue"
	if code != 76 || !strings.Contains(out.String(), want) || !strings.Contains(out.String(), counts) {
		t.Fatalf("Go exit %d: %s %s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java := runJavaTLCForExit(t, jar, dir, "MC", 76)
		if !strings.Contains(java, want) || !strings.Contains(java, counts) {
			t.Fatalf("Java output: %s", java)
		}
	}
}

func TestJavaTLCFunctionSetModelValue(t *testing.T) {
	for _, tc := range []struct {
		name, constant, message string
		code                    int
	}{
		{"untyped", "item", "Invariant Inv is violated", 12},
		{"typed", "A_item", "Attempted to check if the typed model value A_item is an element of", 76},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "CONSTANT X\nVARIABLE x\nInit == x=0\nNext == x'=1\nInv == x=0 \\/ X \\in [{1} -> {0,1}]"
			writeDOTFixture(t, dir, body, "CONSTANT X = "+tc.constant+" INIT Init NEXT Next INVARIANT Inv")
			var out, errout bytes.Buffer
			if code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout); code != tc.code || !strings.Contains(out.String(), tc.message) {
				t.Fatalf("Go exit %d: %s %s", code, out.String(), errout.String())
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
