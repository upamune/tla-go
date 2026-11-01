package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// buildTimedStopTestCLI builds the actual CLI with a test-only clock overlay.
// Its first two readings are equal (schedule and stop deadline), and all
// subsequent readings advance by one second. Thus a 1ns deadline expires at
// the first expansion boundary, regardless of the OS clock resolution.
func buildTimedStopTestCLI(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path, err := filepath.Abs(filepath.Join("..", "..", "checker", "checkpoint_api.go"))
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const original = "return checkWithCheckpointClock(ctx, source, configuration, options, time.Now)"
	const replacement = `calls := 0
	now := func() time.Time {
		calls++
		if calls > 2 { return time.Unix(1, 0) }
		return time.Unix(0, 0)
	}
	return checkWithCheckpointClock(ctx, source, configuration, options, now)`
	if strings.Count(string(source), original) != 1 {
		t.Fatal("checkpoint clock entry point not found uniquely")
	}
	modified := filepath.Join(dir, "checkpoint_api.go")
	if err := os.WriteFile(modified, []byte(strings.Replace(string(source), original, replacement, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	overlay, err := json.Marshal(map[string]any{"Replace": map[string]string{path: modified}})
	if err != nil {
		t.Fatal(err)
	}
	overlayPath := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(overlayPath, overlay, 0o600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(dir, "tla-go.exe")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-overlay", overlayPath, "-o", binary, ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build clock-overlay CLI: %v\n%s", err, output)
	}
	return binary
}

func runTimedStopTestCLI(ctx context.Context, binary, duration string, args ...string) (int, string, string) {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = append(os.Environ(), "TLA_GO_DEBUG_STOP_AFTER="+duration, "TLA_GO_DEBUG_CHECKPOINT_AFTER=")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if err == nil {
		return 0, stdout.String(), stderr.String()
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && ctx.Err() == nil {
		return exit.ExitCode(), stdout.String(), stderr.String()
	}
	return -1, stdout.String(), err.Error() + "\n" + stderr.String()
}
