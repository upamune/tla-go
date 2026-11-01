package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A timed checkpoint must survive a process restart for named specifications,
// including operator arguments in the next-state action.
func TestCheckpointNamedTimedProcess(t *testing.T) {
	binary := buildTimedStopTestCLI(t)
	for _, tc := range []struct {
		name, invariant string
		code            int
	}{
		{"success", "x <= 3", 0},
		{"violation", "x < 3", 12},
	} {
		t.Run(tc.name, func(t *testing.T) { checkNamedTimedProcess(t, binary, tc.invariant, tc.code) })
	}
}

func checkNamedTimedProcess(t *testing.T, binary, invariant string, wantCode int) {
	t.Helper()
	dir := t.TempDir()
	spec, config, root := filepath.Join(dir, "M.tla"), filepath.Join(dir, "M.cfg"), filepath.Join(dir, "states")
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLES x, y\nStep(n) == n + 1\nInit == x = 0 /\\ y \\in 1..3\nNext == x < 3 /\\ x' = Step(x) /\\ UNCHANGED y\nSpec == Init /\\ [][Next]_<<x,y>>\nInv == " + invariant + "\n====\n"
	cfg := "SPECIFICATION Spec\nINVARIANT Inv\nCHECK_DEADLOCK FALSE\n"
	writeCheckpointFixture(t, spec, source)
	writeCheckpointFixture(t, config, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	code, output := namedCheckpointProcess(ctx, binary, "1ns", "-tool", "-checkpoint", "1", "-metadir", root, spec)
	if code != 255 || !strings.Contains(output, "search incomplete") {
		t.Fatalf("pause: %d\n%s", code, output)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("metadata: %v %v", entries, err)
	}
	saved := filepath.Join(root, entries[0].Name())
	checkpoint := filepath.Join(saved, "tla-go.chkpt")
	before, err := os.ReadFile(checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	writeCheckpointFixture(t, config, cfg+"\n\\* changed configuration\n")
	code, output = namedCheckpointProcess(ctx, binary, "", "-tool", "-recover", saved, spec)
	after, readErr := os.ReadFile(checkpoint)
	if code != 255 || readErr != nil || !bytes.Equal(before, after) {
		t.Fatalf("modified configuration: code %d, read %v\n%s", code, readErr, output)
	}
	writeCheckpointFixture(t, config, cfg)
	resumedCode, resumed := namedCheckpointProcess(ctx, binary, "", "-tool", "-recover", saved, spec)
	fullCode, full := namedCheckpointProcess(ctx, binary, "", "-tool", spec)
	if resumedCode != wantCode || fullCode != wantCode {
		t.Fatalf("exit: recovered %d, normal %d; want %d\n%s\n%s", resumedCode, fullCode, wantCode, resumed, full)
	}
	ids := []int{2199}
	if wantCode == 12 {
		ids = append(ids, 2110)
	}
	for _, id := range ids {
		if got, want := rawToolMessage(resumed, id), rawToolMessage(full, id); got == "" || got != want {
			t.Errorf("message %d: recovered %q, normal %q", id, got, want)
		}
	}
}

func writeCheckpointFixture(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func namedCheckpointProcess(ctx context.Context, binary, stop string, args ...string) (int, string) {
	code, stdout, stderr := runTimedStopTestCLI(ctx, binary, stop, args...)
	return code, stdout + stderr
}
