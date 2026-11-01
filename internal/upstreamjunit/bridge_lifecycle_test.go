package upstreamjunit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Original methods and teardown execute in JUnit; setup is an adapter.
// Even a successful run cannot establish the omitted recorder and setup effects.
func checkBridgeLifecycle(t *testing.T, classes string, deps []string, dir, class, assertionStack string, records []bridgeRecord, exit int) {
	t.Helper()
	run := func(records []bridgeRecord, exit int) []executionEvent {
		t.Helper()
		data, err := encodeBridge(records)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "bridge-records"), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "bridge-exit"), []byte(fmt.Sprint(exit)), 0o600); err != nil {
			t.Fatal(err)
		}
		events, err := executeJUnit(classes, deps, dir, dir, class, "testSpec")
		if err != nil {
			t.Fatal(err)
		}
		return events
	}
	display := "testSpec(" + class + ")"
	events := run(records, exit)
	if err := validateExecution(events, display); err != nil {
		t.Fatalf("%s adapter: %v", class, err)
	}
	t.Logf("%s: original assertion and inherited teardown passed on projected Go observations; setup and recorder completeness remain unverified", class)
	for _, tc := range []struct {
		name, stack string
		records     []bridgeRecord
		exit        int
	}{
		{"assertion", assertionStack, []bridgeRecord{{"2186", []string{"1ms"}}, {"2199", []string{"7", "0", "0"}}}, 0},
		{"teardown", "tlc2.tool.liveness.ModelCheckerTestCase.assertExitStatus", records, 1},
	} {
		events = run(tc.records, tc.exit)
		if !validBridgeFailure(events, display) || !strings.HasPrefix(events[1].value, "java.lang.AssertionError\n") || !strings.Contains(events[1].value, tc.stack) {
			t.Errorf("%s: expected upstream assertion failure from %s, got %+v", tc.name, tc.stack, events)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "bridge-exit"), []byte("invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	events, err := executeJUnit(classes, deps, dir, dir, class, "testSpec")
	if err != nil {
		t.Fatal(err)
	}
	if validateExecution(events, display) == nil || !hasSetupFailure(events) {
		t.Errorf("malformed exit must fail setup, got %+v", events)
	}
}

func hasSetupFailure(events []executionEvent) bool {
	for _, event := range events {
		if event.kind == "failure" && strings.Contains(event.value, "cannot load Go observations") {
			return true
		}
	}
	return false
}
