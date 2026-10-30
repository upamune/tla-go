package checker

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Unchanged models and counts from tla-ts's tlc.test.ts.
func TestUpstreamTLCSpecificationDescriptions(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		generated, distinct, depth int
	}{
		{"Rich", 1121, 128, 9},
		{"Inst", 46, 9, 4},
		{"Subst", 5, 3, 3},
		{"Unnamed", 5, 3, 3},
		{"Param", 13, 4, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := "testdata/tlc-spec"
			source, err := os.ReadFile(filepath.Join(dir, tc.name+".tla"))
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := os.ReadFile(filepath.Join(dir, tc.name+".cfg"))
			if err != nil {
				t.Fatal(err)
			}
			loader := func(_ context.Context, name string) (string, error) {
				data, err := os.ReadFile(filepath.Join(dir, name+".tla"))
				return string(data), err
			}
			result, err := CheckWithLoader(context.Background(), string(source), string(cfg), loader)
			if err != nil || result.Status != Success || result.GeneratedStates != tc.generated || result.DistinctStates != tc.distinct || result.Depth != tc.depth {
				t.Fatalf("got %+v, %v", result, err)
			}
		})
	}
}

func TestUpstreamSpecificationDescriptionMatrix(t *testing.T) {
	f, err := os.Open("testdata/upstream-spec-description-matrix.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	count := 0
	for scanner.Scan() {
		var sample struct {
			Name, Source, Config, Expected, Status string
			Generated, Distinct, Depth             int
		}
		if err := json.Unmarshal(scanner.Bytes(), &sample); err != nil {
			t.Fatal(err)
		}
		count++
		t.Run(sample.Name, func(t *testing.T) {
			source := strings.Replace(sample.Source, `ASSUME PrintT(TLCGet("spec"))`, "ASSUME TLCGet(\"spec\") = "+sample.Expected, 1)
			result, err := Check(source, sample.Config)
			if err != nil {
				t.Fatal(err)
			}
			if string(result.Status) != sample.Status || result.GeneratedStates != sample.Generated || result.DistinctStates != sample.Distinct || result.Depth != sample.Depth {
				t.Fatalf("got %+v violation=%+v, want status=%s generated=%d distinct=%d depth=%d", result, result.Violation, sample.Status, sample.Generated, sample.Distinct, sample.Depth)
			}
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 132 {
		t.Fatalf("got %d cases, want 132", count)
	}
}

func BenchmarkTLCSpecificationDescription(b *testing.B) {
	const dir = "testdata/tlc-spec"
	source, err := os.ReadFile(filepath.Join(dir, "Rich.tla"))
	if err != nil {
		b.Fatal(err)
	}
	cfg, err := os.ReadFile(filepath.Join(dir, "Rich.cfg"))
	if err != nil {
		b.Fatal(err)
	}
	loader := func(_ context.Context, name string) (string, error) {
		data, err := os.ReadFile(filepath.Join(dir, name+".tla"))
		return string(data), err
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		result, err := CheckWithLoader(context.Background(), string(source), string(cfg), loader)
		if err != nil || result.Status != Success || result.DistinctStates != 128 {
			b.Fatalf("got %+v, %v", result, err)
		}
	}
}
