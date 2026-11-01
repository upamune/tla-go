package upstreamjunit

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/upamune/tla-go/internal/javatmp"
	"github.com/upamune/tla-go/internal/recorderdebug"
)

//go:embed OriginalCoverageTLC.java
var originalCoverageTLCJava string

//go:embed OriginalCoverageArgsFixture.java
var originalCoverageArgsFixtureJava string

// The helper runs during the original upstream setUp, after it installs its
// recorder. Only its typed events are fed into the original assertion.
func TestOriginalCoverageGoHelper(t *testing.T) {
	if os.Getenv("TLA_GO_ORIGINAL_COVERAGE_HELPER") != "1" {
		return
	}
	if err := originalCoverageGoHelper(); err != nil {
		t.Fatal(err)
	}
}

func coverageOriginalInvocation(target string) ([]string, error) {
	if target != "B" && target != "G" {
		return nil, fmt.Errorf("unsupported coverage target %q", target)
	}
	dump := filepath.Join("${metadir}", "tlc2.tool.coverage."+target+"CoverageTest.dot")
	return []string{"-tool", "-deadlock", "-fp", "0", "-coverage", "1", "-workers", "1", "-checkpoint", "0", "-dump", "dot", dump, "-coverage", "9999", target}, nil
}

func originalCoverageGoHelper() error {
	binary, model := os.Getenv("TLA_GO_ORIGINAL_COVERAGE_BINARY"), os.Getenv("TLA_GO_ORIGINAL_COVERAGE_MODEL")
	target, mode := os.Getenv("TLA_GO_ORIGINAL_COVERAGE_TARGET"), os.Getenv("TLA_GO_ORIGINAL_COVERAGE_MODE")
	if binary == "" || model == "" || !slices.Contains([]string{"success", "bad-coverage", "missing-coverage", "exit"}, mode) {
		return fmt.Errorf("missing or unsupported coverage helper configuration")
	}
	args, err := coverageOriginalInvocation(target)
	if err != nil {
		return err
	}
	recorder := filepath.Join(model, "coverage-"+target+"-"+mode+"-recorder.jsonl")
	result, err := runCommandWithEnv(model, 30*time.Second, []string{"TLA_GO_DEBUG_RECORDER=" + recorder}, binary, args...)
	if err != nil || result.TimedOut || result.ExitCode != 0 {
		return fmt.Errorf("Go coverage invocation: timeout=%t exit=%d error=%v output=%s", result.TimedOut, result.ExitCode, err, result.Output)
	}
	messages, err := Messages(result.Output)
	if err != nil || len(messages["2201"]) != 1 || len(messages["2202"]) != 1 || len(messages["2776"]) != 0 {
		return fmt.Errorf("incomplete coverage output: %v", err)
	}
	projected, omissions, err := projectBridge(result.Output)
	if err != nil {
		return err
	}
	f, err := os.Open(recorder)
	if err != nil {
		return err
	}
	events, err := recorderdebug.Decode(f)
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		return fmt.Errorf("read recorder: %v; close: %v", err, closeErr)
	}
	records, err := coverageRecorderRecords(projected, events)
	if err != nil {
		return err
	}
	if err := changeOriginalCoverageRecords(records, mode); err != nil {
		return err
	}
	if mode == "missing-coverage" {
		records = slices.DeleteFunc(records, func(r bridgeRecord) bool { return r.code == "2221" })
	}
	encoded, err := encodeBridge(records)
	if err != nil {
		return err
	}
	if err := os.WriteFile("bridge-records", []byte(encoded), 0o600); err != nil {
		return err
	}
	omissions = append(omissions, "tool mode and incomplete recorder differ from original Java TLC setup", "metadir lifecycle and Java state files not reproduced")
	data, err := json.Marshal(omissions)
	if err != nil {
		return err
	}
	return os.WriteFile("bridge-coverage-omissions.json", data, 0o600)
}

func changeOriginalCoverageRecords(records []bridgeRecord, mode string) error {
	if mode != "bad-coverage" {
		return nil
	}
	for i := range records {
		if records[i].code == "2772" && len(records[i].values) >= 2 {
			records[i].values[1] = "999"
			return nil
		}
	}
	return fmt.Errorf("missing coverage action record")
}

func compileOriginalCoverageAdapter(t *testing.T, classes string, deps []string) string {
	t.Helper()
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "src", "tlc2")
	if err := os.MkdirAll(sourceDir, 0o700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(sourceDir, "TLC.java")
	if err := os.WriteFile(source, []byte(originalCoverageTLCJava), 0o600); err != nil {
		t.Fatal(err)
	}
	adapter := filepath.Join(dir, "classes")
	if err := os.Mkdir(adapter, 0o700); err != nil {
		t.Fatal(err)
	}
	cp := strings.Join(append([]string{classes}, deps...), string(os.PathListSeparator))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "javac", "-J-Xmx256m", "-proc:none", "-cp", cp, "-d", adapter, source)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile coverage TLC adapter: %v\n%s", err, out)
	}
	checkOriginalCoverageArguments(t, ctx, dir, adapter, cp)
	return adapter
}

func checkOriginalCoverageArguments(t *testing.T, ctx context.Context, dir, adapter, cp string) {
	t.Helper()
	source := filepath.Join(dir, "OriginalCoverageArgsFixture.java")
	if err := os.WriteFile(source, []byte(originalCoverageArgsFixtureJava), 0o600); err != nil {
		t.Fatal(err)
	}
	cp = adapter + string(os.PathListSeparator) + cp
	compile := exec.CommandContext(ctx, "javac", "-J-Xmx128m", "-proc:none", "-cp", cp, "-d", adapter, source)
	if out, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("compile coverage argument controls: %v\n%s", err, out)
	}
	cmd := exec.CommandContext(ctx, "java", "-Xmx128m", "-cp", cp, "OriginalCoverageArgsFixture")
	javatmp.Apply(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("coverage argument controls: %v\n%s", err, out)
	}
}

func checkOriginalCoverageSetup(t *testing.T, classes string, deps []string, basedir, binary, model string) {
	t.Helper()
	adapter := compileOriginalCoverageAdapter(t, classes, deps)
	for _, target := range []string{"B", "G"} {
		for _, mode := range []string{"success", "bad-coverage", "missing-coverage", "exit"} {
			t.Run(target+"/"+mode, func(t *testing.T) {
				checkOriginalCoverageCase(t, adapter, classes, deps, basedir, binary, model, target, mode)
			})
		}
	}
}

func checkOriginalCoverageCase(t *testing.T, adapter, classes string, deps []string, basedir, binary, model, target, mode string) {
	t.Helper()
	dir := t.TempDir()
	class := "tlc2.tool.coverage." + target + "CoverageTest"
	display := "testSpec(" + class + ")"
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cp := strings.Join(append([]string{adapter, classes}, deps...), string(os.PathListSeparator))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-Xmx256m", "-Dbasedir="+basedir+string(os.PathSeparator), "-Djava.io.tmpdir="+dir,
		"-Dtla.coverage.helper="+helper, "-Dtla.coverage.binary="+binary, "-Dtla.coverage.model="+model,
		"-Dtla.coverage.target="+target, "-Dtla.coverage.mode="+mode, "-cp", cp, "Execution", class, "testSpec")
	cmd.Dir = dir
	javatmp.Apply(cmd)
	events, err := runJUnitCommand(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	marker, err := os.ReadFile(filepath.Join(dir, "original-coverage-setup-observed"))
	if err != nil || string(marker) != target {
		log, _ := os.ReadFile(filepath.Join(dir, "coverage-helper.log"))
		t.Fatalf("original setup did not invoke Go adapter: %q %v; JUnit %v helper %s", marker, err, events, log)
	}
	data, err := os.ReadFile(filepath.Join(dir, "bridge-coverage-omissions.json"))
	var omissions []string
	if err != nil || json.Unmarshal(data, &omissions) != nil || bridgeVerdict(nil, omissions) == nil {
		t.Fatalf("missing incomplete bridge verdict: %q %v", data, err)
	}
	if mode == "success" {
		if err := validateExecution(events, display); err != nil {
			t.Fatalf("original coverage JUnit with Go observations: %v", err)
		}
		return
	}
	if !validBridgeFailure(events, display) {
		t.Fatalf("expected upstream JUnit assertion failure: %v", events)
	}
	if mode == "exit" && !strings.Contains(events[1].value, "ModelCheckerTestCase.assertExitStatus") {
		t.Fatalf("expected upstream teardown exit assertion: %v", events)
	}
	if mode != "exit" && !strings.Contains(events[1].value, target+"CoverageTest.testSpec") {
		t.Fatalf("expected upstream coverage assertion: %v", events)
	}
}
