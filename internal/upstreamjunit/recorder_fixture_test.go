package upstreamjunit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func checkTypedRecorderFixture(t *testing.T, classes string, deps []string, dir string) {
	t.Helper()
	same := recorderRecord{"2199", "strings", []recorderValue{{"s", "first\n\t😀"}, {"s", ""}}}
	mixed := recorderRecord{"2199", "objects", []recorderValue{{"s", "mixed"}, {"i", "-7"}, {"l", "2147483648"}, {"b", "true"}, {"n", ""}}}
	safety := []recorderRecord{{"2110", "strings", []recorderValue{{"s", "Inv"}}}, {"2114", "strings", nil}, {"2121", "strings", nil}}
	data, err := encodeRecorder(append([]recorderRecord{same, mixed, same}, safety...))
	if err != nil {
		t.Fatal(err)
	}
	run := func(data string) []executionEvent {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "bridge-records"), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		events, err := executeJUnit(classes, deps, dir, dir, "RecorderFixture", "typed")
		if err != nil {
			t.Fatal(err)
		}
		return events
	}
	const display = "typed(RecorderFixture)"
	if err := validateExecution(run(data), display); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ data, exception string }{
		{"2199\tstrings\ti:1\n", "java.lang.IllegalArgumentException"},
		{"2199\tobjects\tx:abc\n", "java.lang.IllegalArgumentException"},
		{"2199\tobjects\ti:2147483648\n", "java.lang.NumberFormatException"},
		{"2199\tobjects\ts:/w==\n", "java.nio.charset.MalformedInputException"},
		{"2199\tother\ts:YQ==\n", "java.lang.IllegalArgumentException"},
		{"9999\tstrings\ts:YQ==\n", "java.lang.IllegalArgumentException"},
		{"002199\tstrings\ts:YQ==\n", "java.lang.IllegalArgumentException"},
	} {
		events := run(tc.data)
		if !validBridgeFailure(events, display) || !strings.HasPrefix(events[1].value, tc.exception+"\n") || !strings.Contains(events[1].value, "Bridge.loadRecords") {
			t.Errorf("invalid recorder data %q did not fail in decoder: %+v", tc.data, events)
		}
	}
}
