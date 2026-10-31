package checker

import (
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkUpstreamDetlefs(b *testing.B) {
	dir := filepath.Join("testdata", "detlefs")
	source, err := os.ReadFile(filepath.Join(dir, "Detlefs.tla"))
	if err != nil {
		b.Fatal(err)
	}
	config, err := os.ReadFile(filepath.Join(dir, "Detlefs.cfg"))
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for b.Loop() {
		result, err := Check(string(source), string(config))
		if err != nil || result.Status != Success || result.GeneratedStates != 23821 || result.DistinctStates != 10149 || result.Depth != 42 {
			b.Fatalf("unexpected result: %+v, %v", result, err)
		}
	}
}
