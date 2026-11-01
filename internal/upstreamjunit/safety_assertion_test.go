package upstreamjunit

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// Run a discovered upstream assertion against actual typed Go safety calls.
// ETest3 requires coverage, which the Go model currently rejects: an assertion
// result on the partial stream is therefore never counted as a JUnit pass.
func checkSafetyAssertionBridge(t *testing.T, binary, model, classes string, deps []string, basedir, class string) {
	t.Helper()
	args := []string{"-fp", "0", "-workers", "1", "-checkpoint", "0", "-dump", "dot", filepath.Join(t.TempDir(), "graph.dot"), "etest3"}
	checkSafetyCoverageUnsupported(t, binary, model, args)
	records, omissions := readSafetyCalls(t, binary, model, args)
	omissions = append(omissions, "upstream ETest3 requires -coverage 1; Go rejected that model and this invocation omits coverage")
	if bridgeVerdict(nil, omissions) == nil {
		t.Fatal("partial recorder and missing coverage cannot establish a JUnit pass")
	}
	assertSafetyCalls(t, records)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bridge-target"), []byte("class:"+class), 0o600); err != nil {
		t.Fatal(err)
	}
	checkSafetyBridgeCases(t, classes, deps, dir, basedir, records)
	t.Logf("original ETest3 assertion executed with partial Go safety calls; no Go JUnit pass: %v", bridgeVerdict(nil, omissions))
}

func checkSafetyCoverageUnsupported(t *testing.T, binary, model string, args []string) {
	t.Helper()
	withCoverage := append([]string{"-coverage", "1"}, args...)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, withCoverage...)
	cmd.Dir = model
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if ctx.Err() != nil || !errors.As(err, &exit) || exit.ExitCode() != 255 || !strings.Contains(string(out), "-coverage does not yet support") {
		t.Fatalf("expected unsupported coverage: %v, context=%v\n%s", err, ctx.Err(), out)
	}
}

func readSafetyCalls(t *testing.T, binary, model string, args []string) ([]recorderRecord, []string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "records.jsonl")
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = model
	cmd.Env = append(os.Environ(), "TLA_GO_DEBUG_RECORDER="+path)
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if ctx.Err() != nil || !errors.As(err, &exit) || exit.ExitCode() != 11 {
		t.Fatalf("Go safety invocation: %v, context=%v\n%s", err, ctx.Err(), out)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	records, omissions, err := decodePartialRecorder(file)
	if err != nil {
		t.Fatal(err)
	}
	return records, omissions
}

func assertSafetyCalls(t *testing.T, records []recorderRecord) {
	t.Helper()
	for _, code := range []string{"2114", "2121", "2199", "2186"} {
		if !slices.ContainsFunc(records, func(record recorderRecord) bool { return record.code == code && record.array == "strings" }) {
			t.Fatalf("missing observed safety call %s: %+v", code, records)
		}
	}
}

func checkSafetyBridgeCases(t *testing.T, classes string, deps []string, dir, basedir string, records []recorderRecord) {
	t.Helper()
	cases := []struct {
		name    string
		records []recorderRecord
		want    executionOutcome
	}{
		{"partial observations", records, executionPassed},
		{"missing deadlock", slices.DeleteFunc(slices.Clone(records), func(r recorderRecord) bool { return r.code == "2114" }), executionUpstreamAssertion},
		{"missing finished", slices.DeleteFunc(slices.Clone(records), func(r recorderRecord) bool { return r.code == "2186" }), executionUpstreamAssertion},
		{"wrong statistics", wrongSafetyStatistics(t, records), executionUpstreamAssertion},
		{"general error", append(slices.Clone(records), recorderRecord{code: "1000", array: "strings"}), executionUpstreamAssertion},
		{"uncovered expression", append(slices.Clone(records), recorderRecord{code: "2221", array: "strings", values: []recorderValue{{"s", "line 1, col 1 to line 1, col 2 of module M"}, {"s", "0"}}}), executionUpstreamAssertion},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := encodeRecorder(tc.records)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "bridge-records"), []byte(encoded), 0o600); err != nil {
				t.Fatal(err)
			}
			events, err := executeJUnit(classes, deps, dir, basedir, "Bridge", "observations")
			if err != nil || classifyBridgeExecution(events) != tc.want {
				t.Fatalf("expected %v; events=%+v error=%v", tc.want, events, err)
			}
		})
	}
}

func wrongSafetyStatistics(t *testing.T, records []recorderRecord) []recorderRecord {
	t.Helper()
	result := slices.Clone(records)
	i := slices.IndexFunc(result, func(record recorderRecord) bool { return record.code == "2199" })
	if i < 0 || len(result[i].values) != 3 {
		t.Fatalf("missing observed statistics: %+v", records)
	}
	result[i].values = slices.Clone(result[i].values)
	result[i].values[0].text = "999"
	return result
}
