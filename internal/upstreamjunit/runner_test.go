package upstreamjunit

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCopyFixtureIsolation(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "M.tla"), []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := copyFixture(src, dst, "M.tla"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "M.tla"), []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(src, "M.tla"))
	if err != nil || string(data) != "original" {
		t.Fatalf("source: %q %v", data, err)
	}
	if err := copyFixture(src, dst, "../escape"); err == nil {
		t.Fatal("accepted escaping path")
	}
}

func TestRunCommandTimeoutAndExit(t *testing.T) {
	r, err := runCommand(t.TempDir(), time.Second, "sh", "-c", "printf failure; exit 7")
	if err != nil || r.ExitCode != 7 || r.Output != "failure" {
		t.Fatalf("%+v %v", r, err)
	}
	r, err = runCommand(t.TempDir(), 10*time.Millisecond, "sleep", "1")
	if err == nil || !r.TimedOut {
		t.Fatalf("timeout: %+v %v", r, err)
	}
}
