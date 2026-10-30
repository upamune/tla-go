package checker

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestUpstreamFairnessPropertyMatrix(t *testing.T) {
	f, err := os.Open("testdata/upstream-fair-property-matrix.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		var tc struct {
			Name, Source, Config, Status, Error string
			Generated, Distinct, Depth          int
		}
		if err := json.Unmarshal(scanner.Bytes(), &tc); err != nil {
			t.Fatal(err)
		}
		count++
		t.Run(tc.Name, func(t *testing.T) {
			got, err := Check(tc.Source, tc.Config)
			if tc.Error != "" {
				if err == nil {
					t.Fatalf("got %+v; want upstream error %q", got, tc.Error)
				}
				if strings.HasPrefix(tc.Error, "TLC cannot handle the temporal formula:") && !strings.Contains(err.Error(), tc.Error) {
					t.Fatalf("got error %q; want %q", err, tc.Error)
				}
				return
			}
			if err != nil || string(got.Status) != tc.Status || got.GeneratedStates != tc.Generated || got.DistinctStates != tc.Distinct || got.Depth != tc.Depth {
				t.Fatalf("got %+v, %v; want %s %d %d %d", got, err, tc.Status, tc.Generated, tc.Distinct, tc.Depth)
			}
			if got.Status == LivenessViolation && (got.Violation == nil || got.Violation.LoopStart == nil || *got.Violation.LoopStart >= len(got.Violation.Trace)) {
				t.Fatalf("invalid lasso %+v", got.Violation)
			}
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 3072 {
		t.Fatalf("got %d cases, want 3072", count)
	}
}

func BenchmarkFairnessProperty(b *testing.B) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 40\nA == x' = (x+1)%40\nP == WF_x(A) \\/ []<>(x=20)\n===="
	for b.Loop() {
		result, err := Check(source, "INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE")
		if err != nil || result.Status != LivenessViolation {
			b.Fatalf("got %+v, %v", result, err)
		}
	}
}
