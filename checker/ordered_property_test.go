package checker

import (
	"reflect"
	"testing"
)

// Expected statuses and counts were captured from tla-ts at the pinned commit.
func TestOrderedEventuallyRemembersWhetherItWasAlreadySatisfied(t *testing.T) {
	for _, tc := range []struct {
		name, next, trigger, goal string
		fair                      bool
		status                    Status
		generated, distinct       int
	}{
		{"satisfied before a later trigger", "x' = IF x < 3 THEN x + 1 ELSE x", "(x = 1 \\/ x = 3)", "x = 2", true, Success, 5, 4},
		{"no trigger", "x' = x", "x = 1", "x = 2", false, LivenessViolation, 2, 1},
		{"no goal", "x' = IF x = 0 THEN 1 ELSE x", "x = 1", "x = 2", true, LivenessViolation, 3, 2},
		{"goal wraps around the loop", "x' = 1 - x", "x = 1", "x = 0", true, Success, 3, 2},
		{"trigger and goal together", "x' = x", "x = 0", "x = 0", false, Success, 2, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fair := ""
			if tc.fair {
				fair = " /\\ WF_x(Next)"
			}
			source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == " + tc.next + "\nSpec == Init /\\ [][Next]_x" + fair + "\nP == <>(" + tc.trigger + " /\\ <>(" + tc.goal + "))\n===="
			result, err := Check(source, "SPECIFICATION Spec PROPERTY P")
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != tc.status || result.GeneratedStates != tc.generated || result.DistinctStates != tc.distinct {
				t.Fatalf("got %+v", result)
			}
		})
	}
}

func TestOrderedEventuallyInMultiplePropertyReport(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 3\nSpec == Init /\\ [][Next]_x /\\ WF_x(Next)\nMissing == <>(x = 9)\nSatisfied == <>(x = 2 /\\ <>(x = 1))\nAbsent == <>(x = 9 /\\ <>(x = 1))\nGuard == [](x = 2 => [](x /= 0))\n===="
	result, err := Check(source, "SPECIFICATION Spec PROPERTIES Missing Satisfied Absent Guard")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Missing", "Absent", "Guard"}
	if result.Status != LivenessViolation || result.Violation == nil || !reflect.DeepEqual(result.Violation.Properties, want) {
		t.Fatalf("got %+v violation=%+v", result, result.Violation)
	}
}
