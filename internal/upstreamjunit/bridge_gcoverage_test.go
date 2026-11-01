package upstreamjunit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func checkJavaGCoverageJUnit(t *testing.T, classes string, deps []string, temp, runDir string, test DiscoveryRecord) {
	t.Helper()
	dir := filepath.Join(temp, "g-coverage-run")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	events, err := executeJUnit(classes, deps, dir, runDir, test.Class, "testSpec")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateExecution(events, test.Display); err != nil {
		t.Fatalf("original Java GCoverageTest: %v", err)
	}
}

func checkGCoverageBridge(t *testing.T, binary, modelDir, classes string, deps []string, dir string, runBridge func([]bridgeRecord) (executionOutcome, error)) {
	t.Helper()
	dump := filepath.Join(dir, "g-go.dot")
	args := []string{"-tool", "-deadlock", "-fp", "0", "-coverage", "1", "-workers", "1", "-checkpoint", "0", "-dump", "dot", dump, "-coverage", "9999", "G"}
	result, err := runCommand(modelDir, 30*time.Second, binary, args...)
	if err != nil || result.ExitCode != 0 || result.TimedOut {
		t.Fatalf("G coverage invocation: result %+v error %v", result, err)
	}
	messages, err := Messages(result.Output)
	if err != nil || len(messages["2201"]) != 1 || len(messages["2202"]) != 1 || len(messages["2776"]) != 0 {
		t.Fatalf("G coverage report incomplete: error %v output %s", err, result.Output)
	}
	records, omissions, err := projectBridge(result.Output)
	if err != nil {
		t.Fatal(err)
	}
	checkCoverageRecorderAgainstJava(t, modelDir, classes, deps, dir, "g", args, records)
	if err := os.WriteFile(filepath.Join(dir, "bridge-target"), []byte("GCoverageTest"), 0o600); err != nil {
		t.Fatal(err)
	}
	if outcome, err := runBridge(records); err != nil || outcome != executionPassed {
		t.Fatalf("G original assertion: outcome %v error %v; omissions %v", outcome, err, omissions)
	}
	checkBridgeLifecycle(t, classes, deps, dir, "GoGCoverageTest", "GCoverageTest.java:46", records, result.ExitCode)
	checkGNegativeAssertions(t, classes, deps, dir, result.Output, records)
	omissions = append(omissions, "upstream metadir dump path and setup were not reproduced", "tool projection cannot establish complete typed recorder calls")
	t.Logf("original GCoverageTest assertion and teardown ran on projected Go observations; no Go JUnit pass: %v", bridgeVerdict(nil, omissions))
}

func checkGNegativeAssertions(t *testing.T, classes string, deps []string, dir, goodOutput string, records []bridgeRecord) {
	t.Helper()
	withGeneral, omissions, err := projectBridge(goodOutput + msg("1000", "synthetic error"))
	if err != nil || !strings.Contains(strings.Join(omissions, ";"), "presence only; recorder arguments unavailable for message 1000") {
		t.Fatalf("GENERAL projection: %v %v", omissions, err)
	}
	for _, tc := range []struct {
		name, line string
		records    []bridgeRecord
	}{
		{"finished", "GCoverageTest.java:44", removeBridgeRecord(t, records, "2186")},
		{"depth", "GCoverageTest.java:45", alterBridgeRecord(t, records, "2194", 0, "999")},
		{"stats", "GCoverageTest.java:46", alterBridgeRecord(t, records, "2199", 0, "999")},
		{"general", "GCoverageTest.java:49", withGeneral},
		{"action coverage", "GCoverageTest.java:51", alterBridgeRecord(t, records, "2772", 1, "999")},
		{"expression coverage", "GCoverageTest.java:51", alterBridgeRecord(t, records, "2221", 1, "999")},
		{"missing coverage", "GCoverageTest.java:51", removeBridgeRecord(t, records, "2221")},
	} {
		checkOriginalBridgeFailure(t, classes, deps, dir, "G "+tc.name, tc.line, tc.records)
	}
}
