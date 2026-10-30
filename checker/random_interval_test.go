package checker

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestUpstreamRandomSubsetLargeIntervals(t *testing.T) {
	file, err := os.Open("testdata/upstream-large-interval-random.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		var tc struct {
			Name, Source, Config, Seed, Status string
			Generated, Distinct, Depth         int
			Printed                            []string
		}
		if err := json.Unmarshal(scanner.Bytes(), &tc); err != nil {
			t.Fatal(err)
		}
		// tla-ts accepts oversized intervals that Java TLC rejects. Retain its
		// fixture for provenance, but only compare the valid-size cases here.
		if !strings.HasPrefix(tc.Name, "range-0..2147483646-") {
			continue
		}
		count++
		t.Run(tc.Name, func(t *testing.T) {
			seed, err := strconv.ParseInt(tc.Seed, 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			var printed []string
			got, err := CheckWithOptions(context.Background(), tc.Source, tc.Config, Options{Seed: seed, ModernRandomSubsets: true, OnPrint: func(line string) { printed = append(printed, line) }})
			if err != nil || string(got.Status) != tc.Status || got.GeneratedStates != tc.Generated || got.DistinctStates != tc.Distinct || got.Depth != tc.Depth || !reflect.DeepEqual(printed, tc.Printed) {
				t.Fatalf("got %+v, %v, output %q; want %s %d %d %d output %q", got, err, printed, tc.Status, tc.Generated, tc.Distinct, tc.Depth, tc.Printed)
			}
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 12 {
		t.Fatalf("got %d cases; want 12", count)
	}
}
