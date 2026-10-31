package checker

import (
	"context"
	"testing"
)

const dfidBenchmarkSpec = `---- MODULE DFIDBench ----
EXTENDS Naturals
VARIABLES a, b, c
Init == a = 0 /\ b = 0 /\ c = 0
Next == \/ a < 8 /\ a' = a + 1 /\ UNCHANGED <<b, c>>
        \/ b < 8 /\ b' = b + 1 /\ UNCHANGED <<a, c>>
        \/ c < 8 /\ c' = c + 1 /\ UNCHANGED <<a, b>>
        \/ a > 0 /\ b > 0 /\ a' = a - 1 /\ b' = b - 1 /\ UNCHANGED c
Inv == a + b + c >= 0
====`

// BenchmarkDepthFirst measures iterative deepening, which regenerates the
// states of every earlier level in each iteration. Like Java TLC, the check
// finds 729 distinct states and ends after starting level 26, as the state
// graph has diameter 25; it generates 33157 states.
func BenchmarkDepthFirst(b *testing.B) {
	options := Options{DepthFirst: &DepthFirstOptions{MaxDepth: 100}}
	for b.Loop() {
		result, err := CheckWithOptions(context.Background(), dfidBenchmarkSpec, "INIT Init\nNEXT Next\nINVARIANT Inv\nCHECK_DEADLOCK FALSE\n", options)
		if err != nil || result.Status != Success || result.DistinctStates != 729 || result.Depth != 26 || result.GeneratedStates != 33157 {
			b.Fatalf("result = %+v, %v", result, err)
		}
	}
}
