package upstreamjunit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func checkJavaStatisticsJUnit(t *testing.T, classes string, deps []string, temp, runDir string, statistics DiscoveryRecord) {
	t.Helper()
	dir := filepath.Join(temp, "statistics-run")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	events, err := executeJUnit(classes, deps, dir, runDir, statistics.Class, "testSpec")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateExecution(events, statistics.Display); err != nil {
		t.Fatal(err)
	}
	t.Logf("Java JUnit pass: %s; events=%v", statistics.Display, events)
}

func checkCoverageStatisticsBridge(t *testing.T, binary, modelDir, classes string, deps []string, dir string, runBridge func([]bridgeRecord) (executionOutcome, error)) {
	t.Helper()
	dump := filepath.Join(dir, "statistics-go.dot")
	args := []string{"-tool", "-deadlock", "-fp", "0", "-coverage", "1", "-workers", "1", "-checkpoint", "0", "-dump", "dot", dump, "-coverage", "9999", "CoverageStatistics"}
	result, records, omissions := runCoverageRecorder(t, binary, modelDir, filepath.Join(dir, "statistics-recorder.jsonl"), args)
	checkCoverageRecorderAgainstJava(t, modelDir, classes, deps, dir, "statistics", args, records)
	if err := os.WriteFile(filepath.Join(dir, "bridge-target"), []byte("CoverageStatisticsTest"), 0o600); err != nil {
		t.Fatal(err)
	}
	if outcome, err := runBridge(records); err != nil || outcome != executionPassed {
		t.Fatalf("statistics original assertion: outcome %v error %v; omissions %v", outcome, err, omissions)
	}
	checkBridgeLifecycle(t, classes, deps, dir, "GoCoverageStatisticsTest", "CoverageStatisticsTest.java:47", records, result.ExitCode)
	checkStatisticsNegativeAssertions(t, classes, deps, dir, result.Output, records)
	omissions = append(omissions, "upstream metadir dump path and setup were not reproduced", "typed recorder diagnostics omit other calls")
	t.Logf("original CoverageStatisticsTest assertion and teardown ran on partial typed Go observations; no Go JUnit pass: %v", bridgeVerdict(nil, omissions))
}

func checkStatisticsNegativeAssertions(t *testing.T, classes string, deps []string, dir, goodOutput string, records []bridgeRecord) {
	t.Helper()
	withGeneral, omissions, err := projectBridge(goodOutput + msg("1000", "synthetic error"))
	if err != nil || !strings.Contains(strings.Join(omissions, ";"), "presence only; recorder arguments unavailable for message 1000") {
		t.Fatalf("GENERAL projection: %v %v", omissions, err)
	}
	for _, tc := range []struct {
		name, line string
		records    []bridgeRecord
	}{
		{"finished", "CoverageStatisticsTest.java:45", removeBridgeRecord(t, records, "2186")},
		{"depth", "CoverageStatisticsTest.java:46", alterBridgeRecord(t, records, "2194", 0, "999")},
		{"stats", "CoverageStatisticsTest.java:47", alterBridgeRecord(t, records, "2199", 0, "999")},
		{"general", "CoverageStatisticsTest.java:50", withGeneral},
		{"action coverage", "CoverageStatisticsTest.java:52", alterBridgeRecord(t, records, "2772", 1, "999")},
		{"expression coverage", "CoverageStatisticsTest.java:52", alterBridgeRecord(t, records, "2221", 1, "999")},
		{"missing coverage", "CoverageStatisticsTest.java:52", removeBridgeRecord(t, records, "2221")},
	} {
		checkOriginalBridgeFailure(t, classes, deps, dir, tc.name, tc.line, tc.records)
	}
}
