package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestCommitForSourceHash(t *testing.T) {
	for _, tc := range []struct{ hash, commit string }{
		{"4769aef155298934c93845b6a0130091bbd0f1060d03ebd94762068acdcb755a", "2399b1535d1ef68aa78d81bba2b9559cf8e586f7"},
		{"fa1bf74a467a45cbd1d029017dbafb7ae0305221d5f1e5e91201fc7a6cc29ec5", "56336d54f659dbb744aff66ffa8e653a6117c06a"},
		{"65e957a50c1a58779f5982458ca42adb3d4ed16e0884229dd7bbe752aa298765", "e81f221f7ba59b833244e7613bd149f836532c86"},
		{"e548e0fba387f59c1278069d2bb0bcd7196475e36a6049003336d2616bb13000", "9f5816269083eb7c9d44e5477695e95e6bf68059"},
		{"606f46f5a51d30125f9b980a8eafccc03c40b219faff0194933eca28ac135598", "51c3a77c6c4cbc02c6f388b0c90047cc895f9869"},
		{"c4f95b37f08cb2be7b59a887333a14f0a3ec14af2f95f2525b698cb875bf01fa", "aadcf4ba4d9e07bd03c6d06c6086da1bfe123c29"},
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

func TestRegenerateE81Snapshot(t *testing.T) {
	checkRegeneration(t, "TLA_GO_E81_SWEEP_LOG", "/tmp/tla-e81-diagnostic-sweep.log", "sweep-e81f221f.json")
}

func TestRegenerate9F58162Snapshot(t *testing.T) {
	checkRegeneration(t, "TLA_GO_9F58162_SWEEP_LOG", "/tmp/tla-9f58162-diagnostic-sweep.log", "sweep-9f581626.json")
}

func TestRegenerate51C3A77Snapshot(t *testing.T) {
	checkRegeneration(t, "TLA_GO_51C3A77_SWEEP_LOG", "/tmp/tla-51c3a77-sweep.log", "sweep-51c3a77c.json")
}

func TestRegenerateAADCF4BSnapshot(t *testing.T) {
	checkRegeneration(t, "TLA_GO_AADCF4B_SWEEP_LOG", "/tmp/tla-aadcf4b-sweep.log", "sweep-aadcf4ba.json")
}

func checkRegeneration(t *testing.T, env, defaultPath, snapshot string) {
	t.Helper()
	path := os.Getenv(env)
	if path == "" {
		path = defaultPath
	}
	log, err := os.ReadFile(path)
	if os.IsNotExist(err) && os.Getenv(env) == "" {
		t.Skipf("ephemeral source log unavailable; set %s to verify regeneration", env)
	}
	if err != nil {
		t.Fatal(err)
	}
	snap, err := snapshotFromLog(log)
	if err != nil {
		t.Fatal(err)
	}
	var got bytes.Buffer
	enc := json.NewEncoder(&got)
	enc.SetIndent("", "  ")
	if err := enc.Encode(snap); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../testdata/snapshots/" + snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), want) {
		t.Fatal("regenerated snapshot differs from checked-in bytes")
	}
	if _, err := snapshotFromLog(append(log, '\n')); err == nil {
		t.Fatal("accepted modified source log")
	}
}

func TestSnapshotFromLogRejectsUnrecognizedInput(t *testing.T) {
	// Even a structurally complete sweep cannot be assigned a known revision.
	log := []byte("    integration_test.go:123: incomplete sweep: 0 candidates with recovered spec, 0 diagnostic runs, 0 candidates with unsafe or unresolved setup, 0 unresolved non-abstract candidates; inventory is not an exhaustive JUnit count\n--- FAIL: TestUpstreamJUnitSweep (1.00s)\n")
	if snap, err := snapshotFromLog(log); err == nil || snap.TlaGoCommit != "" {
		t.Fatalf("accepted unrecognized log: %+v, %v", snap, err)
	}
}
