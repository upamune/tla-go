package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/upamune/tla-go/checker"
)

func BenchmarkRecorderDisabledToolStatistics(b *testing.B) {
	result := checker.Result{GeneratedStates: 1000, DistinctStates: 400, QueuedStates: 0, Depth: 12}
	output := toolOutput{out: io.Discard, started: time.Now()}
	b.ReportAllocs()
	for b.Loop() {
		printToolStatistics(output, result)
		output.finish()
	}
}

func BenchmarkRecorderDisabledCLI(b *testing.B) {
	b.Setenv("TLA_GO_DEBUG_RECORDER", "")
	dir := b.TempDir()
	model := filepath.Join(dir, "M.tla")
	if err := os.WriteFile(model, []byte("---- MODULE M ----\n====\n"), 0o600); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), nil, 0o600); err != nil {
		b.Fatal(err)
	}
	args := []string{"-tool", "-workers", "1", model}
	b.ReportAllocs()
	for b.Loop() {
		if code := run(context.Background(), args, io.Discard, io.Discard); code != 0 {
			b.Fatalf("exit %d", code)
		}
	}
}

func BenchmarkRecorderDisabledHumanCLI(b *testing.B) {
	b.Setenv("TLA_GO_DEBUG_RECORDER", "")
	dir := b.TempDir()
	model := filepath.Join(dir, "M.tla")
	if err := os.WriteFile(model, []byte("---- MODULE M ----\n====\n"), 0o600); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), nil, 0o600); err != nil {
		b.Fatal(err)
	}
	args := []string{"-workers", "1", model}
	b.ReportAllocs()
	for b.Loop() {
		if code := run(context.Background(), args, io.Discard, io.Discard); code != 0 {
			b.Fatalf("exit %d", code)
		}
	}
}
