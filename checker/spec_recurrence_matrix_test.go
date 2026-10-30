package checker

import (
	"bufio"
	"encoding/json"
	"os"
	"testing"
)

func TestUpstreamSpecificationRecurrenceMatrix(t *testing.T) {
	file, err := os.Open("testdata/upstream-spec-recurrence-matrix.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		var sample struct{ Name, Source, Config, Status string }
		if err := json.Unmarshal(scanner.Bytes(), &sample); err != nil {
			t.Fatal(err)
		}
		count++
		t.Run(sample.Name, func(t *testing.T) {
			result, err := Check(sample.Source, sample.Config)
			if err != nil {
				t.Fatal(err)
			}
			if string(result.Status) != sample.Status {
				t.Fatalf("got %s, want %s", result.Status, sample.Status)
			}
			if result.Status == LivenessViolation {
				checkLasso(t, sample.Source, sample.Config, result.Violation)
			}
			if result.Status == PropertyViolation {
				checkGuardedTrace(t, sample.Source, sample.Config, result.Violation)
			}
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 400 {
		t.Fatalf("expected 400 cases, got %d", count)
	}
}
