package upstreamjunit

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func checkBNegativeAssertions(t *testing.T, binary, modelDir, classes string, deps []string, dir, goodOutput string, records []bridgeRecord) {
	t.Helper()
	goResult, err := runCommand(modelDir, 30*time.Second, binary, "-tool", "-dfid", "2", "-workers", "2", "B")
	if err != nil || goResult.ExitCode != 255 {
		t.Fatalf("Go GENERAL control: exit %d error %v output %s", goResult.ExitCode, err, goResult.Output)
	}
	general := actualGeneralMessage(t, goResult.Output)
	withGeneral, omissions, err := projectBridge(goodOutput + general)
	if err != nil || !strings.Contains(strings.Join(omissions, ";"), "presence only; recorder arguments unavailable for message 1000") {
		t.Fatalf("Go GENERAL projection: %v %v", omissions, err)
	}
	withMismatch, omissions, err := projectBridge(goodOutput + msg("2776", "lookup mismatch"))
	if err != nil || !strings.Contains(strings.Join(omissions, ";"), "presence only; recorder arguments unavailable for message 2776") {
		t.Fatalf("mismatch projection: %v %v", omissions, err)
	}
	for _, tc := range []struct {
		name, line string
		records    []bridgeRecord
	}{
		{"finished", "BCoverageTest.java:45", removeBridgeRecord(t, records, "2186")},
		{"depth", "BCoverageTest.java:46", alterBridgeRecord(t, records, "2194", 0, "999")},
		{"stats", "BCoverageTest.java:47", alterBridgeRecord(t, records, "2199", 0, "999")},
		{"real Go GENERAL", "BCoverageTest.java:50", withGeneral},
		{"coverage mismatch", "BCoverageTest.java:52", withMismatch},
		{"action coverage", "BCoverageTest.java:53", alterBridgeRecord(t, records, "2772", 1, "999")},
		{"zero coverage", "BCoverageTest.java:53", alterBridgeRecord(t, records, "2221", 1, "0")},
	} {
		checkOriginalBFailure(t, classes, deps, dir, tc.name, tc.line, tc.records)
	}
}

func actualGeneralMessage(t *testing.T, output string) string {
	t.Helper()
	const start = "@!@!@STARTMSG 1000:1 @!@!@\n"
	const end = "@!@!@ENDMSG 1000 @!@!@\n"
	i := strings.Index(output, start)
	if i < 0 {
		t.Fatalf("Go did not emit GENERAL: %s", output)
	}
	j := strings.Index(output[i:], end)
	if j < 0 {
		t.Fatalf("Go emitted incomplete GENERAL: %s", output)
	}
	return output[i : i+j+len(end)]
}

func removeBridgeRecord(t *testing.T, records []bridgeRecord, code string) []bridgeRecord {
	t.Helper()
	i := slices.IndexFunc(records, func(r bridgeRecord) bool { return r.code == code })
	if i < 0 {
		t.Fatalf("missing record %s", code)
	}
	return slices.Delete(slices.Clone(records), i, i+1)
}

func alterBridgeRecord(t *testing.T, records []bridgeRecord, code string, index int, value string) []bridgeRecord {
	t.Helper()
	result := slices.Clone(records)
	i := slices.IndexFunc(result, func(r bridgeRecord) bool { return r.code == code })
	if i < 0 || index >= len(result[i].values) {
		t.Fatalf("missing record %s value %d", code, index)
	}
	result[i].values = slices.Clone(result[i].values)
	result[i].values[index] = value
	return result
}

func checkOriginalBFailure(t *testing.T, classes string, deps []string, dir, name, line string, records []bridgeRecord) {
	t.Helper()
	data, err := encodeBridge(records)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bridge-records"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	events, err := executeJUnit(classes, deps, dir, dir, "Bridge", "observations")
	if err != nil || classifyBridgeExecution(events) != executionUpstreamAssertion || !strings.Contains(events[1].value, line) {
		t.Errorf("%s: expected upstream assertion at %s, events %v error %v", name, line, events, err)
	}
}

func checkGeneralJavaTypes(t *testing.T, modelDir, classes string, deps []string, dir string) {
	t.Helper()
	path := filepath.Join(dir, "java-general-records")
	cp := strings.Join(append([]string{classes}, deps...), string(os.PathListSeparator))
	result, err := runCommand(modelDir, 30*time.Second, "java", "-Djava.io.tmpdir="+t.TempDir(), "-cp", cp, "CaptureRecorder", path, "-tool", "-dfid", "2", "-workers", "2", "-metadir", filepath.Join(dir, "general-java-states"), "B")
	if err != nil || result.ExitCode != 255 {
		t.Fatalf("Java GENERAL: exit %d error %v output %s", result.ExitCode, err, result.Output)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "1000\tstrings\ts:") || !strings.Contains(string(data), "1000\tunsupported-array:[Ljava.lang.Object;\tunsupported-value:util.Assert$TLCRuntimeException") {
		t.Fatalf("Java GENERAL recorder types %q, error %v", data, err)
	}
}
