package upstreamjunit

import (
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
)

// The pinned diagnostic snapshot contains selected observations, not JUnit
// results. A difference category does not retain the differing message values.
//
//go:embed testdata/snapshots/sweep-aadcf4ba.json
var regressionBaseline []byte

const (
	regressionBaselineSHA = "ce2858eaaba92d6645d3b6b6d144ea37ff0065667977c3c0c65525032a0e4752"
	regressionBaseCommit  = "aadcf4ba4d9e07bd03c6d06c6086da1bfe123c29"
)

// regressionExclusions lists known nondeterministic cases. This is not an
// exhaustive list of nondeterminism in the diagnostic harness.
var regressionExclusions = map[string]string{
	"tlc2.tool.ContinueTest":                        "parallel DOT ordering",
	"tlc2.tool.DiameterTest":                        "parallel depth and DOT ordering",
	"tlc2.tool.RandomElementT4Test":                 "random multi-worker execution",
	"tlc2.tool.RandomSubsetNextT4Test":              "random multi-worker execution",
	"tlc2.tool.TSnapShotTest":                       "parallel execution order",
	"tlc2.tool.liveness.CodePlexBug08EWD840FL1Test": "time-based liveness checks",
}

// SweepRegressionFinding is an observation requiring investigation. A newly
// observed difference can be nondeterministic and is not proof of regression.
type SweepRegressionFinding struct {
	Class  string
	Kind   string
	Detail string
}

// SweepRegression is a selected-output comparison, not a JUnit pass count.
// Unresolved lists cases for which the baseline cannot establish a comparison.
type SweepRegression struct {
	Findings           []SweepRegressionFinding
	Excluded           []SweepRegressionFinding
	Unresolved         []SweepRegressionFinding
	ExistingMismatches int
	Compared           int
}

// PinnedRegressionBaseline returns the exact recorded reference snapshot.
func PinnedRegressionBaseline() (SweepSnapshot, error) {
	if fmt.Sprintf("%x", sha256.Sum256(regressionBaseline)) != regressionBaselineSHA {
		return SweepSnapshot{}, fmt.Errorf("diagnostic baseline hash mismatch")
	}
	var baseline SweepSnapshot
	if err := json.Unmarshal(regressionBaseline, &baseline); err != nil {
		return SweepSnapshot{}, err
	}
	if baseline.Schema != 1 || baseline.TlaGoCommit != regressionBaseCommit || baseline.UpstreamCommit != UpstreamCommit || baseline.JarSHA256 != JarSHA256 {
		return SweepSnapshot{}, fmt.Errorf("diagnostic baseline provenance mismatch")
	}
	return baseline, nil
}

// CompareSweepRegression detects new selected-difference categories and changed
// exit values. Unavailable observations and changed Java results or selection
// scopes are inconclusive, not passes. Baseline cases that were already
// unavailable remain unresolved. Inputs must be a trusted parsed baseline and
// a complete ParseSweepLog result; the candidate's binary provenance is not
// established by this comparison.
func CompareSweepRegression(baseline, candidate SweepSnapshot) (SweepRegression, error) {
	old, err := regressionIndex(baseline.Classes)
	if err != nil {
		return SweepRegression{}, err
	}
	current, err := regressionIndex(candidate.Classes)
	if err != nil {
		return SweepRegression{}, err
	}
	var out SweepRegression
	for class, previous := range old {
		observed, ok := current[class]
		if !ok {
			out.add(class, "inconclusive", "candidate missing from sweep")
			continue
		}
		if reason := regressionExclusions[class]; reason != "" {
			detail := fmt.Sprintf("%s; candidate=%s/%s", reason, observed.Status, observed.Unavailable)
			out.Excluded = append(out.Excluded, SweepRegressionFinding{Class: class, Kind: "nondeterministic", Detail: detail})
			continue
		}
		out.compare(previous, observed)
	}
	for class := range current {
		if _, ok := old[class]; !ok {
			out.add(class, "inconclusive", "candidate absent from baseline")
		}
	}
	sortFindings(out.Findings)
	sortFindings(out.Excluded)
	sortFindings(out.Unresolved)
	return out, nil
}

func regressionIndex(records []SweepRecord) (map[string]SweepRecord, error) {
	out := make(map[string]SweepRecord, len(records))
	for _, rec := range records {
		if rec.Class == "" || !validRegressionRecord(rec) {
			return nil, fmt.Errorf("invalid diagnostic record %q", rec.Class)
		}
		if _, exists := out[rec.Class]; exists {
			return nil, fmt.Errorf("duplicate diagnostic record %q", rec.Class)
		}
		out[rec.Class] = rec
	}
	return out, nil
}

func validRegressionRecord(rec SweepRecord) bool {
	switch rec.Status {
	case "compared":
		return validComparedRecord(rec)
	case "unsafe_setup":
		return rec.Unavailable == "" && validUnobservedRecord(rec)
	case "unavailable":
		return rec.Unavailable != "" && validUnobservedRecord(rec)
	default:
		return false
	}
}

func validComparedRecord(rec SweepRecord) bool {
	return rec.JavaExit != nil && rec.GoExit != nil && rec.Unavailable == "" && validComparedObservations(rec)
}

func validUnobservedRecord(rec SweepRecord) bool {
	return rec.JavaExit == nil && rec.GoExit == nil && len(rec.Differences) == 0
}

func validComparedObservations(rec SweepRecord) bool {
	if slices.Contains(rec.Differences, "exit") != (*rec.JavaExit != *rec.GoExit) {
		return false
	}
	for _, difference := range rec.Differences {
		if !diffID.MatchString(difference) || !selectedDifference(rec, difference) {
			return false
		}
	}
	return true
}

func selectedDifference(rec SweepRecord, difference string) bool {
	if id, ok := strings.CutPrefix(difference, "message:"); ok {
		return slices.Contains(rec.Bodies, id)
	}
	if id, ok := strings.CutPrefix(difference, "presence:"); ok {
		return slices.Contains(rec.Presence, id)
	}
	return true
}

func (r *SweepRegression) compare(previous, current SweepRecord) {
	if previous.Status != "compared" {
		detail := fmt.Sprintf("baseline=%s/%s candidate=%s/%s", previous.Status, previous.Unavailable, current.Status, current.Unavailable)
		r.Unresolved = append(r.Unresolved, SweepRegressionFinding{Class: previous.Class, Kind: previous.Status, Detail: detail})
		return
	}
	if reason := regressionInconclusive(previous, current); reason != "" {
		r.add(previous.Class, "inconclusive", reason)
		return
	}
	r.Compared++
	if len(previous.Differences) > 0 {
		r.ExistingMismatches++
	}
	r.compareDifferences(previous, current)
}

func regressionInconclusive(previous, current SweepRecord) string {
	if current.Status != "compared" {
		return "previously compared case is " + current.Status + ": " + current.Unavailable
	}
	if !slices.Equal(previous.Bodies, current.Bodies) || !slices.Equal(previous.Presence, current.Presence) {
		return "selected message scope changed"
	}
	if *previous.JavaExit != *current.JavaExit {
		return "Java exit changed"
	}
	return ""
}

func (r *SweepRegression) compareDifferences(previous, current SweepRecord) {
	for _, difference := range current.Differences {
		if !slices.Contains(previous.Differences, difference) {
			r.add(previous.Class, "new_difference", difference)
		}
	}
	for _, difference := range previous.Differences {
		if !slices.Contains(current.Differences, difference) {
			r.add(previous.Class, "no_longer_observed", difference)
		}
	}
	if slices.Contains(previous.Differences, "exit") && slices.Contains(current.Differences, "exit") && *previous.GoExit != *current.GoExit {
		r.add(previous.Class, "new_difference", fmt.Sprintf("Go exit changed from %d to %d", *previous.GoExit, *current.GoExit))
	}
}

func (r *SweepRegression) add(class, kind, detail string) {
	r.Findings = append(r.Findings, SweepRegressionFinding{Class: class, Kind: kind, Detail: detail})
}

func sortFindings(findings []SweepRegressionFinding) {
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Class != findings[j].Class {
			return findings[i].Class < findings[j].Class
		}
		if findings[i].Kind != findings[j].Kind {
			return findings[i].Kind < findings[j].Kind
		}
		return findings[i].Detail < findings[j].Detail
	})
}
