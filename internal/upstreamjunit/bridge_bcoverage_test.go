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
	// The upstream setup requests a dot dump. Go coverage currently rejects it,
	// so this invocation remains a partial observation, never a JUnit pass.
	args := []string{"-tool", "-deadlock", "-fp", "0", "-coverage", "1", "-workers", "1", "-checkpoint", "0", "-coverage", "9999", "B"}
	result, err := runCommand(modelDir, 30*time.Second, binary, args...)
	if err != nil || result.ExitCode != 0 || result.TimedOut {
		t.Fatalf("B coverage invocation: result %+v error %v", result, err)
	}
	messages, err := Messages(result.Output)
	if err != nil || len(messages["2201"]) != 1 || len(messages["2202"]) != 1 || len(messages["2776"]) != 0 {
		t.Fatalf("B coverage report incomplete: error %v output %s", err, result.Output)
	}
	records, omissions, err := projectBridge(result.Output)
	if err != nil {
		t.Fatal(err)
	}
	checkBRecorderAgainstJava(t, modelDir, classes, deps, dir, args, records)
	omissions = append(omissions, "upstream dot dump and setup were not reproduced", "tool projection cannot establish complete typed recorder calls")
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
			t.Logf("original BCoverageTest assertions and teardown ran on projected Go observations; no Go JUnit pass: %v", bridgeVerdict(nil, omissions))
			return
		}
	}
	t.Fatalf("B report missing action coverage: %s", strings.Join(omissions, "; "))
}

func checkBRecorderAgainstJava(t *testing.T, modelDir, classes string, deps []string, dir string, args []string, projected []bridgeRecord) {
	t.Helper()
	capture := filepath.Join(dir, "java-b-records")
	cp := strings.Join(append([]string{classes}, deps...), string(os.PathListSeparator))
	javaArgs := []string{"-Djava.io.tmpdir=" + t.TempDir(), "-cp", cp, "CaptureRecorder", capture}
	javaArgs = append(javaArgs, args[:len(args)-1]...)
	javaArgs = append(javaArgs, "-metadir", filepath.Join(dir, "b-java-states"), args[len(args)-1])
	result, err := runCommand(modelDir, 30*time.Second, "java", javaArgs...)
	if err != nil || result.ExitCode != 0 || result.TimedOut {
		t.Fatalf("Java recorder capture: result %+v error %v", result, err)
	}
	actual, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	var selected []bridgeRecord
	for _, record := range projected {
		switch record.code {
		case "2190", "2194", "2199", "2221", "2772", "2773", "2774", "2775":
			selected = append(selected, record)
		}
	}
	want, err := encodeBridge(selected)
	if err != nil || string(actual) != want {
		t.Fatalf("typed Java recorder and Go projection differ: error %v\nJava %s\nGo %s", err, actual, want)
	}
}
