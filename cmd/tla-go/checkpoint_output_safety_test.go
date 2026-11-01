package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/upamune/tla-go/internal/javatmp"
)

func createPausedCheckpoint(t *testing.T, dir string) (string, string, []byte) {
	t.Helper()
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == /\\ x' = x + 1 /\\ x < 4\n====\n"
	spec := filepath.Join(dir, "M.tla")
	if err := os.WriteFile(spec, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "states")
	t.Setenv("TLA_GO_DEBUG_CHECKPOINT_AFTER", "1")
	var out, errout bytes.Buffer
	if code := run(context.Background(), []string{"-checkpoint", "1", "-metadir", root, spec}, &out, &errout); code != 255 || !strings.Contains(errout.String(), "debug checkpoint saved") {
		t.Fatalf("checkpoint creation: %d %s %s", code, &out, &errout)
	}
	t.Setenv("TLA_GO_DEBUG_CHECKPOINT_AFTER", "")
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 {
		t.Fatalf("checkpoint metadata: %v %v", entries, err)
	}
	recovery := filepath.Join(root, entries[0].Name())
	checkpoint, err := os.ReadFile(filepath.Join(recovery, "tla-go.chkpt"))
	if err != nil || len(checkpoint) == 0 {
		t.Fatalf("read checkpoint: %d bytes, %v", len(checkpoint), err)
	}
	return spec, recovery, checkpoint
}

func requireCheckpointUnchanged(t *testing.T, path string, want []byte) {
	t.Helper()
	actual, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(actual, want) {
		t.Fatalf("checkpoint changed: got %d bytes, want %d, err %v", len(actual), len(want), err)
	}
}

func TestRecoveryRejectsDumpBeforeCreatingOutputs(t *testing.T) {
	dir := t.TempDir()
	spec, recovery, checkpoint := createPausedCheckpoint(t, dir)
	path := filepath.Join(recovery, "tla-go.chkpt")
	for _, format := range []string{"dot", "text"} {
		for _, location := range []string{"metadata", "absolute"} {
			for _, link := range []string{"symlink", "hardlink"} {
				t.Run(format+"/"+location+"/"+link, func(t *testing.T) {
					testRecoveryDumpAlias(t, dir, spec, recovery, path, checkpoint, format, location, link)
				})
			}
		}
	}
	// An unsupported option must not destroy the state needed by a later run.
	var out, errout bytes.Buffer
	if code := run(context.Background(), []string{"-checkpoint", "0", "-recover", recovery, spec}, &out, &errout); code != 0 {
		t.Fatalf("subsequent recovery: %d %s %s", code, &out, &errout)
	}
	requireCheckpointUnchanged(t, path, checkpoint)
}

func testRecoveryDumpAlias(t *testing.T, dir, spec, recovery, checkpointPath string, checkpoint []byte, format, location, link string) {
	t.Helper()
	suffix := ".dump"
	if format == "dot" {
		suffix = ".dot"
	}
	base := filepath.Join(dir, "alias-"+format+"-"+location+"-"+link)
	if location == "metadata" {
		base = filepath.Join(recovery, "alias-"+format+"-"+link)
	}
	alias := base + suffix
	if link == "symlink" {
		if err := os.Symlink(checkpointPath, alias); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
	} else if err := os.Link(checkpointPath, alias); err != nil {
		t.Skipf("hardlink unavailable: %v", err)
	}
	dump := base
	if location == "metadata" {
		dump = filepath.Join("${metadir}", filepath.Base(base))
	}
	trace := filepath.Join(dir, "trace-"+format+"-"+location+"-"+link)
	recorder := filepath.Join(dir, "recorder-"+format+"-"+location+"-"+link)
	t.Setenv("TLA_GO_DEBUG_RECORDER", recorder)
	args := []string{"-tool", "-debug-trace", trace, "-recover", recovery, "-dump"}
	if format == "dot" {
		args = append(args, "dot")
	}
	args = append(args, dump, spec)
	var out, errout bytes.Buffer
	if code := run(context.Background(), args, &out, &errout); code != 255 || !strings.Contains(errout.String(), "checkpoint recovery does not support these options") {
		t.Fatalf("recovery accepted dump: %d %s %s", code, &out, &errout)
	}
	requireCheckpointUnchanged(t, checkpointPath, checkpoint)
	for _, output := range []string{trace, recorder} {
		if _, err := os.Lstat(output); !os.IsNotExist(err) {
			t.Fatalf("diagnostic output created before rejection: %s %v", output, err)
		}
	}
}

func TestRecoveryRejectsUserFileBeforeCreatingOutputs(t *testing.T) {
	dir := t.TempDir()
	spec, recovery, checkpoint := createPausedCheckpoint(t, dir)
	path := filepath.Join(recovery, "tla-go.chkpt")
	for _, link := range []string{"direct", "symlink", "hardlink"} {
		t.Run(link, func(t *testing.T) {
			output := path
			if link != "direct" {
				output = filepath.Join(dir, "user-"+link)
				var err error
				if link == "symlink" {
					err = os.Symlink(path, output)
				} else {
					err = os.Link(path, output)
				}
				if err != nil {
					t.Skipf("link unavailable: %v", err)
				}
			}
			trace := filepath.Join(dir, "user-trace-"+link)
			var out, errout bytes.Buffer
			if code := run(context.Background(), []string{"-debug-trace", trace, "-recover", recovery, "-userFile", output, spec}, &out, &errout); code != 255 || !strings.Contains(errout.String(), "checkpoint recovery does not support these options") {
				t.Fatalf("recovery accepted userFile: %d %s %s", code, &out, &errout)
			}
			requireCheckpointUnchanged(t, path, checkpoint)
			if _, err := os.Lstat(trace); !os.IsNotExist(err) {
				t.Fatalf("trace created before rejection: %v", err)
			}
		})
	}
}

func TestRecoveryCleanupRetainsCheckpoint(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	spec, recovery, checkpoint := createPausedCheckpoint(t, dir)
	path := filepath.Join(recovery, "tla-go.chkpt")
	sentinel := filepath.Join(dir, "states", "sentinel")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errout bytes.Buffer
	if code := run(context.Background(), []string{"-cleanup", "-checkpoint", "0", "-recover", recovery, spec}, &out, &errout); code != 0 {
		t.Fatalf("cleanup recovery: %d %s %s", code, &out, &errout)
	}
	requireCheckpointUnchanged(t, path, checkpoint)
	if content, err := os.ReadFile(sentinel); err != nil || string(content) != "keep" {
		t.Fatalf("cleanup removed states during recovery: %q %v", content, err)
	}
	checkJavaRecoveryCleanup(t)
}

// Java TLC skips -cleanup whenever -recover is supplied, including when the
// referenced recovery directory cannot be loaded.
func checkJavaRecoveryCleanup(t *testing.T) {
	t.Helper()
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		return
	}
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n")
	states := filepath.Join(dir, "states")
	if err := os.Mkdir(states, 0o700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(states, "sentinel")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-jar", jar, "-cleanup", "-recover", filepath.Join(states, "missing"), "M.tla")
	cmd.Dir = dir
	output, err := javatmp.Apply(cmd).CombinedOutput()
	if err == nil {
		t.Fatalf("expected missing recovery to fail: %s", output)
	}
	if content, err := os.ReadFile(sentinel); err != nil || string(content) != "keep" {
		t.Fatalf("Java cleanup removed states during recovery: %q %v; output %s", content, err, output)
	}
}

func TestRecoveryRejectsDumpBeforeProfileCollision(t *testing.T) {
	dir := t.TempDir()
	spec, recovery, checkpoint := createPausedCheckpoint(t, dir)
	path := filepath.Join(recovery, "tla-go.chkpt")
	for _, link := range []string{"direct", "symlink", "hardlink"} {
		t.Run(link, func(t *testing.T) {
			profile := path
			if link != "direct" {
				profile = filepath.Join(dir, "profile-"+link)
				var err error
				if link == "symlink" {
					err = os.Symlink(path, profile)
				} else {
					err = os.Link(path, profile)
				}
				if err != nil {
					t.Skipf("link unavailable: %v", err)
				}
			}
			var out, errout bytes.Buffer
			args := []string{"-recover", recovery, "-debug-trace", profile, "-dump", "dot", "${metadir}/graph", spec}
			if code := run(context.Background(), args, &out, &errout); code != 255 || !strings.Contains(errout.String(), "checkpoint recovery does not support these options") {
				t.Fatalf("unsupported recovery: %d %s %s", code, &out, &errout)
			}
			requireCheckpointUnchanged(t, path, checkpoint)
			if _, err := os.Stat(filepath.Join(recovery, "graph.dot")); !os.IsNotExist(err) {
				t.Fatalf("dump created before rejection: %v", err)
			}
		})
	}
}
