package upstreamjunit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestValidateExecution(t *testing.T) {
	pass := []executionEvent{{"started", "m(C)"}, {"finished", "m(C)"}, {"summary", "1,0,0"}}
	tests := []struct {
		name   string
		events []executionEvent
		ok     bool
	}{
		{"pass", pass, true},
		{"ignored", []executionEvent{{"ignored", "m(C)"}, {"summary", "0,1,0"}}, false},
		{"initialization error", []executionEvent{{"started", "initializationError(C)"}, {"failure", "bad"}, {"finished", "initializationError(C)"}, {"summary", "1,0,1"}}, false},
		{"assertion failure", []executionEvent{{"started", "m(C)"}, {"failure", "assertion"}, {"finished", "m(C)"}, {"summary", "1,0,1"}}, false},
		{"assumption", []executionEvent{{"started", "m(C)"}, {"assumption", "skipped"}, {"finished", "m(C)"}, {"summary", "1,0,0"}}, false},
		{"missing", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if (validateExecution(tt.events, "m(C)") == nil) != tt.ok {
				t.Fatal(tt.events)
			}
		})
	}
	if _, err := parseExecution("EXECUTION\tbogus\tYQ==\n"); err == nil {
		t.Fatal("accepted invalid event")
	}
}

func TestBridgeFailureClassification(t *testing.T) {
	display := "observations(Bridge)"
	failure := func(value string) []executionEvent {
		return []executionEvent{{"started", display}, {"failure", value}, {"finished", display}, {"summary", "1,0,1"}}
	}
	for _, tc := range []struct {
		name   string
		events []executionEvent
		want   executionOutcome
	}{
		{"assertion", failure("Bridge$UpstreamAssertionFailure\nBridge$UpstreamAssertionFailure: cause"), executionUpstreamAssertion},
		{"other exception", failure("java.lang.IllegalArgumentException\ninvalid record"), executionOtherFailure},
		{"incomplete marker", failure("Bridge$UpstreamAssertionFailure\n"), executionOtherFailure},
		{"missing summary", failure("Bridge$UpstreamAssertionFailure\nBridge$UpstreamAssertionFailure: cause")[:3], executionInvalid},
		{"wrong method", []executionEvent{{"started", "other(Bridge)"}, {"failure", "Bridge$UpstreamAssertionFailure\nBridge$UpstreamAssertionFailure"}, {"finished", "other(Bridge)"}, {"summary", "1,0,1"}}, executionInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyBridgeExecution(tc.events); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
	for _, output := range []string{"EXECUTION\tfailure\t%%%\n", "EXECUTION\tfailure\n", "EXECUTION\tunknown\tYQ==\n"} {
		if _, err := parseExecution(output); err == nil {
			t.Fatalf("accepted malformed output %q", output)
		}
	}
}

func TestJUnitProcessFailures(t *testing.T) {
	t.Run("start", func(t *testing.T) {
		ctx := context.Background()
		if _, err := runJUnitCommand(ctx, exec.CommandContext(ctx, filepath.Join(t.TempDir(), "missing"))); err == nil {
			t.Fatal("accepted start failure")
		}
	})
	t.Run("exit", func(t *testing.T) {
		ctx := context.Background()
		if _, err := runJUnitCommand(ctx, exec.CommandContext(ctx, "sh", "-c", "exit 7")); err == nil {
			t.Fatal("accepted process failure")
		}
	})
	t.Run("timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		if _, err := runJUnitCommand(ctx, exec.CommandContext(ctx, "sleep", "5")); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected timeout, got %v", err)
		}
	})
}

func TestJUnitExecutionFixture(t *testing.T) {
	junit, hamcrest := os.Getenv("TLA_GO_JUNIT_JUNIT"), os.Getenv("TLA_GO_JUNIT_HAMCREST")
	if junit == "" || hamcrest == "" {
		t.Skip("set JUnit and Hamcrest jar paths")
	}
	deps := []string{junit, hamcrest}
	for i, hash := range []string{junitSHA, hamcrestSHA} {
		if err := verifyDependency(deps[i], hash); err != nil {
			t.Fatal(err)
		}
		deps[i], _ = filepath.Abs(deps[i])
	}
	dir := t.TempDir()
	files := map[string]string{"Execution.java": executionJava, "Fixture.java": `import org.junit.*; public class Fixture { @Before public void setup() { System.setProperty("fixture.setup", "yes"); } @Test public void pass() { Assert.assertEquals("yes", System.getProperty("fixture.setup")); } @Test public void fail() { Assert.fail("expected failure"); } @Ignore @Test public void ignored() {} @Test public void assumption() { Assume.assumeTrue(false); } @Test public void invalid(int x) {} }`}
	// The invalid method makes the whole runner invalid, so compile a separate broken class.
	files["Fixture.java"] = strings.Replace(files["Fixture.java"], " @Test public void invalid(int x) {}", "", 1)
	files["Broken.java"] = `public class Broken { @org.junit.Test public void bad(int x) {} }`
	var paths []string
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	args := append([]string{"-cp", strings.Join(deps, string(os.PathListSeparator)), "-d", dir}, paths...)
	if out, err := exec.Command("javac", args...).CombinedOutput(); err != nil {
		t.Fatalf("javac: %v %s", err, out)
	}
	for _, tc := range []struct {
		class, method string
		ok            bool
	}{{"Fixture", "pass", true}, {"Fixture", "fail", false}, {"Fixture", "ignored", false}, {"Fixture", "assumption", false}, {"Broken", "bad", false}, {"Fixture", "missing", false}} {
		t.Run(tc.class+"/"+tc.method, func(t *testing.T) {
			events, err := executeJUnit(dir, deps, dir, dir, tc.class, tc.method)
			if err != nil {
				t.Fatal(err)
			}
			if (validateExecution(events, tc.method+"("+tc.class+")") == nil) != tc.ok {
				t.Fatal(events)
			}
		})
	}
}

// The entire CLI response is intentional: partial diagnostics or additional output
// cannot be mistaken for this one known unsupported invocation.
const periodicCoverageRejection = `periodic -coverage reports are not yet supported; use 0
Usage: tla-go [-DTLA-Library=paths] [-metadir dir] [-config file.cfg] [-deadlock] [-seed integer] [-aril integer] [-simulate [num=N,file=prefix]] [-dfid N] [-continue] [-depth N] [-workers N|auto] [-checkpoint minutes] [-coverage 0] [-maxSetSize N] [-fp N] [-tool] [-nowarning] [-view] [-dump [dot[,colorize][,actionlabels]] file] [-debug-liveness] [-modern-random-subsets] <file.tla|file.cfg|glob>...
       tla-go -translate [pcal.trans options] file.tla
       tla-go -sany-metadata [-I dir] file.tla
       tla-go -version

Experimental: supports a subset of TLA+ model checking and simulation.
-dfid N checks with depth-first iterative deepening up to depth N.
-continue reports supported violations and continues exploration.
-translate, which must come first, translates PlusCal like Java's pcal.trans; tla-go -translate -help lists its options.
-sany-metadata emits a partial SANY XML projection.
-debug-liveness writes JSON debugging events to stderr.
-modern-random-subsets selects the newer TLC random-subset algorithm.
Java TLC options: [-difftrace] [-terse] [-userFile file] [-cleanup] [-fpmem num] [-fpbits num] [-gzip] [-modelcheck] [-debug] [-generateSpecTE [nomonolith]]
TLA_GO_DEBUG_RECORDER=FILE writes partial recorder diagnostics to a new file in -tool mode.
`

// Match the complete observed Test10 stream, including severity, ordering, and
// finish record. Variable process metadata is restricted to its expected form.
var coverageShapeRejection = func() *regexp.Regexp {
	pattern := `\A@!@!@STARTMSG 2262:0 @!@!@\ntla-go [^\n]+\n@!@!@ENDMSG 2262 @!@!@\n` +
		`@!@!@STARTMSG 2187:0 @!@!@\nRunning breadth-first search Model-Checking with fp 0 and seed 0 with 1 worker on [0-9]+ cores\.\n@!@!@ENDMSG 2187 @!@!@\n` +
		`@!@!@STARTMSG 2220:0 @!@!@\nStarting SANY\.\.\.\n@!@!@ENDMSG 2220 @!@!@\n` +
		`@!@!@STARTMSG 2219:0 @!@!@\nSANY finished\.\n@!@!@ENDMSG 2219 @!@!@\n` +
		`@!@!@STARTMSG 2185:0 @!@!@\nStarting\.\.\. \([0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}\)\n@!@!@ENDMSG 2185 @!@!@\n`
	for i := 1; i <= 31; i++ {
		space := " "
		if i == 27 {
			space = "  "
		}
		pattern += fmt.Sprintf(`"Test %d%sOK"  TRUE\n`, i, space)
	}
	pattern += `@!@!@STARTMSG 1000:1 @!@!@\n-coverage does not yet support this model's action or expression forms\n@!@!@ENDMSG 1000 @!@!@\n` +
		`@!@!@STARTMSG 2186:0 @!@!@\nFinished in [0-9]+(?:ms|s) at \([0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}\)\n@!@!@ENDMSG 2186 @!@!@\n\z`
	return regexp.MustCompile(pattern)
}()

func runBridgeModel(run func(bool) (Result, error)) (Result, []bridgeRecord, []string, error) {
	result, err := run(true)
	if err != nil {
		return result, nil, nil, err
	}
	if result.TimedOut {
		return result, nil, nil, fmt.Errorf("coverage invocation timed out")
	}
	fallbackReason := ""
	if result.ExitCode == 255 {
		switch {
		case result.Output == periodicCoverageRejection:
			fallbackReason = "periodic -coverage 1 unsupported"
		case coverageShapeRejection.MatchString(result.Output):
			fallbackReason = "coverage model shape unsupported"
		}
	}
	fallback := fallbackReason != ""
	if fallback {
		result, err = run(false)
		if err != nil {
			return result, nil, nil, err
		}
	}
	if result.TimedOut || result.ExitCode != 0 {
		return result, nil, nil, fmt.Errorf("unexpected Go invocation: timeout=%t exit=%d output=%s", result.TimedOut, result.ExitCode, result.Output)
	}
	records, omissions, err := projectBridge(result.Output)
	if err != nil {
		return result, nil, nil, fmt.Errorf("invalid Go tool output: %w", err)
	}
	if fallback {
		omissions = append(omissions, "coverage-enabled invocation rejected: "+fallbackReason)
	}
	return result, records, omissions, nil
}

func TestBridgeCoverageFallbackClassification(t *testing.T) {
	valid := "@!@!@STARTMSG 2199:0 @!@!@\n2 states generated, 1 distinct states found, 0 states left on queue.\n@!@!@ENDMSG 2199 @!@!@\n"
	shape := msg("2262", "tla-go dev") +
		msg("2187", "Running breadth-first search Model-Checking with fp 0 and seed 0 with 1 worker on 8 cores.") +
		msg("2220", "Starting SANY...") + msg("2219", "SANY finished.") +
		msg("2185", "Starting... (2026-11-01 13:36:31)")
	for i := 1; i <= 31; i++ {
		space := " "
		if i == 27 {
			space = "  "
		}
		shape += fmt.Sprintf("\"Test %d%sOK\"  TRUE\n", i, space)
	}
	shape += "@!@!@STARTMSG 1000:1 @!@!@\n-coverage does not yet support this model's action or expression forms\n@!@!@ENDMSG 1000 @!@!@\n" +
		msg("2186", "Finished in 3ms at (2026-11-01 13:36:31)")
	for _, tc := range []struct {
		name      string
		first     Result
		firstErr  error
		second    Result
		secondErr error
		calls     int
		ok        bool
	}{
		{"expected rejection", Result{ExitCode: 255, Output: periodicCoverageRejection}, nil, Result{Output: valid}, nil, 2, true},
		{"shape rejection", Result{ExitCode: 255, Output: shape}, nil, Result{Output: valid}, nil, 2, true},
		{"shape wrong severity", Result{ExitCode: 255, Output: strings.Replace(shape, "1000:1", "1000:0", 1)}, nil, Result{}, nil, 1, false},
		{"shape wrong text", Result{ExitCode: 255, Output: strings.Replace(shape, "expression forms", "expression form", 1)}, nil, Result{}, nil, 1, false},
		{"shape extra error", Result{ExitCode: 255, Output: shape + msg("1000", "unknown error")}, nil, Result{}, nil, 1, false},
		{"shape truncated finish", Result{ExitCode: 255, Output: shape[:len(shape)-2]}, nil, Result{}, nil, 1, false},
		{"shape malformed marker", Result{ExitCode: 255, Output: shape + "@!@!@STARTMSG"}, nil, Result{}, nil, 1, false},
		{"shape wrong exit", Result{ExitCode: 1, Output: shape}, nil, Result{}, nil, 1, false},
		{"shape timeout", Result{TimedOut: true, ExitCode: 255, Output: shape}, nil, Result{}, nil, 1, false},
		{"coverage succeeds", Result{Output: valid}, nil, Result{}, nil, 1, true},
		{"malformed marker", Result{Output: valid + "@!@!@BROKEN"}, nil, Result{}, nil, 1, false},
		{"truncated message", Result{Output: "@!@!@STARTMSG 2199:0 @!@!@\npartial"}, nil, Result{}, nil, 1, false},
		{"truncated rejection", Result{ExitCode: 255, Output: periodicCoverageRejection[:len(periodicCoverageRejection)-1]}, nil, Result{}, nil, 1, false},
		{"rejection with malformed output", Result{ExitCode: 255, Output: periodicCoverageRejection + "@!@!@STARTMSG"}, nil, Result{}, nil, 1, false},
		{"unknown error", Result{ExitCode: 255, Output: "unknown error\n"}, nil, Result{}, nil, 1, false},
		{"wrong rejection exit", Result{ExitCode: 1, Output: periodicCoverageRejection}, nil, Result{}, nil, 1, false},
		{"rejection with success exit", Result{Output: periodicCoverageRejection}, nil, Result{}, nil, 1, false},
		{"unexpected exit with valid output", Result{ExitCode: 7, Output: valid}, nil, Result{}, nil, 1, false},
		{"timeout error", Result{TimedOut: true, ExitCode: 255, Output: periodicCoverageRejection}, context.DeadlineExceeded, Result{}, nil, 1, false},
		{"timeout flag", Result{TimedOut: true, ExitCode: 255, Output: periodicCoverageRejection}, nil, Result{}, nil, 1, false},
		{"launch error", Result{}, errors.New("launch failed"), Result{}, nil, 1, false},
		{"fallback malformed", Result{ExitCode: 255, Output: periodicCoverageRejection}, nil, Result{Output: "@!@!@STARTMSG"}, nil, 2, false},
		{"fallback unexpected exit", Result{ExitCode: 255, Output: periodicCoverageRejection}, nil, Result{ExitCode: 7, Output: valid}, nil, 2, false},
		{"fallback timeout", Result{ExitCode: 255, Output: periodicCoverageRejection}, nil, Result{TimedOut: true}, context.DeadlineExceeded, 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			_, _, omissions, err := runBridgeModel(func(coverage bool) (Result, error) {
				calls++
				if coverage != (calls == 1) {
					t.Fatal("wrong coverage flag")
				}
				if calls == 1 {
					return tc.first, tc.firstErr
				}
				return tc.second, tc.secondErr
			})
			if calls != tc.calls || (err == nil) != tc.ok {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
			if tc.ok && (!strings.Contains(strings.Join(omissions, ";"), "do not establish recorder") || strings.Contains(strings.Join(omissions, ";"), "coverage-enabled invocation rejected") != (tc.calls == 2)) {
				t.Fatalf("omissions=%v", omissions)
			}
		})
	}
}

func TestBridgeCoverageSuccessRetainsObservations(t *testing.T) {
	output := msg("2201", "start") + msg("2221", "  line 1: 0") + msg("2202", "end")
	calls := 0
	result, records, omissions, err := runBridgeModel(func(coverage bool) (Result, error) {
		calls++
		return Result{Output: output}, nil
	})
	if err != nil || calls != 1 || result.Output != output {
		t.Fatalf("calls=%d result=%+v err=%v", calls, result, err)
	}
	zero, err := ZeroCoverage(result)
	if err != nil || len(zero) != 1 || len(records) != 1 || records[0].code != "2221" {
		t.Fatalf("coverage=%v records=%v omissions=%v err=%v", zero, records, omissions, err)
	}
}

func TestUpstreamJUnitExecution(t *testing.T) {
	if os.Getenv("TLA_GO_JUNIT_EXECUTION") != "1" {
		t.Skip("set TLA_GO_JUNIT_EXECUTION=1")
	}
	root, jar, junit, hamcrest := os.Getenv("TLA_GO_JUNIT_ROOT"), os.Getenv("TLA_GO_JUNIT_JAR"), os.Getenv("TLA_GO_JUNIT_JUNIT"), os.Getenv("TLA_GO_JUNIT_HAMCREST")
	for _, p := range []*string{&root, &jar, &junit, &hamcrest} {
		if *p == "" {
			t.Fatal("set root and all three jar paths")
		}
		var err error
		*p, err = filepath.Abs(*p)
		if err != nil {
			t.Fatal(err)
		}
	}
	discoveryCtx, discoveryCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer discoveryCancel()
	if err := verifyInputs(discoveryCtx, root, jar); err != nil {
		t.Fatal(err)
	}
	for _, dep := range []struct{ path, hash string }{{junit, junitSHA}, {hamcrest, hamcrestSHA}} {
		if err := verifyDependency(dep.path, dep.hash); err != nil {
			t.Fatal(err)
		}
	}
	temp := t.TempDir()
	// Discovery compiles only tracked sources and establishes that this exact leaf exists.
	coverageClass := "tlc2.tool.coverage.BCoverageTest"
	statisticsClass := "tlc2.tool.coverage.CoverageStatisticsTest"
	gCoverageClass := "tlc2.tool.coverage.GCoverageTest"
	loopClass := "tlc2.tool.liveness.LoopTestWeakFair"
	records, err := discover(discoveryCtx, root, jar, junit, hamcrest, temp, []string{"tlc2.tool.suite.Test10", "tlc2.tool.EmptyTest", coverageClass, statisticsClass, gCoverageClass, loopClass})
	if err != nil {
		t.Fatal(err)
	}
	want := DiscoveryRecord{"case", "tlc2.tool.suite.Test10", "testSpec(tlc2.tool.suite.Test10)", ""}
	empty := DiscoveryRecord{"case", "tlc2.tool.EmptyTest", "testSpec(tlc2.tool.EmptyTest)", ""}
	coverage := DiscoveryRecord{"case", coverageClass, "testSpec(" + coverageClass + ")", ""}
	statistics := DiscoveryRecord{"case", statisticsClass, "testSpec(" + statisticsClass + ")", ""}
	gCoverage := DiscoveryRecord{"case", gCoverageClass, "testSpec(" + gCoverageClass + ")", ""}
	loop := DiscoveryRecord{"case", loopClass, "testSpec(" + loopClass + ")", ""}
	if len(records) != 6 || !slices.Contains(records, want) || !slices.Contains(records, empty) || !slices.Contains(records, coverage) || !slices.Contains(records, statistics) || !slices.Contains(records, gCoverage) || !slices.Contains(records, loop) {
		t.Fatalf("unsupported lifecycle or discovery: %+v", records)
	}
	classes := filepath.Join(temp, "classes")
	source := filepath.Join(temp, "Execution.java")
	if err := os.WriteFile(source, []byte(executionJava), 0o600); err != nil {
		t.Fatal(err)
	}
	deps := []string{jar, junit, hamcrest}
	bridgeSource := filepath.Join(temp, "Bridge.java")
	if err := os.WriteFile(bridgeSource, []byte(bridgeJava), 0o600); err != nil {
		t.Fatal(err)
	}
	goEmptySource := filepath.Join(temp, "GoEmptyTest.java")
	if err := os.WriteFile(goEmptySource, []byte(goEmptyTestJava), 0o600); err != nil {
		t.Fatal(err)
	}
	goSuiteSource := filepath.Join(temp, "GoSuiteTest10.java")
	if err := os.WriteFile(goSuiteSource, []byte(goSuiteTest10Java), 0o600); err != nil {
		t.Fatal(err)
	}
	recorderSource := filepath.Join(temp, "RecorderFixture.java")
	if err := os.WriteFile(recorderSource, []byte(recorderFixtureJava), 0o600); err != nil {
		t.Fatal(err)
	}
	goBCoverageSource := filepath.Join(temp, "GoBCoverageTest.java")
	if err := os.WriteFile(goBCoverageSource, []byte(goBCoverageTestJava), 0o600); err != nil {
		t.Fatal(err)
	}
	goStatisticsSource := filepath.Join(temp, "GoCoverageStatisticsTest.java")
	if err := os.WriteFile(goStatisticsSource, []byte(goCoverageStatisticsTestJava), 0o600); err != nil {
		t.Fatal(err)
	}
	goGCoverageSource := filepath.Join(temp, "GoGCoverageTest.java")
	if err := os.WriteFile(goGCoverageSource, []byte(goGCoverageTestJava), 0o600); err != nil {
		t.Fatal(err)
	}
	goLoopSource := filepath.Join(temp, "GoLoopTestWeakFair.java")
	if err := os.WriteFile(goLoopSource, []byte(goLoopTestWeakFairJava), 0o600); err != nil {
		t.Fatal(err)
	}
	captureSource := filepath.Join(temp, "CaptureRecorder.java")
	if err := os.WriteFile(captureSource, []byte(captureRecorderJava), 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"-proc:none", "-cp", strings.Join(append([]string{classes}, deps...), string(os.PathListSeparator)), "-d", classes, source, bridgeSource, goEmptySource, goSuiteSource, recorderSource, goBCoverageSource, goGCoverageSource, goStatisticsSource, goLoopSource, captureSource}
	if out, err := exec.Command("javac", args...).CombinedOutput(); err != nil {
		t.Fatalf("javac: %v\n%s", err, out)
	}
	// The upstream isolated class loader resolves standard modules relative to classes.
	extract := exec.Command("jar", "xf", jar, "tla2sany/StandardModules")
	extract.Dir = classes
	if out, err := extract.CombinedOutput(); err != nil {
		t.Fatalf("extract standard modules: %v\n%s", err, out)
	}
	if err := copyModels(root, temp); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(temp, "run")
	if err := os.Mkdir(runDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(temp, "java"), filepath.Join(runDir, "test-model")); err != nil {
		t.Fatal(err)
	}
	checkTypedRecorderFixture(t, classes, deps, runDir)
	events, err := executeJUnit(classes, deps, runDir, runDir, want.Class, "testSpec")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateExecution(events, want.Display); err != nil {
		t.Fatal(err)
	}
	t.Logf("Java JUnit pass: %s#testSpec (inherited setup, assertion and teardown); events=%v", want.Class, events)
	// TLC names its states directory at one-second resolution. Keep each JVM's
	// working directory separate even when both cases finish in the same second.
	emptyDir := filepath.Join(temp, "empty-run")
	if err := os.Mkdir(emptyDir, 0o700); err != nil {
		t.Fatal(err)
	}
	emptyEvents, err := executeJUnit(classes, deps, emptyDir, runDir, empty.Class, "testSpec")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateExecution(emptyEvents, empty.Display); err != nil {
		t.Fatal(err)
	}
	t.Logf("Java JUnit pass: %s; events=%v", empty.Display, emptyEvents)
	coverageDir := filepath.Join(temp, "coverage-run")
	if err := os.Mkdir(coverageDir, 0o700); err != nil {
		t.Fatal(err)
	}
	coverageEvents, err := executeJUnit(classes, deps, coverageDir, runDir, coverage.Class, "testSpec")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateExecution(coverageEvents, coverage.Display); err != nil {
		t.Fatal(err)
	}
	t.Logf("Java JUnit pass: %s; events=%v", coverage.Display, coverageEvents)
	checkJavaStatisticsJUnit(t, classes, deps, temp, runDir, statistics)
	checkJavaGCoverageJUnit(t, classes, deps, temp, runDir, gCoverage)
	checkJavaLoopJUnit(t, classes, deps, temp, runDir, loop)
	binary := filepath.Join(temp, "tla-go")
	if out, err := exec.Command("go", "build", "-o", binary, "../../cmd/tla-go").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	c := Case{Name: want.Class, Model: "test10", Path: "suite", Coverage: true, Dump: true, IDs: []string{"2199", "2190"}, PresenceIDs: []string{"2186", "1000"}}
	_, _, differences, err := RunCase(root, jar, binary, c, 30*time.Second)
	if err != nil {
		t.Fatalf("partial Go diagnostic infrastructure error: %v", err)
	}
	if len(differences) != 0 {
		t.Fatalf("Go mismatch in partial diagnostic for %s: %v", want.Display, differences)
	}

	// A separate isolated Go copy supplies the adapter; preserve every omission.
	goDir := filepath.Join(temp, "go", "suite")
	goResult, projected, omissions, err := runBridgeModel(func(coverage bool) (Result, error) {
		dump := "bridge-no-coverage.dot"
		if coverage {
			dump = "bridge.dot"
		}
		return runCommand(goDir, 30*time.Second, binary, caseArgs(c, coverage, filepath.Join(temp, dump))...)
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Go bridge invocation omissions: %v", omissions)

	if err := os.WriteFile(filepath.Join(runDir, "bridge-target"), []byte("Test10"), 0o600); err != nil {
		t.Fatal(err)
	}
	runBridge := func(records []bridgeRecord) (executionOutcome, error) {
		data, e := encodeBridge(records)
		if e != nil {
			return executionInvalid, e
		}
		if e = os.WriteFile(filepath.Join(runDir, "bridge-records"), []byte(data), 0o600); e != nil {
			return executionInvalid, e
		}
		ev, e := executeJUnit(classes, deps, runDir, runDir, "Bridge", "observations")
		if e != nil {
			return executionInvalid, e
		}
		outcome := classifyBridgeExecution(ev)
		if outcome == executionInvalid || outcome == executionOtherFailure {
			return outcome, fmt.Errorf("unexpected bridge events: %+v", ev)
		}
		return outcome, nil
	}
	// The original method may fail or succeed, but neither outcome is a full pass.
	assertionOutcome, assertionErr := runBridge(projected)
	if assertionErr != nil {
		t.Fatal(assertionErr)
	}
	t.Logf("Go assertion-method result: %v; exit=%d; unrepresented observations=%v", assertionOutcome, goResult.ExitCode, omissions)
	t.Logf("No Go JUnit pass: %v", bridgeVerdict(fmt.Errorf("outcome=%v", assertionOutcome), omissions))
	checkBridgeLifecycle(t, classes, deps, runDir, "GoSuiteTest10", "SuiteTestCase.java:73", projected, goResult.ExitCode)
	// A malformed recorder input fails inside the bridge, before testSpec runs.
	if err := os.WriteFile(filepath.Join(runDir, "bridge-records"), []byte("not-a-code\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	badEvents, err := executeJUnit(classes, deps, runDir, runDir, "Bridge", "observations")
	if err != nil || classifyBridgeExecution(badEvents) != executionOtherFailure {
		t.Fatalf("malformed bridge input: events=%v error=%v", badEvents, err)
	}
	// Complete synthetic records exercise the real upstream assertions, including coverage.
	good := []bridgeRecord{{"2186", []string{"1ms"}}, {"2199", []string{"2", "1", "0"}}, {"2190", []string{"1", ""}}, {"2221", []string{"fixture", "1"}}}
	if outcome, err := runBridge(good); err != nil || outcome != executionPassed {
		t.Fatalf("synthetic baseline: outcome=%v error=%v", outcome, err)
	}
	for name, bad := range map[string][]bridgeRecord{
		"wrong stats":      {{"2186", []string{"1ms"}}, {"2199", []string{"3", "1", "0"}}, {"2190", []string{"1", ""}}},
		"wrong init":       {{"2186", []string{"1ms"}}, {"2199", []string{"2", "1", "0"}}, {"2190", []string{"2", ""}}},
		"missing finished": {{"2199", []string{"2", "1", "0"}}, {"2190", []string{"1", ""}}},
		"zero coverage":    append(append([]bridgeRecord{}, good...), bridgeRecord{"2221", []string{"uncovered", "0"}}),
		"general error":    append(append([]bridgeRecord{}, good...), bridgeRecord{"1000", []string{"error"}}),
	} {
		if outcome, err := runBridge(bad); err != nil || outcome != executionUpstreamAssertion {
			t.Errorf("%s: expected original assertion failure, outcome=%v error=%v", name, outcome, err)
		}
	}
	if err := os.WriteFile(filepath.Join(runDir, "bridge-target"), []byte("EmptyTest"), 0o600); err != nil {
		t.Fatal(err)
	}
	emptyArgs := emptyInvocation()
	emptyResult, err := runCommand(filepath.Join(temp, "go"), 30*time.Second, binary, emptyArgs...)
	if err != nil {
		t.Fatal(err)
	}
	disabledArgs := slices.Clone(emptyArgs)
	for i := range disabledArgs {
		if disabledArgs[i] == "-coverage" {
			disabledArgs[i+1] = "0"
		}
	}
	disabledResult, err := runCommand(filepath.Join(temp, "go"), 30*time.Second, binary, disabledArgs...)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := projectEmpty(disabledArgs, disabledResult); err == nil {
		t.Fatal("accepted disabled coverage")
	}
	emptyRecords, emptyOmissions, err := projectEmpty(emptyArgs, emptyResult)
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := runBridge(emptyRecords)
	if err != nil || outcome != executionPassed {
		t.Fatalf("EmptyTest assertion: %v %v", outcome, err)
	}
	t.Logf("EmptyTest original assertion matches projected Go observations; args=%v exit=%d omissions=%v; no Go JUnit pass: %v", emptyArgs, emptyResult.ExitCode, emptyOmissions, bridgeVerdict(nil, emptyOmissions))
	goodEmpty := []bridgeRecord{{"2186", []string{"1ms"}}, {"2199", []string{"0", "0", "0"}}}
	for name, bad := range map[string][]bridgeRecord{
		"wrong stats":      {{"2186", []string{"1ms"}}, {"2199", []string{"1", "0", "0"}}},
		"missing finished": {{"2199", []string{"0", "0", "0"}}},
		"general":          append(slices.Clone(goodEmpty), bridgeRecord{"1000", []string{"error"}}),
	} {
		if got, e := runBridge(bad); e != nil || got != executionUpstreamAssertion {
			t.Errorf("%s: outcome=%v error=%v", name, got, e)
		}
	}
	checkBridgeLifecycle(t, classes, deps, runDir, "GoEmptyTest", "EmptyTest.java:48", emptyRecords, emptyResult.ExitCode)
	checkOriginalEmptySetup(t, classes, deps, runDir, binary, filepath.Join(temp, "go"))
	checkOriginalCoverageSetup(t, classes, deps, runDir, binary, filepath.Join(temp, "go", "coverage"))
	checkOriginalEmptyFullRecorder(t, root, classes, deps, runDir, binary, filepath.Join(temp, "go"))
	checkBridgeBCoverage(t, binary, filepath.Join(temp, "go", "coverage"), classes, deps, runDir, runBridge)
	checkCoverageStatisticsBridge(t, binary, filepath.Join(temp, "go", "coverage"), classes, deps, runDir, runBridge)
	checkGCoverageBridge(t, binary, filepath.Join(temp, "go", "coverage"), classes, deps, runDir, runBridge)
	checkLoopRecorderBridge(t, binary, filepath.Join(temp, "go", "Loop"), classes, deps, runDir)
}
