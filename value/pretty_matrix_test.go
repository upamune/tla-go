package value

import (
	"bufio"
	"encoding/json"
	"os"
	"testing"
)

func TestUpstreamPrettyMatrix(t *testing.T) {
	f, err := os.Open("testdata/upstream-pretty-matrix.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	count := 0
	for scanner.Scan() {
		var sample struct{ Input, Want string }
		if err := json.Unmarshal(scanner.Bytes(), &sample); err != nil {
			t.Fatal(err)
		}
		count++
		if got := PrettyPrint(sample.Input); got != sample.Want {
			t.Errorf("case %d: got %q, want %q; input %q", count, got, sample.Want, sample.Input)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 300 {
		t.Fatalf("got %d cases, want 300", count)
	}
}

func BenchmarkPrettyPrint(b *testing.B) {
	text := "<<" + "<<1, 2, 3, 4, 5, 6, 7, 8, 9, 10>>, " + "<<1, 2, 3, 4, 5, 6, 7, 8, 9, 10>>, " + "<<1, 2, 3, 4, 5, 6, 7, 8, 9, 10>>" + ">>"
	b.ReportAllocs()
	for b.Loop() {
		PrettyPrint(text)
	}
}
