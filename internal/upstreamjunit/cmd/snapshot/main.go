// Command snapshot extracts a known completed diagnostic sweep into JSON.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"

	"github.com/upamune/tla-go/internal/upstreamjunit"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./internal/upstreamjunit/cmd/snapshot LOG")
		os.Exit(2)
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fail(err)
	}
	snapshot, err := snapshotFromLog(data)
	if err != nil {
		fail(err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(snapshot); err != nil {
		fail(err)
	}
}
func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }

func snapshotFromLog(data []byte) (upstreamjunit.SweepSnapshot, error) {
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	commit, err := commitForSourceHash(hash)
	if err != nil {
		return upstreamjunit.SweepSnapshot{}, err
	}
	snapshot, err := upstreamjunit.ParseSweepLog(bytes.NewReader(data))
	if err != nil {
		return upstreamjunit.SweepSnapshot{}, err
	}
	snapshot.Schema = 1
	snapshot.TlaGoCommit = commit
	snapshot.UpstreamCommit = upstreamjunit.UpstreamCommit
	snapshot.JarSHA256 = upstreamjunit.JarSHA256
	snapshot.SourceSHA256 = hash
	return snapshot, nil
}

// The log does not identify the executable revision itself. Only hashes whose
// binary provenance was independently recorded can be assigned a revision.
func commitForSourceHash(hash string) (string, error) {
	switch hash {
	case "4769aef155298934c93845b6a0130091bbd0f1060d03ebd94762068acdcb755a":
		return "2399b1535d1ef68aa78d81bba2b9559cf8e586f7", nil
	case "fa1bf74a467a45cbd1d029017dbafb7ae0305221d5f1e5e91201fc7a6cc29ec5":
		return "56336d54f659dbb744aff66ffa8e653a6117c06a", nil
	case "65e957a50c1a58779f5982458ca42adb3d4ed16e0884229dd7bbe752aa298765":
		return "e81f221f7ba59b833244e7613bd149f836532c86", nil
	case "e548e0fba387f59c1278069d2bb0bcd7196475e36a6049003336d2616bb13000":
		return "9f5816269083eb7c9d44e5477695e95e6bf68059", nil
	case "606f46f5a51d30125f9b980a8eafccc03c40b219faff0194933eca28ac135598":
		return "51c3a77c6c4cbc02c6f388b0c90047cc895f9869", nil
	default:
		return "", fmt.Errorf("unexpected source log SHA-256: %s", hash)
	}
}
