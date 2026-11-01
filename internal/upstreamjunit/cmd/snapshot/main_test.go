package main

import "testing"

func TestCommitForSourceHash(t *testing.T) {
	for _, tc := range []struct{ hash, commit string }{
		{"4769aef155298934c93845b6a0130091bbd0f1060d03ebd94762068acdcb755a", "2399b1535d1ef68aa78d81bba2b9559cf8e586f7"},
		{"fa1bf74a467a45cbd1d029017dbafb7ae0305221d5f1e5e91201fc7a6cc29ec5", "56336d54f659dbb744aff66ffa8e653a6117c06a"},
	} {
		got, err := commitForSourceHash(tc.hash)
		if err != nil || got != tc.commit {
			t.Fatalf("hash %s: got %q, %v", tc.hash, got, err)
		}
	}
	for _, hash := range []string{"", "fa1bf74a467a45cbd1d029017dbafb7ae0305221d5f1e5e91201fc7a6cc29ec4", "56336d54f659dbb744aff66ffa8e653a6117c06a"} {
		if commit, err := commitForSourceHash(hash); err == nil || commit != "" {
			t.Fatalf("accepted unknown hash %q as %q", hash, commit)
		}
	}
}

func TestSnapshotFromLogRejectsUnrecognizedInput(t *testing.T) {
	// Even a structurally complete sweep cannot be assigned a known revision.
	log := []byte("    integration_test.go:123: incomplete sweep: 0 candidates with recovered spec, 0 diagnostic runs, 0 candidates with unsafe or unresolved setup, 0 unresolved non-abstract candidates; inventory is not an exhaustive JUnit count\n--- FAIL: TestUpstreamJUnitSweep (1.00s)\n")
	if snap, err := snapshotFromLog(log); err == nil || snap.TlaGoCommit != "" {
		t.Fatalf("accepted unrecognized log: %+v, %v", snap, err)
	}
}
