package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestJavaTLCUpstreamFastMutex(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"FastMutexWithGoto2.tla", "FastMutexWithGoto2.cfg"} {
		data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", "fast-mutex", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	checkSymmetryRecurrence(t, dir, "FastMutexWithGoto2", 0, "42277 states generated, 15900 distinct states found, 0 states left on queue")
}
