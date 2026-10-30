package checker

import (
	"bufio"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestUpstreamFairnessBooleanSimplification(t *testing.T) {
	for _, tc := range []struct {
		formula string
		message string
	}{
		{"WF_x(x=0) \\/ TRUE", "Temporal formula is a tautology"},
		{"IF TRUE THEN TRUE ELSE WF_x(x=0)", "Temporal formula is a tautology"},
		{"WF_x(x=0) /\\ FALSE", "The spec is trivially false"},
		{"FALSE /\\ WF_x(x=0)", "The spec is trivially false"},
		{"WF_x(x=0) /\\ (x=1 /\\ FALSE)", "The spec is trivially false"},
	} {
		source := "---- MODULE M ----\nVARIABLE x\nInit==x=0\nNext==x'=x\nP==" + tc.formula + "\n===="
		_, err := Check(source, "INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE")
		if err == nil || !strings.Contains(err.Error(), tc.message) {
			t.Fatalf("%s: got %v; want error containing %q", tc.formula, err, tc.message)
		}
	}
}

func TestUpstreamFairnessFailureMatrix(t *testing.T) {
	checkUpstreamLivenessFailures(t, "testdata/upstream-fair-failure-matrix.jsonl", 1920)
}

func TestUpstreamLiveSuccessorOrder(t *testing.T) {
	checkUpstreamLivenessFailures(t, "testdata/upstream-live-successor-matrix.jsonl", 268)
}

func TestUpstreamSpecificationFairnessFailures(t *testing.T) {
	checkUpstreamLivenessFailures(t, "testdata/upstream-spec-fair-failure-matrix.jsonl", 640)
}

func TestUpstreamFairnessLevelMatrix(t *testing.T) {
	checkUpstreamLivenessFailures(t, "testdata/upstream-fair-level-matrix.jsonl", 1152)
}

func TestUpstreamFairnessRegisterMatrix(t *testing.T) {
	checkUpstreamLivenessFailures(t, "testdata/upstream-fair-register-matrix.jsonl", 1152, knownRegisterLassoDifference)
}

func knownRegisterLassoDifference(name string) bool {
	for _, graph := range []string{"5", "12"} {
		for _, fairness := range []string{"WF_x(Next)", "SF_x(Next)"} {
			for _, view := range []string{"no-view", "x"} {
				if name == "fair-register-"+graph+"-"+fairness+"-5-"+view {
					return true
				}
			}
		}
	}
	return false
}

func checkUpstreamLivenessFailures(t *testing.T, path string, expected int, known ...func(string) bool) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		var tc struct {
			Name, Source, Config, Status, Error string
			Generated, Distinct, Depth, Queued  int
			OutDegrees                          []int
			Failure                             *struct {
				Message  string
				ExitCode int
				Trace    []struct {
					Action string
					State  map[string]int32
				}
			}
			Violation *struct {
				Name       string
				Properties []string
			}
		}
		if err := json.Unmarshal(scanner.Bytes(), &tc); err != nil {
			t.Fatal(err)
		}
		count++
		t.Run(tc.Name, func(t *testing.T) {
			got, err := Check(tc.Source, tc.Config)
			if tc.Error != "" {
				if err == nil {
					t.Fatalf("got %+v; want error %q", got, tc.Error)
				}
				if strings.HasPrefix(tc.Error, "The variable ") && !strings.Contains(err.Error(), tc.Error) {
					t.Fatalf("got error %q; want %q", err, tc.Error)
				}
				return
			}
			if err != nil || string(got.Status) != tc.Status || got.GeneratedStates != tc.Generated || got.DistinctStates != tc.Distinct || got.Depth != tc.Depth || got.QueuedStates != tc.Queued {
				t.Fatalf("got %+v, %v; want %s %d %d %d queued %d", got, err, tc.Status, tc.Generated, tc.Distinct, tc.Depth, tc.Queued)
			}
			if tc.Failure == nil {
				if tc.Violation != nil && got.Violation == nil {
					t.Fatalf("missing liveness violation; want %+v", tc.Violation)
				}
				if tc.Violation != nil && (got.Violation.Name != tc.Violation.Name || !reflect.DeepEqual(got.Violation.Properties, tc.Violation.Properties)) {
					if len(known) == 0 || !known[0](tc.Name) {
						t.Fatalf("got violation %+v; want %+v", got.Violation, tc.Violation)
					}
					t.Logf("known lasso path difference: got properties %v; upstream %v", got.Violation.Properties, tc.Violation.Properties)
				}
				if tc.Violation != nil && (got.Violation.LoopStart == nil || *got.Violation.LoopStart < 0 || *got.Violation.LoopStart >= len(got.Violation.Trace)) {
					t.Fatalf("invalid lasso %+v", got.Violation)
				}
				return
			}
			if got.Failure == nil || got.Failure.Message != tc.Failure.Message || got.Failure.ExitCode != tc.Failure.ExitCode || !got.HasOutDegrees || !reflect.DeepEqual(got.OutDegrees[:], tc.OutDegrees) || len(got.Failure.Trace) != len(tc.Failure.Trace) {
				t.Fatalf("got failure %+v, outdegrees %v; want failure %+v, outdegrees %v", got.Failure, got.OutDegrees, tc.Failure, tc.OutDegrees)
			}
			for i, want := range tc.Failure.Trace {
				x, ok := got.Failure.Trace[i].State["x"].Int()
				if !ok || x != want.State["x"] || got.Failure.Trace[i].Action != want.Action {
					t.Fatalf("trace step %d: got %+v; want %+v", i, got.Failure.Trace[i], want)
				}
			}
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != expected {
		t.Fatalf("got %d cases, want %d", count, expected)
	}
}
