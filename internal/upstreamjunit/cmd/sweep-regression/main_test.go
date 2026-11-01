package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/upamune/tla-go/internal/upstreamjunit"
)

func TestReportExit(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result upstreamjunit.SweepRegression
		want   int
	}{
		{"complete", upstreamjunit.SweepRegression{}, 0},
		{"unresolved", upstreamjunit.SweepRegression{Unresolved: []upstreamjunit.SweepRegressionFinding{{Class: "A", Kind: "unavailable", Detail: "java_timeout"}}}, 2},
		{"excluded", upstreamjunit.SweepRegression{Excluded: []upstreamjunit.SweepRegressionFinding{{Class: "B", Kind: "nondeterministic", Detail: "parallel"}}}, 2},
		{"known mismatch remains", upstreamjunit.SweepRegression{RemainingMismatches: 1}, 2},
		{"missing or changed scope", upstreamjunit.SweepRegression{Findings: []upstreamjunit.SweepRegressionFinding{{Class: "C", Kind: "inconclusive"}}}, 2},
		{"prior difference not observed", upstreamjunit.SweepRegression{Findings: []upstreamjunit.SweepRegressionFinding{{Class: "D", Kind: "no_longer_observed"}}}, 0},
		{"unknown finding", upstreamjunit.SweepRegression{Findings: []upstreamjunit.SweepRegressionFinding{{Class: "D", Kind: "new-unrecognized-kind"}}}, 2},
		{"new difference takes precedence", upstreamjunit.SweepRegression{Unresolved: []upstreamjunit.SweepRegressionFinding{{Class: "A"}}, Findings: []upstreamjunit.SweepRegressionFinding{{Class: "E", Kind: "new_difference"}}}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if got := writeReport(&out, upstreamjunit.SweepSnapshot{}, tc.result, [32]byte{}); got != tc.want {
				t.Fatalf("exit %d, want %d: %s", got, tc.want, &out)
			}
			if !strings.Contains(out.String(), "not a JUnit pass") || !strings.Contains(out.String(), "provenance are not established") {
				t.Fatalf("missing limitations: %s", &out)
			}
		})
	}
}

func TestBinaryPreservesExitCodes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	bin := filepath.Join(t.TempDir(), "sweep-regression")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if output, err := exec.CommandContext(ctx, "go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	for _, tc := range []struct {
		name, differences, marker string
		want                      int
	}{
		{"inconclusive", "", "candidate missing from sweep", 2},
		{"new difference", "message:2199", "new_difference pcal.BakeryTest", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			log := fmt.Sprintf(`    integration_test.go:1: unresolved pcal.BakeryTest spec="Bakery"
    integration_test.go:2: diagnostic pcal.BakeryTest differences=[%s] selected bodies=[2190 2194 2199] presence=[1000 2186]; partial comparison, liveness strategy unresolved, not a match; Java exit=0 Go exit=0
    integration_test.go:3: incomplete sweep: 1 candidates with recovered spec, 1 diagnostic runs, 0 candidates with unsafe or unresolved setup, 1 unresolved non-abstract candidates; inventory is not an exhaustive JUnit count
--- FAIL: TestUpstreamJUnitSweep (1.00s)
`, tc.differences)
			path := filepath.Join(t.TempDir(), "sweep.log")
			if err := os.WriteFile(path, []byte(log), 0o600); err != nil {
				t.Fatal(err)
			}
			output, err := exec.CommandContext(ctx, bin, path).CombinedOutput()
			exit, ok := errors.AsType[*exec.ExitError](err)
			if !ok || exit.ExitCode() != tc.want || !bytes.Contains(output, []byte(tc.marker)) {
				t.Fatalf("exit: %v, want %d\n%s", err, tc.want, output)
			}
		})
	}
}

func TestRunRejectsIncompleteLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log")
	if err := os.WriteFile(path, []byte("incomplete"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errout bytes.Buffer
	if code := run([]string{path}, &out, &errout); code != 2 || out.Len() != 0 || !strings.Contains(errout.String(), "missing summary") {
		t.Fatalf("exit %d, stdout %s, stderr %s", code, &out, &errout)
	}
}
