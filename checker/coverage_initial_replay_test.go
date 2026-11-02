package checker

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCoverageInitialConstraintFailureReplaysAfterHeader(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x \\in 0..1\nA == UNCHANGED x\nB == UNCHANGED x\nNext == A \\/ B\nLimit == IF PrintT(x) THEN IF x = 0 THEN TRUE ELSE x < TRUE ELSE FALSE\n===="
	var events []string
	result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next CONSTRAINT Limit CHECK_DEADLOCK FALSE", Options{
		Coverage:        true,
		OnPrint:         func(text string) { events = append(events, "print "+text) },
		OnFailureHeader: func(Failure) { events = append(events, "header") },
	})
	want := []string{"print 0", "print 1", "header", "print 0", "print 1"}
	if err != nil || result.Failure == nil || !result.Failure.HeaderReported || !reflect.DeepEqual(events, want) {
		t.Fatalf("result %+v, error %v, events %q", result, err, events)
	}
}

func TestCoverageInitialReplayCancellation(t *testing.T) {
	for _, tc := range []struct{ name, init, extra, config string }{
		{"invariant", `x = TRUE`, `Inv == x < 2`, "INVARIANT Inv"},
		{"constraint", `x \in 0..1`, `Limit == IF x = 0 THEN TRUE ELSE x < TRUE`, "CONSTRAINT Limit"},
	} {
		for _, deadline := range []bool{false, true} {
			t.Run(tc.name+map[bool]string{false: "-cancel", true: "-deadline"}[deadline], func(t *testing.T) {
				source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == " + tc.init + "\nA == UNCHANGED x\nB == UNCHANGED x\nNext == A \\/ B\n" + tc.extra + "\n===="
				ctx, cancel := context.WithCancel(context.Background())
				want := context.Canceled
				if deadline {
					cancel()
					ctx, cancel = context.WithTimeout(context.Background(), 250*time.Millisecond)
					want = context.DeadlineExceeded
				}
				defer cancel()
				result, err := CheckWithOptions(ctx, source, "INIT Init NEXT Next CHECK_DEADLOCK FALSE "+tc.config, Options{Coverage: true, OnFailureHeader: func(Failure) {
					if deadline {
						<-ctx.Done()
					} else {
						cancel()
					}
				}})
				if !errors.Is(err, want) || len(result.Coverage) != 0 {
					t.Fatalf("error %v, coverage %+v", err, result.Coverage)
				}
			})
		}
	}
}

func TestCoverageInitialReplayChangedOutcomeSuppressesReport(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nASSUME TLCSet(0, 0)\nVARIABLE x\nInit == x = 0\nA == UNCHANGED x\nB == UNCHANGED x\nNext == A \\/ B\nLimit == IF TLCGet(0) = 0 THEN TLCSet(0, 1) /\\ x < TRUE ELSE TRUE\n===="
	result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next CONSTRAINT Limit CHECK_DEADLOCK FALSE", Options{Coverage: true})
	if err == nil || !strings.Contains(err.Error(), "-coverage cannot report initial error replay") || len(result.Coverage) != 0 {
		t.Fatalf("result %+v, error %v", result, err)
	}
}

func TestCoverageInitialReplayMutableOperatorsThroughDefinitions(t *testing.T) {
	for _, tc := range []struct{ name, defs, condition string }{
		{"read-wrapper", `Read == TLCGet(0)`, `Read = 0 /\ x < TRUE`},
		{"write-wrapper", `Write == TLCSet(0, 1)`, `Write /\ x < TRUE`},
		{"operator-argument", `Write(F(_, _)) == F(0, 1)`, `Write(TLCSet) /\ x < TRUE`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nASSUME TLCSet(0, 0)\nVARIABLE x\nInit == x = 0\nA == UNCHANGED x\nB == UNCHANGED x\nNext == A \\/ B\n" + tc.defs + "\nLimit == " + tc.condition + "\n===="
			var progress LiveProgress
			result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next CONSTRAINT Limit CHECK_DEADLOCK FALSE", Options{Coverage: true, LiveProgress: &progress})
			if err == nil || !strings.Contains(err.Error(), "-coverage cannot report initial error replay") || len(result.Coverage) != 0 || progress.CoverageSnapshot() != nil {
				t.Fatalf("result %+v, error %v, progress %+v", result, err, progress.CoverageSnapshot())
			}
		})
	}
}

func TestCoverageRejectsNamedRuntimeReadsInConstraints(t *testing.T) {
	for _, constraint := range []string{
		`TLCGet("generated") = 1`,
		`Read = 1`,
		`Get("generated") = 1`,
		`Forward("generated") = 1`,
	} {
		source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x = 0\nA == UNCHANGED x\nB == UNCHANGED x\nNext == A \\/ B\nRead == TLCGet(\"generated\")\nGet(k) == TLCGet(k)\nForward(n) == Get(n)\nLimit == " + constraint + "\n===="
		config := "INIT Init NEXT Next CONSTRAINT Limit CHECK_DEADLOCK FALSE"
		_, err := CheckWithOptions(context.Background(), source, config, Options{Coverage: true})
		if err == nil {
			t.Fatalf("coverage accepted constraint %s", constraint)
		}
		if _, err := CheckWithOptions(context.Background(), source, config, Options{}); err != nil {
			t.Fatalf("ordinary exploration rejected constraint %s: %v", constraint, err)
		}
	}
}

func TestCoverageRejectsRecursiveNamedRuntimeRead(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x = 0\nA == UNCHANGED x\nB == UNCHANGED x\nNext == A \\/ B\nRECURSIVE Read(_, _)\nRead(k, n) == IF n = 0 THEN TLCGet(k) ELSE Read(\"generated\", n - 1)\nLimit == Read(0, x + 1) = 0\n===="
	result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next CONSTRAINT Limit CHECK_DEADLOCK FALSE", Options{Coverage: true})
	if err == nil {
		t.Fatalf("coverage accepted recursive named runtime read: %+v; failure %+v", result, result.Failure)
	}
}

func TestCoverageNamedConstraintReadFinalProgressAndEffects(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x = 0\nA == UNCHANGED x\nB == UNCHANGED x\nNext == A \\/ B\nLimit == IF PrintT(\"before\") THEN TLCGet(\"generated\") = 0 ELSE FALSE\n===="
	var prints []string
	var progress LiveProgress
	result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next CONSTRAINT Limit CHECK_DEADLOCK FALSE", Options{Coverage: true, LiveProgress: &progress, OnPrint: func(text string) { prints = append(prints, text) }})
	if !errors.Is(err, errCoverageNamedConstraintRead) || len(result.Coverage) != 0 || progress.CoverageSnapshot() != nil || !reflect.DeepEqual(prints, []string{`"before"`}) {
		t.Fatalf("error %v, coverage %+v, progress %+v, prints %q", err, result.Coverage, progress.CoverageSnapshot(), prints)
	}
}

func TestCoverageNamedConstraintReadDuringInitialReplay(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x \\in 0..1\nA == UNCHANGED x\nB == UNCHANGED x\nNext == A \\/ B\nLimit == PrintT(x) /\\ IF TLCGet(\"level\") >= 0 THEN IF x = 0 THEN TRUE ELSE x < TRUE ELSE FALSE\n===="
	var progress LiveProgress
	var prints []string
	var headers int
	result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next CONSTRAINT Limit CHECK_DEADLOCK FALSE", Options{Coverage: true, LiveProgress: &progress, OnPrint: func(text string) { prints = append(prints, text) }, OnFailureHeader: func(Failure) { headers++ }})
	if err == nil || !strings.Contains(err.Error(), "-coverage cannot report initial error replay") || len(result.Coverage) != 0 || progress.CoverageSnapshot() != nil || headers != 0 || !reflect.DeepEqual(prints, []string{"0", "1"}) {
		t.Fatalf("error %v, coverage %+v, progress %+v, headers %d, prints %q", err, result.Coverage, progress.CoverageSnapshot(), headers, prints)
	}
}

func BenchmarkCoverageInitialEvaluationError(b *testing.B) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x \\in 0..30\nA == UNCHANGED x\nB == UNCHANGED x\nNext == A \\/ B\nInv == x = 0 \\/ x < Nat\n===="
	for b.Loop() {
		_, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next INVARIANT Inv CHECK_DEADLOCK FALSE", Options{Coverage: true})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCoverageNumericConstraint(b *testing.B) {
	source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nASSUME TLCSet(0, 0)\nVARIABLE x\nInit == x \\in 0..100\nA == UNCHANGED x\nB == UNCHANGED x\nNext == A \\/ B\nLimit == TLCGet(0) = 0\n===="
	for b.Loop() {
		_, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next CONSTRAINT Limit CHECK_DEADLOCK FALSE", Options{Coverage: true})
		if err != nil {
			b.Fatal(err)
		}
	}
}
