package checker

import (
	"context"
	"strings"
	"testing"
)

func TestCoverageParameterizedGuardRejectsBeforeLiveSnapshot(t *testing.T) {
	const source = `---- MODULE M ----
EXTENDS Naturals
VARIABLE x
Init == x=0
A(n) == x<n /\ x'=n
Next == A(1) \/ A(2)
====`
	const cfg = "INIT Init NEXT Next CHECK_DEADLOCK FALSE"
	var progress LiveProgress
	var events int
	_, err := CheckWithOptions(context.Background(), source, cfg, Options{Coverage: true, LiveProgress: &progress, OnSearch: func(SearchEvent) { events++ }})
	if err == nil || !strings.Contains(err.Error(), "-coverage does not yet support") || progress.CoverageSnapshot() != nil || events != 0 {
		t.Fatalf("error %v, coverage %+v, search events %d", err, progress.CoverageSnapshot(), events)
	}
	result, err := CheckWithOptions(context.Background(), source, cfg, Options{})
	if err != nil || result.Status != Success || result.GeneratedStates != 4 || result.DistinctStates != 3 || len(result.Coverage) != 0 {
		t.Fatalf("default result %+v error %v", result, err)
	}
}
