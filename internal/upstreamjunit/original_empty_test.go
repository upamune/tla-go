package upstreamjunit

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// This executes the original EmptyTest class and inherited setup and teardown.
// The TLC adapter starts Go during setup, then feeds projected observations to
// the recorder installed by the original setup. The projection is incomplete.
func checkOriginalEmptySetup(t *testing.T, classes string, deps []string, basedir, binary, model string) {
	t.Helper()
	adapter := compileOriginalEmptyAdapter(t, classes, deps)
	events, omissions := runOriginalEmpty(t, adapter, classes, deps, basedir, binary, model, "success", true, true, "")
	const display = "testSpec(tlc2.tool.EmptyTest)"
	if err := validateExecution(events, display); err != nil {
		t.Fatalf("original EmptyTest with Go projection: %v", err)
	}
	if len(omissions) == 0 || bridgeVerdict(nil, omissions) == nil {
		t.Fatalf("incomplete Go observation must never become a JUnit pass: %v", omissions)
	}
	checkOriginalEmptyControls(t, adapter, classes, deps, basedir, binary, model, display)
	t.Logf("original EmptyTest setup, assertion and teardown executed with projected Go records; unsupported: %v", bridgeVerdict(nil, omissions))
}

func compileOriginalEmptyAdapter(t *testing.T, classes string, deps []string) string {
	t.Helper()
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src", "tlc2")
	if err := os.MkdirAll(srcDir, 0o700); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(srcDir, "TLC.java")
	if err := os.WriteFile(src, []byte(originalEmptyTLCJava), 0o600); err != nil {
		t.Fatal(err)
	}
	adapter := filepath.Join(dir, "classes")
	if err := os.Mkdir(adapter, 0o700); err != nil {
		t.Fatal(err)
	}
	cp := strings.Join(append([]string{classes}, deps...), string(os.PathListSeparator))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "javac", "-J-Xmx256m", "-proc:none", "-cp", cp, "-d", adapter, src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile TLC adapter: %v\n%s", err, out)
	}
	checkOriginalEmptyArguments(t, ctx, dir, adapter, cp)
	return adapter
}

func checkOriginalEmptyArguments(t *testing.T, ctx context.Context, dir, adapter, cp string) {
	t.Helper()
	src := filepath.Join(dir, "OriginalEmptyArgsFixture.java")
	if err := os.WriteFile(src, []byte(originalEmptyArgsFixtureJava), 0o600); err != nil {
		t.Fatal(err)
	}
	cp = adapter + string(os.PathListSeparator) + cp
	compile := exec.CommandContext(ctx, "javac", "-J-Xmx128m", "-proc:none", "-cp", cp, "-d", adapter, src)
	if out, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("compile argument controls: %v\n%s", err, out)
	}
	cmd := exec.CommandContext(ctx, "java", "-Xmx128m", "-cp", cp, "OriginalEmptyArgsFixture")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("adapter argument controls: %v\n%s", err, out)
	}
}

func runOriginalEmpty(t *testing.T, adapter, classes string, deps []string, basedir, binary, model, mode string, expectAdapter, expectOmissions bool, expectHelperLog string) ([]executionEvent, []string) {
	t.Helper()
	dir := t.TempDir()
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cp := strings.Join(append([]string{adapter, classes}, deps...), string(os.PathListSeparator))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-Xmx256m", "-Dbasedir="+basedir+string(os.PathSeparator), "-Djava.io.tmpdir="+dir,
		"-Dtla.bridge.helper="+helper, "-Dtla.bridge.binary="+binary, "-Dtla.bridge.model="+model, "-Dtla.bridge.mode="+mode,
		"-cp", cp, "Execution", "tlc2.tool.EmptyTest", "testSpec")
	cmd.Dir = dir
	events, err := runJUnitCommand(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	checkOriginalEmptyMarker(t, dir, expectAdapter)
	if expectHelperLog != "" {
		log, err := os.ReadFile(filepath.Join(dir, "bridge-helper.log"))
		if err != nil || !strings.Contains(string(log), expectHelperLog) {
			t.Fatalf("Go helper log missing %q: %q, %v", expectHelperLog, log, err)
		}
	}
	omissions := readOriginalEmptyOmissions(t, dir, expectOmissions)
	return events, omissions
}

func checkOriginalEmptyMarker(t *testing.T, dir string, expected bool) {
	t.Helper()
	marker, err := os.ReadFile(filepath.Join(dir, "original-setup-observed"))
	if expected && (err != nil || string(marker) != "original EmptyTest setup invoked adapter") {
		t.Fatalf("original setup did not invoke adapter: %q, %v", marker, err)
	}
	if !expected && !os.IsNotExist(err) {
		t.Fatalf("adapter unexpectedly reached success marker: %q, %v", marker, err)
	}
}

func readOriginalEmptyOmissions(t *testing.T, dir string, expected bool) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "bridge-omissions.json"))
	if !expected {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("unexpected Go helper omissions: %q, %v", data, err)
	}
	if err != nil {
		t.Fatalf("read Go helper omissions: %v", err)
	}
	var omissions []string
	if err := json.Unmarshal(data, &omissions); err != nil {
		t.Fatal(err)
	}
	if len(omissions) == 0 || bridgeVerdict(nil, omissions) == nil {
		t.Fatalf("Go helper did not retain incomplete projection: %v", omissions)
	}
	return omissions
}

func checkOriginalEmptyControls(t *testing.T, adapter, classes string, deps []string, basedir, binary, model, display string) {
	t.Helper()
	for _, tc := range []struct {
		name, mode, stack string
		marker, omitted   bool
		missingModel      bool
	}{
		{"wrong stats", "wrong-stats", "tlc2.tool.EmptyTest.testSpec", true, true, false},
		{"missing finished", "missing-finished", "tlc2.tool.EmptyTest.testSpec", true, true, false},
		{"general error", "general", "tlc2.tool.EmptyTest.testSpec", true, true, false},
		{"wrong exit", "exit", "tlc2.tool.liveness.ModelCheckerTestCase.assertExitStatus", true, true, false},
		{"malformed records", "malformed", "tlc2.tool.liveness.ModelCheckerTestCase.setUp", false, true, false},
		{"Go process unavailable", "success", "tlc2.tool.liveness.ModelCheckerTestCase.setUp", false, false, false},
		{"Go nonzero exit", "success", "tlc2.tool.liveness.ModelCheckerTestCase.setUp", false, false, true},
		{"missing success diagnostic", "missing-success", "tlc2.tool.liveness.ModelCheckerTestCase.setUp", false, false, false},
		{"changed success diagnostic", "wrong-success", "tlc2.tool.liveness.ModelCheckerTestCase.setUp", false, false, false},
		{"unavailable success diagnostic", "omitted-success", "tlc2.tool.liveness.ModelCheckerTestCase.setUp", false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			goBinary, goModel := binary, model
			var expectHelperLog string
			if tc.missingModel {
				goModel = t.TempDir()
				checkOriginalEmptyNonzeroExit(t, goBinary, goModel)
				expectHelperLog = "Go invocation failed: exit status 255"
			} else if tc.mode == "missing-success" {
				expectHelperLog = "incomplete Empty recorder observations"
			} else if tc.mode == "wrong-success" {
				expectHelperLog = "success observation does not match tool message"
			} else if tc.mode == "omitted-success" {
				expectHelperLog = "incomplete Empty recorder observation: observed-fingerprint-probability-unavailable"
			} else if !tc.omitted {
				goBinary = filepath.Join(t.TempDir(), "missing-tla-go")
			}
			events, _ := runOriginalEmpty(t, adapter, classes, deps, basedir, goBinary, goModel, tc.mode, tc.marker, tc.omitted, expectHelperLog)
			if !tc.marker {
				checkOriginalEmptySetupFailure(t, events, display)
				if tc.missingModel && !strings.Contains(events[1].value, "Go helper failed") {
					t.Fatalf("expected Go helper setup failure; events %+v", events)
				}
				return
			}
			if !validBridgeFailure(events, display) || !strings.Contains(events[1].value, tc.stack) {
				t.Fatalf("expected upstream failure in %s; events %+v", tc.stack, events)
			}
		})
	}
}

func checkOriginalEmptyNonzeroExit(t *testing.T, binary, model string) {
	t.Helper()
	_, err := boundedOriginalEmptyRun(binary, model)
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() <= 0 {
		t.Fatalf("expected a real nonzero Go CLI exit for missing model: %v", err)
	}
}

// JUnit still runs @After after @Before fails. With the unmodified upstream
// setup, an adapter failure produces both setup and exit-status failures.
func checkOriginalEmptySetupFailure(t *testing.T, events []executionEvent, display string) {
	t.Helper()
	if len(events) != 5 || events[0] != (executionEvent{"started", display}) || events[1].kind != "failure" || events[2].kind != "failure" || events[3] != (executionEvent{"finished", display}) || events[4] != (executionEvent{"summary", "1,0,2"}) || !strings.Contains(events[1].value, "tlc2.tool.liveness.ModelCheckerTestCase.setUp") || !strings.Contains(events[2].value, "tlc2.tool.liveness.ModelCheckerTestCase.assertExitStatus") {
		t.Fatalf("expected original setup and teardown failures; events %+v", events)
	}
}
