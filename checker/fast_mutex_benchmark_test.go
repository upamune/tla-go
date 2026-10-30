package checker

import (
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkUpstreamFastMutex(b *testing.B) {
	dir := filepath.Join("..", "cmd", "tla-go", "testdata", "java-tlc", "fast-mutex")
	source, err := os.ReadFile(filepath.Join(dir, "FastMutexWithGoto2.tla"))
	if err != nil {
		b.Fatal(err)
	}
	config, err := os.ReadFile(filepath.Join(dir, "FastMutexWithGoto2.cfg"))
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for b.Loop() {
		result, err := Check(string(source), string(config))
		if err != nil || result.Status != Success || result.GeneratedStates != 42277 || result.DistinctStates != 15900 {
			b.Fatalf("unexpected result: %+v, %v", result, err)
		}
	}
}
