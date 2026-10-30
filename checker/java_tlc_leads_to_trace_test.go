package checker

import (
	"fmt"
	"os"
	"testing"
)

// These state graphs exercise a trigger that is reachable before an SCC.
// Java TLC tracks the pending leads-to obligation separately from the state.
func TestJavaTLCLeadsToTrace(t *testing.T) {
	cases := []struct {
		name, next    string
		trigger, goal int
		trace         []int32
	}{
		{"branch", `\/ /\ x = 0 /\ x' \in {2,3}
        \/ /\ x = 1 /\ x' \in {2,3}
        \/ /\ x = 2 /\ x' \in {2,3}
        \/ /\ x = 3 /\ x' \in {0,2,3}`, 2, 1, []int32{0, 2, 3}},
		{"revisit", `\/ /\ x = 0 /\ x' \in {0,1}
        \/ /\ x = 1 /\ x' \in {1,2,3}
        \/ /\ x = 2 /\ x' \in {1,2,3}
        \/ /\ x = 3 /\ x' \in {0,1}`, 3, 0, []int32{0, 1, 3}},
		{"direct", `\/ /\ x = 0 /\ x' \in {0,3}
        \/ /\ x = 1 /\ x' \in {0,1,2}
        \/ /\ x = 2 /\ x' \in {3}
        \/ /\ x = 3 /\ x' \in {0,2,3}`, 3, 1, []int32{0, 3}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := fmt.Sprintf("---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == %s\nSpec == Init /\\ [][Next]_x\nP == (x = %d) ~> (x = %d)\n====\n", tc.next, tc.trigger, tc.goal)
			cfg := "SPECIFICATION Spec\nPROPERTY P\nCHECK_DEADLOCK FALSE\n"
			got, err := Check(source, cfg)
			if err != nil || got.Status != LivenessViolation {
				t.Fatalf("got %+v, %v", got, err)
			}
			checkEventuallyAlwaysTrace(t, got.Violation, tc.trace, len(tc.trace)-1, "Stuttering")
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaEventuallyAlways(t, jar, "M", source, cfg, tc.trace, len(tc.trace)-1, "Stuttering")
			}
		})
	}
}

func TestJavaTLCFairLeadsToTrace(t *testing.T) {
	for _, tc := range []struct {
		name, next, fair, closing string
		trigger, goal             int
		trace                     []int32
		loop                      int
	}{
		{"cycle", `\/ /\ x = 0 /\ x' \in {0,1}
        \/ /\ x = 1 /\ x' \in {0,2,3}
        \/ /\ x = 2 /\ x' \in {1,2}
        \/ /\ x = 3 /\ x' \in {3}`, "Next", "Next", 0, 3, []int32{0, 1, 0}, 1},
		{"pending", `\/ /\ x = 0 /\ x' \in {0,1}
        \/ /\ x = 1 /\ x' \in {0,2}
        \/ /\ x = 2 /\ x' \in {0,1,2}
        \/ /\ x = 3 /\ x' \in {0,1,3}`, "Next", "Next", 2, 0, []int32{0, 1, 2, 1}, 2},
		{"disabled", `\/ /\ x = 0 /\ x' \in {1}
        \/ /\ x = 1 /\ x' \in {1,2}
        \/ /\ x = 2 /\ x' \in {1,3}
        \/ /\ x = 3 /\ x' \in {2}`, `(x \in {2,3}) /\ Next`, "Stuttering", 0, 3, []int32{0, 1}, 1},
		{"disabled-initial", `\/ /\ x = 0 /\ x' \in {1,2}
        \/ /\ x = 1 /\ x' \in {3}
        \/ /\ x = 2 /\ x' \in {0,1,3}
        \/ /\ x = 3 /\ x' \in {0,1,2}`, `(x \in {2}) /\ Next`, "Stuttering", 0, 2, []int32{0}, 0},
		{"disabled-after-trigger", `\/ /\ x = 0 /\ x' \in {0,1}
        \/ /\ x = 1 /\ x' \in {0,1,2}
        \/ /\ x = 2 /\ x' \in {1,3}
        \/ /\ x = 3 /\ x' \in {0,1,3}`, `(x \in {0}) /\ Next`, "Stuttering", 3, 0, []int32{0, 1, 2, 3}, 3},
		{"disabled-normal-node", `\/ /\ x = 0 /\ x' \in {0,2,3}
        \/ /\ x = 1 /\ x' \in {1,2,3}
        \/ /\ x = 2 /\ x' \in {0,2,3}
        \/ /\ x = 3 /\ x' \in {0,3}`, `(x \in {1,2}) /\ Next`, "Next", 0, 1, []int32{0, 3, 0}, 1},
		{"disabled-late-node", `\/ /\ x = 0 /\ x' \in {0,1,2}
        \/ /\ x = 1 /\ x' \in {2,3}
        \/ /\ x = 2 /\ x' \in {0,1,2}
        \/ /\ x = 3 /\ x' \in {0,1,3}`, `(x \in {2}) /\ Next`, "Stuttering", 1, 2, []int32{0, 1}, 1},
		{"disabled-normal-cycle", `\/ /\ x = 0 /\ x' \in {0,1,3}
        \/ /\ x = 1 /\ x' \in {1,2,3}
        \/ /\ x = 2 /\ x' \in {0,2,3}
        \/ /\ x = 3 /\ x' \in {0,2,3}`, `(x \in {0,1}) /\ Next`, "Next", 1, 0, []int32{0, 1, 3, 2}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := fmt.Sprintf("---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == %s\nSpec == Init /\\ [][Next]_x /\\ WF_x(%s)\nP == (x = %d) ~> (x = %d)\n====\n", tc.next, tc.fair, tc.trigger, tc.goal)
			cfg := "SPECIFICATION Spec\nPROPERTY P\nCHECK_DEADLOCK FALSE\n"
			got, err := Check(source, cfg)
			if err != nil || got.Status != LivenessViolation {
				t.Fatalf("got %+v, %v", got, err)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaEventuallyAlways(t, jar, "M", source, cfg, tc.trace, tc.loop, tc.closing)
			}
			checkEventuallyAlwaysTrace(t, got.Violation, tc.trace, tc.loop, tc.closing)
		})
	}
}
