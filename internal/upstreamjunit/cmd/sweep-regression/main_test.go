package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
