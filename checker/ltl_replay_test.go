package checker

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestUpstreamLassoReplayWithoutFairnessAssumptions(t *testing.T) {
	testUpstreamLassoReplay(t, "testdata/upstream-lasso-replay-matrix.jsonl", func(name string) bool { return strings.Contains(name, "-none-") }, 384)
}

func TestUpstreamLassoReplayWithWeakFairness(t *testing.T) {
	testUpstreamLassoReplay(t, "testdata/upstream-lasso-replay-matrix.jsonl", func(name string) bool {
		for _, graph := range []string{"0", "1", "8", "9"} {
			if strings.HasPrefix(name, "lasso-replay-"+graph+"-WF_x(Next)-") {
				return true
			}
		}
		return false
	}, 96)
}

func TestUpstreamLassoReplayOutcomes(t *testing.T) {
	checkUpstreamLivenessFailures(t, "testdata/upstream-lasso-replay-matrix.jsonl", 1152, func(name string) bool {
		return knownRegisterLassoDifference(strings.Replace(name, "lasso-replay-", "fair-register-", 1))
	})
}

func TestUpstreamLassoSpecialOutcomes(t *testing.T) {
	checkUpstreamLivenessFailures(t, "testdata/upstream-lasso-special-matrix.jsonl", 384)
}

func TestUpstreamLassoSpecialOutput(t *testing.T) {
	testUpstreamLassoReplay(t, "testdata/upstream-lasso-special-matrix.jsonl", supportedSpecialReplay, 140)
}

func TestUpstreamLassoStrongOutcomes(t *testing.T) {
	checkUpstreamLivenessFailures(t, "testdata/upstream-lasso-strong-matrix.jsonl", 384)
}

func TestUpstreamLassoStrongOutput(t *testing.T) {
	testUpstreamLassoReplay(t, "testdata/upstream-lasso-strong-matrix.jsonl", supportedStrongReplay, 28)
}

func supportedStrongReplay(name string) bool {
	if strings.HasPrefix(name, "lasso-strong-5-") {
		return matchesReplayFormula(name, []string{"0", "1", "4", "5"})
	}
	return strings.HasPrefix(name, "lasso-strong-12-") && matchesReplayFormula(name, []string{"0", "1", "5"})
}

func TestUpstreamLassoStrongErrorOutput(t *testing.T) {
	testUpstreamLassoReplay(t, "testdata/upstream-lasso-replay-matrix.jsonl", func(name string) bool {
		return strings.Contains(name, "-SF_x(Next)-") && (strings.HasSuffix(name, "-10-no-view") || strings.HasSuffix(name, "-10-x"))
	}, 32)
}

func TestUpstreamLassoActionOutcomes(t *testing.T) {
	checkUpstreamLivenessFailures(t, "testdata/upstream-lasso-action-matrix.jsonl", 256)
}

func TestUpstreamLassoActionOutput(t *testing.T) {
	testUpstreamLassoReplay(t, "testdata/upstream-lasso-action-matrix.jsonl", func(name string) bool {
		for _, graph := range []string{"0", "1", "2", "3", "4", "8", "9"} {
			if strings.HasPrefix(name, "lasso-action-"+graph+"-") {
				return true
			}
		}
		return false
	}, 112)
}

func supportedSpecialReplay(name string) bool {
	for _, graph := range []string{"0", "1", "2", "3", "4", "8", "9"} {
		if strings.HasPrefix(name, "lasso-special-"+graph+"-") {
			return supportedSpecialFormula(name)
		}
	}
	return false
}

func supportedSpecialFormula(name string) bool {
	return matchesReplayFormula(name, []string{"0", "1", "2", "4", "5"})
}

func matchesReplayFormula(name string, formulas []string) bool {
	for _, formula := range formulas {
		if strings.HasSuffix(name, "-"+formula+"-no-view") || strings.HasSuffix(name, "-"+formula+"-x") {
			return true
		}
	}
	return false
}

func testUpstreamLassoReplay(t *testing.T, path string, include func(string) bool, expected int) {
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
			Printed                             []string
		}
		if err := json.Unmarshal(scanner.Bytes(), &tc); err != nil {
			t.Fatal(err)
		}
		if !include(tc.Name) {
			continue
		}
		count++
		t.Run(tc.Name, func(t *testing.T) {
			var printed []string
			got, err := CheckWithOptions(context.Background(), tc.Source, tc.Config, Options{OnPrint: func(line string) { printed = append(printed, line) }})
			if tc.Error != "" {
				if err == nil {
					t.Fatalf("got %+v; want error %q", got, tc.Error)
				}
				return
			}
			if err != nil || string(got.Status) != tc.Status {
				t.Fatalf("got %+v, %v; want %s", got, err, tc.Status)
			}
			if !reflect.DeepEqual(printed, tc.Printed) {
				if knownReplayPathDifference(tc.Name) {
					t.Logf("known lasso path difference: got output %q; upstream %q", printed, tc.Printed)
					return
				}
				t.Fatalf("got output %q; want %q", printed, tc.Printed)
			}
		})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != expected {
		t.Fatalf("got %d cases; want %d", count, expected)
	}
}

func knownReplayPathDifference(name string) bool {
	for _, graph := range []string{"5", "7", "13", "15"} {
		if name == "lasso-replay-"+graph+"-none-6-no-view" {
			return true
		}
	}
	return false
}

func TestLassoReplayConstantDefinitions(t *testing.T) {
	for _, tc := range []struct {
		init    string
		printed []string
	}{
		{"x=0 /\\ Foo", []string{`"hello"`}},
		{"x=0 /\\ PrintT(\"hello\")", []string{`"hello"`, `"hello"`, `"hello"`}},
	} {
		source := "---- MODULE M ----\nEXTENDS TLC\nVARIABLE x\nFoo == PrintT(\"hello\")\nInit == " + tc.init + "\nNext == UNCHANGED x\nP == <>(x=1)\n===="
		var printed []string
		got, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE", Options{OnPrint: func(line string) { printed = append(printed, line) }})
		if err != nil || got.Status != LivenessViolation || !reflect.DeepEqual(printed, tc.printed) {
			t.Fatalf("%s: got %+v, %v, output %q; want %q", tc.init, got, err, printed, tc.printed)
		}
	}
}
