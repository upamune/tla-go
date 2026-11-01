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
	default:
		return "", fmt.Errorf("unexpected source log SHA-256: %s", hash)
	}
}
