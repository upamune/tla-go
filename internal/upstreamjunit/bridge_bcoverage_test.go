package upstreamjunit

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func checkBridgeBCoverage(t *testing.T, binary, modelDir, classes string, deps []string, dir string, runBridge func([]bridgeRecord) (executionOutcome, error)) {
	t.Helper()
	// Use a separate dump path, while preserving the upstream coverage and dump
	// options. The result remains a partial observation, not a JUnit pass.
	dump := filepath.Join(dir, "b-go.dot")
	args := []string{"-tool", "-deadlock", "-fp", "0", "-coverage", "1", "-workers", "1", "-checkpoint", "0", "-dump", "dot", dump, "-coverage", "9999", "B"}
	result, records, omissions := runCoverageRecorder(t, binary, modelDir, filepath.Join(dir, "b-recorder.jsonl"), args)
	checkCoverageRecorderAgainstJava(t, modelDir, classes, deps, dir, "b", args, records)
	omissions = append(omissions, "upstream metadir dump path and setup were not reproduced", "typed recorder diagnostics omit other calls")
	if err := os.WriteFile(filepath.Join(dir, "bridge-target"), []byte("BCoverageTest"), 0o600); err != nil {
		t.Fatal(err)
	}
	if outcome, err := runBridge(records); err != nil || outcome != executionPassed {
		t.Fatalf("B coverage original assertion: outcome %v error %v; omissions %v", outcome, err, omissions)
	}
	checkBridgeLifecycle(t, classes, deps, dir, "GoBCoverageTest", "BCoverageTest.java:47", records, result.ExitCode)
	checkBNegativeAssertions(t, binary, modelDir, classes, deps, dir, result.Output, records)
	checkGeneralJavaTypes(t, modelDir, classes, deps, dir)
	tampered := slices.Clone(records)
	for i := range tampered {
		if tampered[i].code == "2772" {
			tampered[i].values = slices.Clone(tampered[i].values)
			tampered[i].values[1] = "999"
			if outcome, err := runBridge(tampered); err != nil || outcome != executionUpstreamAssertion {
				t.Fatalf("B tampered action coverage: outcome %v error %v", outcome, err)
			}
			t.Logf("original BCoverageTest assertions and teardown ran on partial typed Go observations; no Go JUnit pass: %v", bridgeVerdict(nil, omissions))
			return
		}
	}
	t.Fatalf("B report missing action coverage: %s", strings.Join(omissions, "; "))
}

func checkCoverageRecorderAgainstJava(t *testing.T, modelDir, classes string, deps []string, dir, label string, args []string, projected []bridgeRecord) {
	t.Helper()
	capture := filepath.Join(dir, "java-"+label+"-records")
	cp := strings.Join(append([]string{classes}, deps...), string(os.PathListSeparator))
	javaArgs := []string{"-Djava.io.tmpdir=" + t.TempDir(), "-cp", cp, "CaptureRecorder", capture}
	dumpIndex := slices.Index(args, "-dump")
	if dumpIndex < 0 || dumpIndex+2 >= len(args) || args[dumpIndex+1] != "dot" {
		t.Fatalf("missing coverage DOT argument: %v", args)
	}
	goDump, javaDump := args[dumpIndex+2], filepath.Join(dir, label+"-java.dot")
	javaOptions := slices.Clone(args[:len(args)-1])
	javaOptions[dumpIndex+2] = javaDump
	javaArgs = append(javaArgs, javaOptions...)
	javaArgs = append(javaArgs, "-metadir", filepath.Join(dir, label+"-java-states"), args[len(args)-1])
	result, err := runCommand(modelDir, 30*time.Second, "java", javaArgs...)
	if err != nil || result.ExitCode != 0 || result.TimedOut {
		t.Fatalf("Java recorder capture: result %+v error %v", result, err)
	}
	compareCoverageDumps(t, goDump, javaDump)
	actual, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	var selected []bridgeRecord
	for _, record := range projected {
		switch record.code {
		case "2190", "2194", "2199", "2221", "2269", "2772", "2773", "2774", "2775":
			selected = append(selected, record)
		}
	}
	want, err := encodeBridge(selected)
	if err != nil || string(actual) != want {
		t.Fatalf("typed Java recorder and Go projection differ: error %v\nJava %s\nGo %s", err, actual, want)
	}
}

func compareCoverageDumps(t *testing.T, goPath, javaPath string) {
	t.Helper()
	goDump, goErr := os.ReadFile(goPath)
	javaDump, javaErr := os.ReadFile(javaPath)
	if goErr != nil || javaErr != nil || !strings.Contains(string(goDump), " -> ") || string(goDump) != string(javaDump) {
		t.Fatalf("coverage DOT: Go error %v, Java error %v\nGo %s\nJava %s", goErr, javaErr, goDump, javaDump)
	}
}
