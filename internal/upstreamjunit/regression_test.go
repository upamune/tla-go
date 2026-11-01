package upstreamjunit

import (
	"slices"
	"testing"
)

func comparedRecord(class string, java, goExit int, differences ...string) SweepRecord {
	return SweepRecord{Class: class, Status: "compared", JavaExit: &java, GoExit: &goExit, Differences: differences, Bodies: []string{"2192", "2199"}, Presence: []string{"1000"}}
}

func TestSweepRegressionPinnedBaseline(t *testing.T) {
	baseline, err := PinnedRegressionBaseline()
	if err != nil {
		t.Fatal(err)
	}
	got, err := CompareSweepRegression(baseline, baseline)
	if err != nil || len(got.Findings) != 0 || len(got.Excluded) != len(regressionExclusions) || len(got.Unresolved) == 0 || got.ExistingMismatches == 0 {
		t.Fatalf("baseline comparison: %+v %v", got, err)
	}
}

func TestSweepRegressionFindings(t *testing.T) {
	base := comparedRecord("example.A", 0, 0)
	known := comparedRecord("example.Known", 0, 1, "exit", "message:2192")
	tests := []struct {
		name       string
		old, next  SweepRecord
		kind       string
		detail     string
		unresolved bool
	}{
		{"same", base, base, "", "", false},
		{"new message", base, comparedRecord("example.A", 0, 0, "message:2199"), "new_difference", "message:2199", false},
		{"new exit", base, comparedRecord("example.A", 0, 1, "exit"), "new_difference", "exit", false},
		{"changed exit", known, comparedRecord("example.Known", 0, 2, "exit", "message:2192"), "new_difference", "Go exit changed from 1 to 2", false},
		{"old category absent", known, comparedRecord("example.Known", 0, 0, "message:2192"), "no_longer_observed", "exit", false},
		{"java changed", base, comparedRecord("example.A", 1, 1), "inconclusive", "Java exit changed", false},
		{"timeout after comparison", base, SweepRecord{Class: "example.A", Status: "unavailable", Unavailable: "java_timeout"}, "inconclusive", "previously compared case is unavailable: java_timeout", false},
		{"unsafe after comparison", base, SweepRecord{Class: "example.A", Status: "unsafe_setup"}, "inconclusive", "previously compared case is unsafe_setup: ", false},
		{"unavailable baseline", SweepRecord{Class: "example.A", Status: "unavailable", Unavailable: "go_timeout"}, base, "", "", true},
		{"unsafe baseline", SweepRecord{Class: "example.A", Status: "unsafe_setup"}, base, "", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CompareSweepRegression(SweepSnapshot{Classes: []SweepRecord{tc.old}}, SweepSnapshot{Classes: []SweepRecord{tc.next}})
			if err != nil {
				t.Fatal(err)
			}
			if tc.kind == "" && len(got.Findings) != 0 {
				t.Fatalf("unexpected findings: %+v", got)
			}
			if tc.kind != "" && !slices.Contains(got.Findings, SweepRegressionFinding{Class: tc.old.Class, Kind: tc.kind, Detail: tc.detail}) {
				t.Fatalf("missing finding: %+v", got)
			}
			if (len(got.Unresolved) > 0) != tc.unresolved {
				t.Fatalf("unexpected unresolved: %+v", got.Unresolved)
			}
		})
	}
}

func TestSweepRegressionScopeAndNondeterminism(t *testing.T) {
	base := comparedRecord("example.A", 0, 0)
	changed := base
	changed.Bodies = []string{"2194"}
	nondeterministic := comparedRecord("tlc2.tool.DiameterTest", 0, 0)
	newCase := comparedRecord("example.New", 0, 0)
	got, err := CompareSweepRegression(SweepSnapshot{Classes: []SweepRecord{base, nondeterministic}}, SweepSnapshot{Classes: []SweepRecord{changed, nondeterministic, newCase}})
	if err != nil || len(got.Findings) != 2 || len(got.Excluded) != 1 || got.Findings[0].Detail != "selected message scope changed" || got.Findings[1].Detail != "candidate absent from baseline" {
		t.Fatalf("scope: %+v %v", got, err)
	}
	got, err = CompareSweepRegression(SweepSnapshot{Classes: []SweepRecord{nondeterministic}}, SweepSnapshot{})
	if err != nil || len(got.Findings) != 1 || got.Findings[0].Detail != "candidate missing from sweep" {
		t.Fatalf("missing excluded case: %+v %v", got, err)
	}
}

func TestSweepRegressionRejectsBadRecords(t *testing.T) {
	valid := comparedRecord("example.A", 0, 0)
	for _, records := range [][]SweepRecord{
		{valid, valid},
		{comparedRecord("example.A", 0, 0, "exit")},
		{comparedRecord("example.A", 0, 0, "message:9999")},
		{comparedRecord("example.A", 0, 0, "not-a-category")},
		{{Class: "example.A", Status: "compared"}},
		{{Class: "example.A", Status: "unknown"}},
		{{Class: "", Status: "unsafe_setup"}},
	} {
		if _, err := CompareSweepRegression(SweepSnapshot{Classes: records}, SweepSnapshot{}); err == nil {
			t.Fatalf("accepted invalid records: %+v", records)
		}
	}
}
