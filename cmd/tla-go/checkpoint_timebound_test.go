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
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == x' = x + 1 /\\ x < 5\nInv == x < 5", "INIT Init\nNEXT Next\nINVARIANT Inv\nCHECK_DEADLOCK FALSE\n")
	spec, root := filepath.Join(dir, "M.tla"), filepath.Join(dir, "states")
	if err := os.WriteFile(spec, []byte("---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = x + 1 /\\ x < 5\nInv == x < 5\n====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TLA_GO_DEBUG_STOP_AFTER", "1ns")
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-tool", "-checkpoint", "1", "-metadir", root, spec}, &out, &errout)
	if code != 255 || !strings.Contains(errout.String(), "search incomplete") {
		t.Fatalf("stop: %d\n%s\n%s", code, &out, &errout)
	}
	for _, marker := range []string{"STARTMSG 2195:", "STARTMSG 2196:", "STARTMSG 2186:"} {
		if !strings.Contains(out.String(), marker) {
			t.Fatalf("missing %s: %s", marker, &out)
		}
	}
	if strings.Contains(out.String(), "STARTMSG 1000:") || strings.Contains(out.String(), "No error has been found") {
		t.Fatalf("misleading result: %s", &out)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("metadata: %v %v", entries, err)
	}
	saved := filepath.Join(root, entries[0].Name())
	if _, err := os.Stat(filepath.Join(saved, "tla-go.chkpt")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TLA_GO_DEBUG_STOP_AFTER", "")
	out.Reset()
	errout.Reset()
	code = run(context.Background(), []string{"-tool", "-recover", saved, spec}, &out, &errout)
	if code != 12 {
		t.Fatalf("recovery: %d\n%s\n%s", code, &out, &errout)
	}
}

func TestCheckpointCLITimedStopHumanOutput(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "M.tla")
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = x + 1\n====\n"
	if err := os.WriteFile(spec, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TLA_GO_DEBUG_STOP_AFTER", "1ns")
	var out, errout bytes.Buffer
	root := filepath.Join(dir, "states")
	code := run(context.Background(), []string{"-checkpoint", "1", "-metadir", root, spec}, &out, &errout)
	if code != 255 || !strings.Contains(errout.String(), "search incomplete") || strings.Contains(out.String(), "No error has been found") {
		t.Fatalf("incomplete: %d\n%s\n%s", code, &out, &errout)
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
	if os.Getenv("TLA_GO_TEST_TIMEBOUND_CHILD") == "1" {
		args := []string{"-tool", "-checkpoint", "1", "-metadir", os.Getenv("TLA_GO_TEST_META"), os.Getenv("TLA_GO_TEST_SPEC")}
		if recovery := os.Getenv("TLA_GO_TEST_RECOVER"); recovery != "" {
			args = []string{"-tool", "-recover", recovery, os.Getenv("TLA_GO_TEST_SPEC")}
		}
		code := run(context.Background(), args, os.Stdout, os.Stderr)
		os.Exit(code)
	}
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
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCheckpointTimedStopProcess$")
	cmd.Env = append(os.Environ(), "TLA_GO_TEST_TIMEBOUND_CHILD=1", "TLA_GO_TEST_SPEC="+spec, "TLA_GO_TEST_META="+root, "TLA_GO_DEBUG_STOP_AFTER=1ns")
	output, err := cmd.CombinedOutput()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 255 || !bytes.Contains(output, []byte("search incomplete")) || !bytes.Contains(output, []byte("STARTMSG 2196:")) {
		t.Fatalf("process: %v\n%s", err, output)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("metadata: %v %v", entries, err)
	}
	saved := filepath.Join(root, entries[0].Name())
	if _, err := os.Stat(filepath.Join(saved, "tla-go.chkpt")); err != nil {
		t.Fatal(err)
	}
	resume := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCheckpointTimedStopProcess$")
	resume.Env = append(os.Environ(), "TLA_GO_TEST_TIMEBOUND_CHILD=1", "TLA_GO_TEST_SPEC="+spec, "TLA_GO_TEST_RECOVER="+saved, "TLA_GO_DEBUG_STOP_AFTER=")
	output, err = resume.CombinedOutput()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 12 || !bytes.Contains(output, []byte("Inv")) {
		t.Fatalf("resume process: %v\n%s", err, output)
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
