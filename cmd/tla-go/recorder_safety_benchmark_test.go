package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// Measures ordinary model-checking error reporting with diagnostics disabled.
func BenchmarkRecorderDisabledSafetyViolation(b *testing.B) {
	b.Setenv("TLA_GO_DEBUG_RECORDER", "")
	dir := b.TempDir()
	model := filepath.Join(dir, "M.tla")
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x < 3 /\\ x' = x + 1\nInv == x < 2\n====\n"
	config := "INIT Init\nNEXT Next\nINVARIANT Inv\nCHECK_DEADLOCK FALSE\n"
	if err := os.WriteFile(model, []byte(source), 0o600); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte(config), 0o600); err != nil {
		b.Fatal(err)
	}
	args := []string{"-tool", "-workers", "1", "-checkpoint", "0", model}
	b.ReportAllocs()
	for b.Loop() {
		if code := run(context.Background(), args, io.Discard, io.Discard); code != 12 {
			b.Fatalf("exit %d", code)
		}
	}
}
