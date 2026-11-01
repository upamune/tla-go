package upstreamjunit

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/upamune/tla-go/internal/recorderdebug"
)

func checkJavaLoopJUnit(t *testing.T, classes string, deps []string, temp, basedir string, test DiscoveryRecord) {
	t.Helper()
	dir := filepath.Join(temp, "loop-java-junit")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	events, err := executeJUnit(classes, deps, dir, basedir, test.Class, "testSpec")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateExecution(events, test.Display); err != nil {
		t.Fatalf("original Java LoopTestWeakFair: %v", err)
	}
}

func checkLoopRecorderBridge(t *testing.T, binary, modelDir, classes string, deps []string, dir string) {
	t.Helper()
	path := filepath.Join(dir, "loop-recorder.jsonl")
	args := []string{"-tool", "-deadlock", "-fp", "0", "-workers", "1", "-checkpoint", "0", "-lncheck", "final", "-config", "SystemLoopWeakFair", "SystemLoop"}
	result, err := runCommandWithEnv(modelDir, 30*time.Second, []string{"TLA_GO_DEBUG_RECORDER=" + path}, binary, args...)
	if err != nil || result.ExitCode != 0 || result.TimedOut {
		t.Fatalf("Go Loop: result %+v error %v", result, err)
	}
	projected, omissions, err := projectBridge(result.Output)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	events, err := recorderdebug.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	records, err := coverageRecorderRecords(projected, events)
	if err != nil {
		t.Fatal(err)
	}
	checkLoopJavaRecorder(t, modelDir, classes, deps, dir, args, records)
	checkBridgeLifecycle(t, classes, deps, dir, "GoLoopTestWeakFair", "LoopTestWeakFair.java:50", records, result.ExitCode)
	checkLoopTemporalAssertion(t, classes, deps, dir, records)
	omissions = append(omissions, "coverage and dump were not run", "original setup, complete recorder and final liveness equivalence remain unverified")
	if bridgeVerdict(nil, omissions) == nil {
		t.Fatal("partial Loop observations were counted as a Go JUnit pass")
	}
	t.Logf("Loop original assertion and teardown used partial typed Go events; no Go JUnit pass: %v", bridgeVerdict(nil, omissions))
}

func checkLoopJavaRecorder(t *testing.T, modelDir, classes string, deps []string, dir string, args []string, records []bridgeRecord) {
	t.Helper()
	path := filepath.Join(dir, "loop-java-recorder")
	cp := strings.Join(append([]string{classes}, deps...), string(os.PathListSeparator))
	javaArgs := []string{"-Djava.io.tmpdir=" + t.TempDir(), "-cp", cp, "CaptureRecorder", path}
	javaArgs = append(javaArgs, args[:len(args)-1]...)
	javaArgs = append(javaArgs, "-metadir", filepath.Join(dir, "loop-java-states"), args[len(args)-1])
	result, err := runCommand(modelDir, 30*time.Second, "java", javaArgs...)
	if err != nil || result.ExitCode != 0 || result.TimedOut {
		t.Fatalf("Java Loop recorder: result %+v error %v", result, err)
	}
	actual, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var selected []bridgeRecord
	for _, record := range records {
		if slices.Contains([]string{"2190", "2192", "2194", "2199", "2212"}, record.code) {
			selected = append(selected, record)
		}
	}
	want, err := encodeBridge(selected)
	if err != nil || string(actual) != want {
		t.Fatalf("typed Loop events: Java %s Go %s error %v", actual, want, err)
	}
}

func checkLoopTemporalAssertion(t *testing.T, classes string, deps []string, dir string, records []bridgeRecord) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "bridge-exit"), []byte("0"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]bridgeRecord{
		removeBridgeRecord(t, records, "2192"),
		alterBridgeRecord(t, records, "2192", 1, "999"),
	} {
		data, err := encodeBridge(bad)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "bridge-records"), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		events, err := executeJUnit(classes, deps, dir, dir, "GoLoopTestWeakFair", "testSpec")
		if err != nil || !validBridgeFailure(events, "testSpec(GoLoopTestWeakFair)") || !strings.Contains(events[1].value, "LoopTestWeakFair.java:54") {
			t.Fatalf("Loop temporal assertion: events %v error %v", events, err)
		}
	}
}
