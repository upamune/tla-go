package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJavaTLCUpstreamLargeSubsetState(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"InitStateBug.tla", "InitStateBug.cfg"} {
		data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", "bug279", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", filepath.Join(dir, "InitStateBug.tla")}, &out, &errout)
	counts := "3 states generated, 3 distinct states found, 0 states left on queue"
	if code != 11 || !strings.Contains(out.String(), counts) || !strings.Contains(out.String(), "set = SUBSET 1..20") {
		t.Fatalf("Go exit %d: %s %s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java := runJavaTLCForExit(t, jar, dir, "InitStateBug", 11)
		if !strings.Contains(java, counts) || !strings.Contains(java, "set = SUBSET 1..20") {
			t.Fatalf("Java output: %s", java)
		}
	}
}
