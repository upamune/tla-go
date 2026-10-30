package checker

import (
	"strings"
	"testing"
)

func TestRecursiveOrderedEventually(t *testing.T) {
	source := `---- MODULE M ----
EXTENDS Naturals
VARIABLE x
Init == x = 0
Next == x' = (x + 1) % 3
RECURSIVE Visit(_)
Visit(n) == IF n = 0 THEN <>(x = 2) ELSE <>(x = n - 1 /\ Visit(n - 1))
P == \A i \in 1..2 : Visit(i)
====`
	result, err := Check(source, "INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != LivenessViolation {
		t.Fatalf("expected stuttering counterexample, got %s", result.Status)
	}
}

func TestRejectUnboundedTemporalRecursion(t *testing.T) {
	for _, recursiveCall := range []string{"Visit(n)", "Visit(n - 1)"} {
		source := `---- MODULE M ----
EXTENDS Integers
VARIABLE x
Init == x = 0
Next == x' = x
RECURSIVE Visit(_)
Visit(n) == <>(x = n /\ ` + recursiveCall + `)
P == Visit(1)
====`
		_, err := Check(source, "INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE")
		if err == nil || !strings.Contains(err.Error(), "unsupported") {
			t.Fatalf("%s: expected unsupported expression, got %v", recursiveCall, err)
		}
	}
}
