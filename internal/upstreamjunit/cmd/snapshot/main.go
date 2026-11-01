// Command snapshot extracts the specific completed diagnostic sweep into JSON.
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
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	if hash != "4769aef155298934c93845b6a0130091bbd0f1060d03ebd94762068acdcb755a" {
		fail(fmt.Errorf("unexpected source log SHA-256: %s", hash))
	}
	snapshot, err := upstreamjunit.ParseSweepLog(bytes.NewReader(data))
	if err != nil {
		fail(err)
	}
	snapshot.Schema = 1
	snapshot.TlaGoCommit = "2399b1535d1ef68aa78d81bba2b9559cf8e586f7"
	snapshot.UpstreamCommit = upstreamjunit.UpstreamCommit
	snapshot.JarSHA256 = upstreamjunit.JarSHA256
	snapshot.SourceSHA256 = hash
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(snapshot); err != nil {
		fail(err)
	}
}
func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
