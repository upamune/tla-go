package checker

import (
	"reflect"
	"testing"
)

// The expected results were captured from tla-ts at the pinned commit.
func TestGuardedSafetyAcrossMergedStates(t *testing.T) {
	for _, tc := range []struct {
		name, next, properties, definitions string
		generated, distinct, queued, depth  int
		trace, violated                     []string
	}{
		{
			name:       "merge before expansion",
			next:       "x' \\in CASE x = 0 -> {1, 2} [] x = 1 -> {3} [] x = 2 -> {3} [] OTHER -> {3}",
			properties: "P", definitions: "P == [](x = 2 => [](x /= 3))",
			generated: 6, distinct: 4, queued: 0, depth: 3,
			trace: []string{"0", "2", "3"}, violated: []string{"P"},
		},
		{
			name:       "propagate into an already expanded state",
			next:       "x' \\in IF x = 0 THEN {1, 2} ELSE {1}",
			properties: "P Q R", definitions: "P == [](x = 2 => [](x /= 1))\nQ == \\A i \\in {2, 3} : [](x = i => [](x /= 1))\nR == [](x = 3 => [](x /= 1))",
			generated: 5, distinct: 3, queued: 0, depth: 2,
			trace: []string{"0", "2", "1"}, violated: []string{"P", "Q"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == " + tc.next + "\nSpec == Init /\\ [][Next]_x\n" + tc.definitions + "\n===="
			result, err := Check(source, "SPECIFICATION Spec PROPERTIES "+tc.properties)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != PropertyViolation || result.Violation == nil {
				t.Fatalf("got %+v", result)
			}
			var trace []string
			for _, step := range result.Violation.Trace {
				trace = append(trace, step.State["x"].String())
			}
			if !reflect.DeepEqual(trace, tc.trace) || !reflect.DeepEqual(result.Violation.Properties, tc.violated) || result.GeneratedStates != tc.generated || result.DistinctStates != tc.distinct || result.QueuedStates != tc.queued || result.Depth != tc.depth {
				t.Fatalf("got %+v violation=%+v trace=%v", result, result.Violation, trace)
			}
		})
	}
}

func TestGuardedSafetyRespectsDeadlockSetting(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x < 1 /\\ x' = x + 1\nSpec == Init /\\ [][Next]_x\nP == [](x > 0 => [](x > 1))\n===="
	for _, tc := range []struct {
		config string
		status Status
	}{
		{"SPECIFICATION Spec PROPERTY P", Deadlock},
		{"SPECIFICATION Spec PROPERTY P CHECK_DEADLOCK FALSE", PropertyViolation},
	} {
		result, err := Check(source, tc.config)
		if err != nil {
			t.Fatal(err)
		}
		if result.Status != tc.status {
			t.Fatalf("got %+v", result)
		}
	}
}

func TestGuardedSafetyRequiresAFairContinuation(t *testing.T) {
	for _, tc := range []struct {
		name, fairness, fairAction, properties string
		status                                 Status
		want                                   []string
	}{
		{"unfair weak", "WF", "2", "P Reach", Success, nil},
		{"unfair strong", "SF", "2", "P Reach", Success, nil},
		{"fair guard first", "WF", "1", "P Reach", LivenessViolation, []string{"P", "Reach"}},
		{"fair reach first", "WF", "1", "Reach P", LivenessViolation, []string{"Reach", "P"}},
		{"fair strong", "SF", "1", "P Reach", LivenessViolation, []string{"P", "Reach"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == x' = 1\nA == x' = " + tc.fairAction + "\nSpec == Init /\\ [][Next]_x /\\ " + tc.fairness + "_x(A)\nP == [](x = 1 => [](x = 0))\nReach == <>(x = 9)\n===="
			result, err := Check(source, "SPECIFICATION Spec PROPERTIES "+tc.properties+" CHECK_DEADLOCK FALSE")
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != tc.status {
				t.Fatalf("got %+v", result)
			}
			if result.GeneratedStates != 3 || result.DistinctStates != 2 {
				t.Fatalf("got %+v", result)
			}
			if tc.want != nil && (result.Violation == nil || !reflect.DeepEqual(result.Violation.Properties, tc.want) || result.Violation.LoopStart == nil) {
				t.Fatalf("got %+v violation=%+v", result, result.Violation)
			}
		})
	}
}
