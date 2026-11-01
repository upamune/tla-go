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

	"github.com/upamune/tla-go/internal/javatmp"
)

// The models are unchanged from Java TLC v1.7.4, commit 5a47802.
func TestJavaTLCSymbolicOperatorCalls(t *testing.T) {
	for _, name := range []string{"test204", "test205"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			for _, file := range []string{"test204.tla", "test204.cfg", "test205.tla", "test205.cfg"} {
				data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", file))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, file), data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, name+".tla")}, &out, &errout)
			stats := "0 states generated, 0 distinct states found, 0 states left on queue."
			if code != 0 || errout.Len() != 0 || !strings.Contains(out.String(), stats) {
				t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				jar, err := filepath.Abs(jar)
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, "java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-workers", "1", "-fp", "0", name+".tla")
				cmd.Dir = dir
				java, err := javatmp.Apply(cmd).CombinedOutput()
				if err != nil || !strings.Contains(string(java), stats) {
					t.Fatalf("Java TLC %v:\n%s", err, java)
				}
			}
		})
	}
}
