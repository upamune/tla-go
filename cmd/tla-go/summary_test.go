package main

import (
	"testing"

	"github.com/upamune/tla-go/checker"
)

// These cases are from tla-ts tests/unit/tlc-summary.test.ts.
func TestUpstreamProbabilityFormatting(t *testing.T) {
	for _, tc := range []struct {
		value float64
		want  string
	}{
		{0, "0.0"},
		{3.1e-8, "3.1E-8"},
		{1.234e-5, "1.2E-5"},
		{0.00095, "9.5E-4"},
		{0.5, ".5"},
		{0.0123, ".012"},
		{0.999, "1.00"},
		{1, "1.0"},
		{9.96e-4, "10.0E-4"},
		{2.5e-10, "2.5E-10"},
		{1.95e-7, "2.0E-7"},
		{0.007, ".007"},
		{0.0995, ".100"},
	} {
		if got := formatProbability(tc.value); got != tc.want {
			t.Errorf("%g: got %q, want %q", tc.value, got, tc.want)
		}
	}
}

func TestUpstreamOutDegreeSummary(t *testing.T) {
	buckets := make([]int, 32)
	for _, degree := range []int{0, 1, 1, 2, 40} {
		buckets[min(degree, 31)]++
	}
	want := "The average outdegree of the complete state graph is 7 (minimum is 0, the maximum 31 and the 95th percentile is 31)."
	if got := outDegreeSummary(buckets); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestTemporalHeadlines(t *testing.T) {
	for _, tc := range []struct {
		names []string
		want  string
	}{
		{[]string{"One"}, "Error: Temporal property One was violated."},
		{[]string{"One", "Two"}, "Error: Temporal properties One and Two were violated."},
		{[]string{"One", "Two", "Three"}, "Error: Temporal properties One, Two, and Three were violated."},
	} {
		if got := temporalHeadline(&checker.Violation{Properties: tc.names}); got != tc.want {
			t.Errorf("got %q, want %q", got, tc.want)
		}
	}
}
