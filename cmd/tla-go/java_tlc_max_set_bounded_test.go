package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"
)

// This helper is invoked as a separate test process by runBoundedMaxSetCLI.
// It isolates models that could enumerate forever or produce unbounded output.
func TestMaxSetBoundedCLIProcess(t *testing.T) {
	if os.Getenv("TLA_GO_MAXSET_CLI_CHILD") != "1" {
		return
	}
	os.Exit(run(context.Background(), maxSetOracleArgs(), os.Stdout, os.Stderr))
}

func runBoundedMaxSetCLI(t *testing.T, dir string) (string, int, string) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	out := &boundedMaxSetOutput{limit: 1 << 20, cancel: cancel}
	errout := &boundedMaxSetOutput{limit: 1 << 20, cancel: cancel}
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestMaxSetBoundedCLIProcess$")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "TLA_GO_MAXSET_CLI_CHILD=1", "GOMEMLIMIT=256MiB")
	cmd.Stdout, cmd.Stderr = out, errout
	err = cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("Go child timed out or exceeded output limit: %v", ctx.Err())
	}
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		return out.String(), exit.ExitCode(), errout.String()
	}
	if err != nil {
		t.Fatalf("run Go child: %v", err)
	}
	return out.String(), 0, errout.String()
}

type boundedMaxSetOutput struct {
	mu     sync.Mutex
	data   bytes.Buffer
	limit  int
	cancel context.CancelFunc
}

func (w *boundedMaxSetOutput) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	remaining := w.limit - w.data.Len()
	if len(p) > remaining {
		_, _ = w.data.Write(p[:remaining])
		w.cancel()
		return len(p), nil
	}
	return w.data.Write(p)
}

func (w *boundedMaxSetOutput) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.data.String()
}

func TestBoundedMaxSetOutputCancelsAtLimit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := &boundedMaxSetOutput{limit: 3, cancel: cancel}
	if n, err := out.Write([]byte("ab")); n != 2 || err != nil || ctx.Err() != nil {
		t.Fatalf("first write = %d, %v; context = %v", n, err, ctx.Err())
	}
	if n, err := out.Write([]byte("cdef")); n != 4 || err != nil || ctx.Err() != context.Canceled || out.String() != "abc" {
		t.Fatalf("second write = %d, %v; context = %v, output = %q", n, err, ctx.Err(), out.String())
	}
}
