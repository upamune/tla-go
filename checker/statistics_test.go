package checker

import "testing"

// Expected counts are from checkSequentially at the pinned tla-ts commit.
func TestSearchQueueAndOutDegrees(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x \\in {0, 1}\nNext == x' \\in {x, (x + 2) % 4}\nSmall == x < 2\nInv == x < 2\n===="
	for _, tc := range []struct {
		config                      string
		status                      Status
		generated, distinct, queued int
		degrees                     bool
	}{
		{"INIT Init NEXT Next", Success, 10, 4, 0, true},
		{"INIT Init NEXT Next INVARIANT Inv", InvariantViolation, 4, 3, 1, false},
		{"INIT Init NEXT Next INVARIANT Inv CONSTRAINT Small", InvariantViolation, 4, 2, 1, false},
	} {
		t.Run(tc.config, func(t *testing.T) {
			result, err := Check(source, tc.config)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != tc.status || result.GeneratedStates != tc.generated || result.DistinctStates != tc.distinct || result.QueuedStates != tc.queued || result.HasOutDegrees != tc.degrees {
				t.Fatalf("got %+v", result)
			}
			if tc.degrees && (result.OutDegrees[0] != 2 || result.OutDegrees[1] != 2) {
				t.Fatalf("outdegrees: %v", result.OutDegrees)
			}
		})
	}
}
