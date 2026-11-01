package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// Measure the complete CLI path, including incremental trace presentation.
func BenchmarkToolLongSimulationCycle(b *testing.B) {
	for _, size := range []int{1000, 10000} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			dir := b.TempDir()
			model := fmt.Sprintf("---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x=0\nNext == x'=(x+1)%% %d\nSpec == Init /\\ [][Next]_x /\\ WF_x(Next)\nP == <>(x=%d)\n====\n", size, size)
			for name, content := range map[string]string{"M.tla": model, "M.cfg": "SPECIFICATION Spec PROPERTY P CHECK_DEADLOCK FALSE\n"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
					b.Fatal(err)
				}
			}
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-simulate", "num=1", "-seed", "1", "-depth", fmt.Sprint(size + 20), filepath.Join(dir, "M.tla")}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if code := run(context.Background(), args, io.Discard, io.Discard); code != 13 {
					b.Fatalf("unexpected exit %d", code)
				}
			}
		})
	}
}
