package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJavaTLCUpstreamSetComparisonFailure(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"MC.tla", "MC.cfg", "PrintTraceRace.tla"} {
		data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", "print-trace-race", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", filepath.Join(dir, "MC.tla")}, &out, &errout)
	want := "Attempted to check equality of the set {1, 2} with the value:\n1"
	if code != 76 || !strings.Contains(out.String(), want) || !strings.Contains(out.String(), "2 states generated, 2 distinct states found") {
		t.Fatalf("Go exit %d: %s %s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java := runJavaTLCForExit(t, jar, dir, "MC", 76)
		if !strings.Contains(java, want) {
			t.Fatalf("Java output: %s", java)
		}
	}
}
