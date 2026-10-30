package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJavaTLCUpstreamASStateOnlySpecification(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"AS.tla", "AS.cfg"} {
		data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", "as", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-tool", filepath.Join(dir, "AS.tla")}, &out, &errout)
	want := "No next state actions defined to generate successor states from."
	if code != 75 || !strings.Contains(out.String(), want) {
		t.Fatalf("Go exit %d: %s %s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java := runJavaTLCForExit(t, jar, dir, "AS", 75)
		if !strings.Contains(java, want) {
			t.Fatalf("Java output: %s", java)
		}
	}
}
