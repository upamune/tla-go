package checker

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestUpstreamTemporalLetMatrix(t *testing.T) {
	f, err := os.Open("testdata/upstream-let-ltl-matrix.jsonl")
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
				if strings.HasPrefix(tc.Error, "Multiply-defined symbol") && !strings.Contains(err.Error(), tc.Error) {
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
	if count != 2400 {
		t.Fatalf("got %d cases, want 2400", count)
	}
}

func TestTemporalLetOperatorArguments(t *testing.T) {
	formulas := []string{
		"LET F(G(_), y) == []<>(G(y)) IN F(Q,0) \\/ F(Q,1)",
		"LET F(y) == <>[](x=y) IN \\A z \\in {0,1} : []<>F(z)",
		"LET A == x=0 IN []<>(LET B == <>[]A IN B)",
	}
	for _, formula := range formulas {
		source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x=0\nNext == x'=(x+1)%3\nQ(y) == x=y\nP == " + formula + "\n===="
		got, err := Check(source, "INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE")
		if err != nil || got.Status != LivenessViolation || got.GeneratedStates != 4 || got.DistinctStates != 3 || got.Depth != 3 {
			t.Fatalf("%s: got %+v, %v; want upstream liveness violation 4 3 3", formula, got, err)
		}
	}
}

func TestTemporalLetRejectsNonrecursiveSelfReference(t *testing.T) {
	source := "---- MODULE M ----\nVARIABLE x\nInit == x=0\nNext == x'=x\nP == LET A == A IN []<>[]A\nA == x=0\n===="
	_, err := Check(source, "INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE")
	if err == nil {
		t.Fatal("expected an error for a non-recursive local operator referring to itself")
	}
}
