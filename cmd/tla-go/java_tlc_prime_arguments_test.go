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

// These unchanged suite models exercise primed operator arguments and ENABLED
// against Java TLC v1.7.4, commit 5a47802b5c391f59ecdd44117981f4ff8c0656ba.
func TestJavaTLCPrimedArgumentModels(t *testing.T) {
	for _, tc := range []struct {
		name, printed                 string
		generated, distinct, maxDepth int
	}{
		{"test17", `"Starting Test 25"  TRUE`, 7, 2, 2},
		{"test20", `"Test N4 Passed"  TRUE`, 5, 1, 1},
		{"test21", `"Test 11 OK"  TRUE`, 8, 2, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, ext := range []string{".tla", ".cfg"} {
				name := tc.name + ext
				data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", name))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, tc.name+".tla")}, &out, &errout)
			stats := fmt.Sprintf("%d states generated, %d distinct states found, 0 states left on queue.", tc.generated, tc.distinct)
			depth := fmt.Sprintf("The depth of the complete state graph search is %d.", tc.maxDepth)
			if code != 0 || errout.Len() != 0 || !strings.Contains(out.String(), stats) || !strings.Contains(out.String(), depth) || !strings.Contains(out.String(), tc.printed) {
				t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				jar, err := filepath.Abs(jar)
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, "java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-workers", "1", "-fp", "0", tc.name+".tla")
				cmd.Dir = dir
				java, err := javatmp.Apply(cmd).CombinedOutput()
				if err != nil || !strings.Contains(string(java), stats) || !strings.Contains(string(java), depth) || !strings.Contains(string(java), tc.printed) {
					t.Fatalf("Java TLC %v:\n%s", err, java)
				}
			}
		})
	}
}

func TestJavaTLCPrimedNestedArguments(t *testing.T) {
	dir := t.TempDir()
	source := `---- MODULE M ----
EXTENDS Naturals
VARIABLES x, y
D(a) == a'
E(a) == D(a)
F(a) == D(a + 1)
G(a) == (a + 2)'
H(a) == (a = 2)'
Init == /\ x = 1 /\ y = 7
Next == /\ x' = 2 /\ y' = 9 /\ E(x) = 2 /\ F(x) = 3 /\ D(x + 1) = 3 /\ G(x) = 4 /\ H(x) /\ D(y) = 9
====
`
	for name, content := range map[string]string{"M.tla": source, "M.cfg": "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
	stats := "3 states generated, 2 distinct states found, 0 states left on queue."
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
		cmd := exec.CommandContext(ctx, "java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-workers", "1", "-fp", "0", "M.tla")
		cmd.Dir = dir
		java, err := javatmp.Apply(cmd).CombinedOutput()
		if err != nil || !strings.Contains(string(java), stats) {
			t.Fatalf("Java TLC %v:\n%s", err, java)
		}
	}
}
