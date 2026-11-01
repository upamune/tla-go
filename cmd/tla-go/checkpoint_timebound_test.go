package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCheckpointCLITimedStopAndRecover(t *testing.T) {
	binary := buildTimedStopTestCLI(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == x' = x + 1 /\\ x < 5\nInv == x < 5", "INIT Init\nNEXT Next\nINVARIANT Inv\nCHECK_DEADLOCK FALSE\n")
	spec, root := filepath.Join(dir, "M.tla"), filepath.Join(dir, "states")
	if err := os.WriteFile(spec, []byte("---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = x + 1 /\\ x < 5\nInv == x < 5\n====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errout := runTimedStopTestCLI(ctx, binary, "1ns", "-tool", "-checkpoint", "1", "-metadir", root, spec)
	if code != 255 || !strings.Contains(errout, "search incomplete") {
		t.Fatalf("stop: %d\n%s\n%s", code, out, errout)
	}
	for _, marker := range []string{"STARTMSG 2195:", "STARTMSG 2196:", "STARTMSG 2186:"} {
		if !strings.Contains(out, marker) {
			t.Fatalf("missing %s: %s", marker, out)
		}
	}
	if strings.Contains(out, "STARTMSG 1000:") || strings.Contains(out, "No error has been found") {
		t.Fatalf("misleading result: %s", out)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("metadata: %v %v", entries, err)
	}
	saved := filepath.Join(root, entries[0].Name())
	if _, err := os.Stat(filepath.Join(saved, "tla-go.chkpt")); err != nil {
		t.Fatal(err)
	}
	code, out, errout = runTimedStopTestCLI(ctx, binary, "", "-tool", "-recover", saved, spec)
	if code != 12 {
		t.Fatalf("recovery: %d\n%s\n%s", code, out, errout)
	}
}

func TestCheckpointCLITimedStopHumanOutput(t *testing.T) {
	binary := buildTimedStopTestCLI(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dir := t.TempDir()
	spec := filepath.Join(dir, "M.tla")
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = x + 1\n====\n"
	if err := os.WriteFile(spec, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "states")
	code, out, errout := runTimedStopTestCLI(ctx, binary, "1ns", "-checkpoint", "1", "-metadir", root, spec)
	if code != 255 || !strings.Contains(errout, "search incomplete") || strings.Contains(out, "No error has been found") {
		t.Fatalf("incomplete: %d\n%s\n%s", code, out, errout)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("metadata: %v %v", entries, err)
	}
	if _, err := os.Stat(filepath.Join(root, entries[0].Name(), "tla-go.chkpt")); err != nil {
		t.Fatal(err)
	}
}

func TestCheckpointTimedStopProcess(t *testing.T) {
	binary := buildTimedStopTestCLI(t)
	dir := t.TempDir()
	spec, root := filepath.Join(dir, "M.tla"), filepath.Join(dir, "states")
	if err := os.WriteFile(spec, []byte("---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = x + 1 /\\ x < 10000\nInv == x < 100\n====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\nINVARIANT Inv\nCHECK_DEADLOCK FALSE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	code, out, errout := runTimedStopTestCLI(ctx, binary, "1ns", "-tool", "-checkpoint", "1", "-metadir", root, spec)
	if code != 255 || !strings.Contains(errout, "search incomplete") || !strings.Contains(out, "STARTMSG 2196:") {
		t.Fatalf("process: %d\n%s\n%s", code, out, errout)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("metadata: %v %v", entries, err)
	}
	saved := filepath.Join(root, entries[0].Name())
	if _, err := os.Stat(filepath.Join(saved, "tla-go.chkpt")); err != nil {
		t.Fatal(err)
	}
	code, out, errout = runTimedStopTestCLI(ctx, binary, "", "-tool", "-recover", saved, spec)
	if code != 12 || !strings.Contains(out, "Inv") {
		t.Fatalf("resume process: %d\n%s\n%s", code, out, errout)
	}
}

// The unmodified clock is also exercised in a bounded subprocess. The model
// has no terminal state, so the test does not depend on timer resolution.
func TestCheckpointRealClockTimedStopProcess(t *testing.T) {
	if os.Getenv("TLA_GO_TEST_REAL_CLOCK_CHILD") == "1" {
		os.Exit(run(context.Background(), []string{"-tool", "-checkpoint", "1", "-metadir", os.Getenv("TLA_GO_TEST_META"), os.Getenv("TLA_GO_TEST_SPEC")}, os.Stdout, os.Stderr))
	}
	dir := t.TempDir()
	spec, root := filepath.Join(dir, "M.tla"), filepath.Join(dir, "states")
	writeCheckpointFixture(t, spec, "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = x + 1\n====\n")
	writeCheckpointFixture(t, filepath.Join(dir, "M.cfg"), "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCheckpointRealClockTimedStopProcess$")
	cmd.Env = append(os.Environ(), "TLA_GO_TEST_REAL_CLOCK_CHILD=1", "TLA_GO_TEST_SPEC="+spec, "TLA_GO_TEST_META="+root, "TLA_GO_DEBUG_STOP_AFTER=1ms", "TLA_GO_DEBUG_CHECKPOINT_AFTER=")
	output, err := cmd.CombinedOutput()
	if exit, ok := err.(*exec.ExitError); !ok || ctx.Err() != nil || exit.ExitCode() != 255 || !bytes.Contains(output, []byte("search incomplete")) || !bytes.Contains(output, []byte("STARTMSG 2196:")) {
		t.Fatalf("real-clock process: %v\n%s", err, output)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("metadata: %v %v", entries, err)
	}
	if _, err := os.Stat(filepath.Join(root, entries[0].Name(), "tla-go.chkpt")); err != nil {
		t.Fatal(err)
	}
}

func TestCheckpointCLITimedStopRejectsUnsupported(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == UNCHANGED x", "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n")
	spec := filepath.Join(dir, "M.tla")
	for _, tc := range []struct {
		duration string
		args     []string
	}{
		{"bad", []string{"-checkpoint", "1"}},
		{"1ns", nil},
		{"1ns", []string{"-checkpoint", "0"}},
		{"1ns", []string{"-checkpoint", "1", "-workers", "2"}},
	} {
		t.Setenv("TLA_GO_DEBUG_STOP_AFTER", tc.duration)
		var out, errout bytes.Buffer
		args := append(append([]string{}, tc.args...), spec)
		if code := run(context.Background(), args, &out, &errout); code == 0 {
			t.Fatalf("accepted %q %v: %s %s", tc.duration, tc.args, &out, &errout)
		}
	}
}
