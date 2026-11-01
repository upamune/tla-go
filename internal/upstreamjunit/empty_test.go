package upstreamjunit

import (
	"slices"
	"strings"
	"testing"
)

func TestProjectEmptyFailClosed(t *testing.T) {
	good := msg("2186", "Finished in 1ms at (2026-11-01 00:00:00)") + msg("2199", "0 states generated, 0 distinct states found, 0 states left on queue.")
	for _, tc := range []struct {
		name, output string
		args         []string
		result       Result
		ok           bool
	}{
		{"valid", good, emptyInvocation(), Result{}, true},
		{"disabled coverage", good, slices.Delete(slices.Clone(emptyInvocation()), 4, 6), Result{}, false},
		{"error exit", good, emptyInvocation(), Result{ExitCode: 10}, false},
		{"timeout", good, emptyInvocation(), Result{TimedOut: true}, false},
		{"malformed stats", strings.Replace(good, "0 states generated", "oops", 1), emptyInvocation(), Result{}, false},
		{"malformed frame", good + "@!@!@STARTMSG", emptyInvocation(), Result{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.result.Output = tc.output
			_, _, err := projectEmpty(tc.args, tc.result)
			if (err == nil) != tc.ok {
				t.Fatalf("error=%v", err)
			}
		})
	}
	records, omissions, err := projectEmpty(emptyInvocation(), Result{Output: good + msg("1000", "error") + msg("9999", "unknown")})
	if err != nil || !slices.ContainsFunc(records, func(r bridgeRecord) bool { return r.code == "1000" }) || !strings.Contains(strings.Join(omissions, " "), "9999") {
		t.Fatalf("records=%v omissions=%v err=%v", records, omissions, err)
	}
}

func TestProjectEmptyCoverageCodes(t *testing.T) {
	good := msg("2186", "Finished in 1ms at (2026-11-01 00:00:00)") + msg("2199", "0 states generated, 0 distinct states found, 0 states left on queue.")
	// Explicitly enumerate the pinned Java TLC's TLC_COVERAGE_* constants so
	// every message is tested independently, including diagnostics without a report.
	for _, id := range []string{"2201", "2202", "2221", "2772", "2773", "2774", "2775", "2776", "2777"} {
		t.Run("reject "+id, func(t *testing.T) {
			_, _, err := projectEmpty(emptyInvocation(), Result{Output: good + msg(id, "unexpected")})
			if err == nil || !strings.Contains(err.Error(), "coverage message "+id) {
				t.Fatalf("expected coverage error for %s, got %v", id, err)
			}
		})
	}
	for _, id := range []string{"2200", "2203", "2220", "2222", "2771", "2778"} {
		t.Run("accept "+id, func(t *testing.T) {
			_, _, err := projectEmpty(emptyInvocation(), Result{Output: good + msg(id, "unrelated")})
			if err != nil {
				t.Fatalf("unexpected rejection of %s: %v", id, err)
			}
		})
	}
}
