package checker

import (
	"fmt"
	"os"
	"testing"
)

// A leads-to obligation is satisfied when its consequent holds at the trigger,
// even if the consequent becomes false later.
func TestJavaTLCLeadsToImmediate(t *testing.T) {
	for _, tc := range []struct {
		name   string
		goal   int
		status Status
	}{
		{"satisfied", 0, Success},
		{"violated", 2, LivenessViolation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := fmt.Sprintf(`---- MODULE M ----
VARIABLE x
Init == x = 0
Next == x' = 1
Spec == Init /\ [][Next]_x /\ WF_x(Next)
P == (x = 0) ~> (x = %d)
====
`, tc.goal)
			cfg := "SPECIFICATION Spec\nPROPERTY P\nCHECK_DEADLOCK FALSE\n"
			got, err := Check(source, cfg)
			if err != nil || got.Status != tc.status || got.GeneratedStates != 3 || got.DistinctStates != 2 {
				t.Fatalf("got %+v, %v", got, err)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				if tc.status == Success {
					compareJavaTLCModel(t, jar, "M", []byte(source), []byte(cfg), 3, 2, 2)
				} else {
					compareJavaTLCStutteringTrace(t, jar, "M", []byte(source), []byte(cfg), []string{"0", "1"}, 3, 2)
				}
			}
		})
	}
}

func TestJavaTLCLeadsToImmediateWithoutFairness(t *testing.T) {
	const source = `---- MODULE M ----
VARIABLE x
Init == x = 0
Next == x' = 1
P == (x = 0) ~> (x = 0)
====
`
	const cfg = "INIT Init\nNEXT Next\nPROPERTY P\nCHECK_DEADLOCK FALSE\n"
	got, err := Check(source, cfg)
	if err != nil || got.Status != Success || got.GeneratedStates != 3 || got.DistinctStates != 2 {
		t.Fatalf("got %+v, %v", got, err)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareJavaTLCModel(t, jar, "M", []byte(source), []byte(cfg), 3, 2, 2)
	}
}
