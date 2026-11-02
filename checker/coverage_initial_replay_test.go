package checker

import (
	"context"
	"reflect"
	"testing"
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

func TestCoverageInitialReplayChangedOutcomeSuppressesReport(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nASSUME TLCSet(0, 0)\nVARIABLE x\nInit == x = 0\nA == UNCHANGED x\nB == UNCHANGED x\nNext == A \\/ B\nLimit == IF TLCGet(0) = 0 THEN TLCSet(0, 1) /\\ x < TRUE ELSE TRUE\n===="
	result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next CONSTRAINT Limit CHECK_DEADLOCK FALSE", Options{Coverage: true})
	if err != nil || result.Status != EvaluationFailure || len(result.Coverage) != 0 {
		t.Fatalf("result %+v, error %v", result, err)
	}
}

func TestCoverageRejectsNamedRuntimeReadsInConstraints(t *testing.T) {
	for _, constraint := range []string{
		`TLCGet("generated") = 1`,
		`Read = 1`,
		`Get("generated") = 1`,
	} {
		source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x = 0\nA == UNCHANGED x\nB == UNCHANGED x\nNext == A \\/ B\nRead == TLCGet(\"generated\")\nGet(k) == TLCGet(k)\nLimit == " + constraint + "\n===="
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

func BenchmarkCoverageInitialEvaluationError(b *testing.B) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x \\in 0..30\nA == UNCHANGED x\nB == UNCHANGED x\nNext == A \\/ B\nInv == x = 0 \\/ x < Nat\n===="
	for b.Loop() {
		_, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next INVARIANT Inv CHECK_DEADLOCK FALSE", Options{Coverage: true})
		if err != nil {
			b.Fatal(err)
		}
	}
}
