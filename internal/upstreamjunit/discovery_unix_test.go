//go:build darwin || linux || freebsd || openbsd || netbsd || dragonfly

package upstreamjunit

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The shell and child share a process group that the test owns, even if the
// child never reaches user code. This also cleans up after a setup failure.
func TestDiscoveryCommandInheritedOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := discoveryCommand(ctx, "sh", "-c", "sleep 30 & printf child-ready")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err != nil && !errors.Is(err, syscall.ESRCH) {
			t.Errorf("kill fixture process group: %v", err)
		}
	})
	if err := cmd.Wait(); !errors.Is(err, exec.ErrWaitDelay) {
		t.Fatalf("expected ErrWaitDelay for inherited output, got %v", err)
	}
	if !strings.Contains(output.String(), "child-ready") {
		t.Fatalf("missing fixture output: %q", output.String())
	}
}
