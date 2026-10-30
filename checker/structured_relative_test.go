package checker

import (
	"context"
	"testing"
)

func TestRelativeProofSteps(t *testing.T) {
	for _, tc := range []struct {
		name, proof string
		valid       bool
	}{
		{"root plus", "<+>* TRUE\n<*>- TRUE\n<*> QED", true},
		{"root star", "<*>* TRUE\n<*> QED", true},
		{"implicit step reference", "<1>1 TRUE\n<1>2 TRUE BY <*>1\n<1> QED", true},
		{"nested plus", "<1>1 TRUE\n <+>* TRUE\n <*> QED\n<1> QED", true},
		{"nested explicit star", "<1>1 TRUE\n PROOF <*>* TRUE\n <*> QED\n<1> QED", true},
		{"implicit siblings", "<1>1 TRUE\n <*>* TRUE\n <*> QED", true},
		{"zero level", "<0>1 TRUE\n<0> QED", true},
		{"unnumbered star", "<1>* TRUE\n<1> QED", true},
		{"unnumbered dash", "<1>- TRUE\n<1> QED", true},
		{"leading zero definition", "<01>1 TRUE\n<1>2 TRUE BY <1>1\n<1> QED", true},
		{"leading zero reference", "<01>1 TRUE\n<1>2 TRUE BY <01>1\n<1> QED", false},
		{"named relative", "<1>1 TRUE\n<*>2 TRUE\n<1> QED", false},
		{"automatic step reference", "<1>* TRUE\n<1>2 TRUE BY <1>*\n<1> QED", false},
		{"plus step reference", "<1>1 TRUE\n<1>2 TRUE BY <+>1\n<1> QED", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CheckContext(context.Background(), "---- MODULE M ----\nTHEOREM T == TRUE\n"+tc.proof+"\n====\n", "")
			if (err == nil) != tc.valid {
				t.Fatalf("valid %t: %v", tc.valid, err)
			}
		})
	}
}
