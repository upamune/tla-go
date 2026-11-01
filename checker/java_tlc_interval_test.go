package checker

import (
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

// TLC_JAR enables a direct comparison with an independently supplied Java TLC.
// The cases intentionally cover behavior shared by the stable and current TLC.
func TestJavaTLCRandomSubsetIntervalSize(t *testing.T) {
	cases := []struct {
		name, interval, display string
		count                   int
		oversized               bool
	}{
		{"maximum-size-empty", "0..2147483646", "0..2147483646", 0, false},
		{"overflow-zero-count", "0..2147483647", "0..2147483647", 0, true},
		{"overflow-five", "0..2147483647", "0..2147483647", 5, true},
		{"negative-overflow", "(-2147483647)..0", "-2147483647..0", 5, true},
		{"non-power-of-two", "(-2147483647)..1", "-2147483647..1", 5, true},
		{"entire-range", "(-2147483647-1)..2147483647", "-2147483648..2147483647", 5, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := fmt.Sprintf("---- MODULE M ----\nEXTENDS Integers, Randomization, TLC\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\nASSUME PrintT(RandomSubset(%d, %s))\n====\n", tc.count, tc.interval)
			config := "INIT Init NEXT Next CHECK_DEADLOCK FALSE\n"
			var printed []string
			result, err := CheckWithOptions(context.Background(), source, config, Options{Seed: 1, OnPrint: func(s string) { printed = append(printed, s) }})
			if err != nil {
				t.Fatal(err)
			}
			wantMessage := "Size of interval value exceeds the maximum representable size (32bits): " + tc.display + "."
			if tc.oversized {
				if result.Status != AssumptionViolation || result.Violation == nil || !strings.Contains(result.Violation.Message, wantMessage) || len(printed) != 0 {
					t.Fatalf("got %+v, output %q; expected assumption error containing %q", result, printed, wantMessage)
				}
			} else if result.Status != Success || len(printed) != 1 || printed[0] != "{}" || result.GeneratedStates != 2 || result.DistinctStates != 1 || result.Depth != 1 {
				t.Fatalf("got %+v, output %q", result, printed)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				checkJavaTLCInterval(t, jar, source, config, tc.oversized, wantMessage)
			}
		})
	}
}

func checkJavaTLCInterval(t *testing.T, jar, source, config string, oversized bool, wantMessage string) {
	t.Helper()
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, contents := range map[string]string{"M.tla": source, "M.cfg": config} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-workers", "1", "-seed", "1", "-fp", "0", "M.tla")
	cmd.Dir = dir
	javatmp.Apply(cmd)
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("Java TLC did not finish: %v\n%s", ctx.Err(), output)
	}
	if oversized {
		if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 10 || !strings.Contains(string(output), wantMessage) {
			t.Fatalf("Java TLC: %v\n%s\nexpected exit 10 and %q", err, output, wantMessage)
		}
	} else if err != nil || !strings.Contains(string(output), "\n{}\n") || !strings.Contains(string(output), "2 states generated, 1 distinct states found") {
		t.Fatalf("Java TLC: %v\n%s", err, output)
	}
}
