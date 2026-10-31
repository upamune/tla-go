package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

const workerModel = "VARIABLES x, y\nInit == x = 0 /\\ y = 0\nNext == x' \\in 0..3 /\\ y' = (y + x) % 4\nInv == x \\in 0..3"

// Java TLC accepts any positive worker count and checkpoint interval. tla-go
// accepts them but explores states sequentially and writes no checkpoints.
func TestJavaTLCWorkerAndCheckpointOptions(t *testing.T) {
	for _, tc := range []struct {
		args    []string
		workers int
	}{
		{[]string{"-workers", "3"}, 3},
		{[]string{"-workers", "auto"}, runtime.NumCPU()},
		{[]string{"-workers", "1", "-checkpoint", "0"}, 1},
		{[]string{"-checkpoint", "5", "-workers", "+2"}, 2},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, workerModel, "INIT Init\nNEXT Next\nINVARIANT Inv\n")
			var out, errout bytes.Buffer
			code := run(context.Background(), append(append([]string{"-tool"}, tc.args...), filepath.Join(dir, "M.tla")), &out, &errout)
			mode := fmt.Sprintf("with %d worker%s on %d cores.", tc.workers, plural(tc.workers), runtime.NumCPU())
			if code != 0 || errout.Len() != 0 || !strings.Contains(out.String(), mode) || !strings.Contains(out.String(), "65 states generated, 16 distinct states found, 0 states left on queue.") {
				t.Fatalf("exit %d; stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runJavaToolMessages(t, jar, dir, append(append([]string{"-tool"}, tc.args...), "M.tla"), code)
				if got := errorBehaviorMessages(readToolMessages(out.String())); !reflect.DeepEqual(got, java) {
					t.Fatalf("messages differ\nJava: %q\nGo:   %q", java, got)
				}
			}
		})
	}
}

func TestJavaTLCInvalidWorkerAndCheckpointOptions(t *testing.T) {
	for _, tc := range []struct {
		args    []string
		message string
	}{
		{[]string{"-workers", "0", "M.tla"}, "Error: at least one worker required."},
		{[]string{"-workers", "-2", "M.tla"}, "Error: at least one worker required."},
		{[]string{"-workers", "many", "M.tla"}, "Error: worker number or 'auto' required. But encountered many"},
		{[]string{"M.tla", "-workers"}, "Error: expect an integer or 'auto' for -workers option."},
		{[]string{"-checkpoint", "-1", "M.tla"}, "Error: expect a nonnegative integer for -checkpoint option."},
		{[]string{"-checkpoint", "soon", "M.tla"}, "Error: An integer for checkpoint interval is required. But encountered soon"},
		{[]string{"M.tla", "-checkpoint"}, "Error: checkpoint interval required."},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, workerModel, "INIT Init\nNEXT Next\n")
			var out, errout bytes.Buffer
			if code := run(context.Background(), tc.args, &out, &errout); code != 1 || errout.String() != tc.message+"\n" {
				t.Fatalf("exit %d; stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				cmd := exec.Command("java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-jar", jar}, tc.args...)...)
				cmd.Dir = dir
				output, _ := cmd.CombinedOutput()
				if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 1 || !strings.Contains(string(output), tc.message) {
					t.Fatalf("Java exit %v; output:\n%s", cmd.ProcessState, output)
				}
			}
		})
	}
}
