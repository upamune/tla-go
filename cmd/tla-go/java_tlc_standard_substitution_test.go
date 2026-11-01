package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/upamune/tla-go/internal/javatmp"
)

// The models are unchanged from Java TLC v1.7.4, commit 5a47802.
func TestJavaTLCStandardOperatorSubstitution(t *testing.T) {
	for _, name := range []string{"test30", "test999"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			for _, ext := range []string{".tla", ".cfg"} {
				data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", name+ext))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, name+ext), data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, name+".tla")}, &out, &errout)
			checkStandardSubstitutionOutput(t, code, out.String(), errout.String())
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
				if err != nil {
					t.Fatalf("Java TLC: %v\n%s", err, java)
				}
				checkStandardSubstitutionOutput(t, 0, string(java), "")
			}
		})
	}
}

func checkStandardSubstitutionOutput(t *testing.T, code int, stdout, stderr string) {
	t.Helper()
	stats := "2 states generated, 1 distinct states found, 0 states left on queue."
	if code != 0 || stderr != "" || !strings.Contains(stdout, stats) {
		t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	for i := 1; i <= 7; i++ {
		if message := fmt.Sprintf("\"Test %d OK\"  TRUE", i); !strings.Contains(stdout, message) {
			t.Fatalf("missing %s in output:\n%s", message, stdout)
		}
	}
}
