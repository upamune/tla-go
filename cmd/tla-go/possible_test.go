package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIReportsUnwitnessedPossible(t *testing.T) {
	path := filepath.Join("..", "..", "checker", "testdata", "possible", "PossibleFail.tla")
	cfg := filepath.Join("..", "..", "checker", "testdata", "possible", "PossibleFailMixedTest.cfg")
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-config", cfg, "-deadlock", path}, &out, &errout)
	want := "[AtOne |-> 1, AllDone |-> 1, Unreachable |-> 0, WrapAround |-> 1, BigJump |-> 0]\nError: The _POSSIBLE predicate Unreachable at line 12, col 16 to line 12, col 20 of module PossibleFail was never witnessed."
	if code != 10 || !strings.Contains(out.String(), want) || errout.Len() != 0 {
		t.Fatalf("code %d stdout %q stderr %q", code, out.String(), errout.String())
	}
}

func TestCLIReportsUnwitnessedAlongsideInvariant(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "M.tla")
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 3\nUnreachable == x = 5\nInv == x < 2\n===="
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init NEXT Next _POSSIBLE Unreachable INVARIANT Inv"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{path}, &out, &errout)
	if code != 12 || !strings.Contains(out.String(), "Error: Invariant Inv is violated.") || !strings.Contains(out.String(), "[Unreachable |-> 0]\nError: The _POSSIBLE predicate Unreachable") || errout.Len() != 0 {
		t.Fatalf("code %d stdout %q stderr %q", code, out.String(), errout.String())
	}
}
