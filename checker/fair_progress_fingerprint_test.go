package checker

import "testing"

func TestFairProgressFingerprintSafety(t *testing.T) {
	for _, tc := range []struct {
		name         string
		fingerprints map[int]uint64
		want         bool
	}{
		{"distinct", map[int]uint64{0: 10, 1: 11}, true},
		{"collision", map[int]uint64{0: 10, 1: 10}, false},
		{"missing", map[int]uint64{0: 10}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := search{nodes: make([]stateNode, 2), ltlFingerprints: tc.fingerprints}
			if got := s.uniqueIncrementalFingerprints(); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
