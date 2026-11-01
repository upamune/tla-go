package upstreamjunit

import (
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/upamune/tla-go/internal/recorderdebug"
)

//go:embed FullRecorderCapture.java
var fullRecorderCaptureJava string

// Instrument only the recorder's entry point, then run the original JUnit test
// and inherited setup with Java TLC. The captured stream is not filtered.
func checkOriginalEmptyFullRecorder(t *testing.T, root, classes string, deps []string, basedir, binary, model string) {
	t.Helper()
	dir := t.TempDir()
	instrumented := compileFullRecorder(t, root, classes, deps, dir)
	path := filepath.Join(dir, "records")
	cp := strings.Join(append([]string{instrumented, classes}, deps...), string(os.PathListSeparator))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-Xmx256m", "-XX:+UseParallelGC", "-Dbasedir="+basedir+string(os.PathSeparator), "-Djava.io.tmpdir="+dir, "-Dtla.full.recorder="+path, "-cp", cp, "Execution", "tlc2.tool.EmptyTest", "testSpec")
	cmd.Dir = dir
	execution, err := runJUnitCommand(ctx, cmd)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateExecution(execution, "testSpec(tlc2.tool.EmptyTest)"); err != nil {
		t.Fatalf("instrumented original JUnit: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	javaRecords, err := decodeFullRecorder(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := recordCodes(javaRecords), []int{2262, 2187, 2185, 2189, 2190, 2193, 2199, 2194, 2186}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unfiltered original Java recorder calls: %v; want %v", got, want)
	}
	_, goEvents, err := boundedOriginalEmptyRunWithRecorder(binary, model)
	if err != nil {
		t.Fatal(err)
	}
	compareOriginalEmptyRecorded(t, javaRecords, goEvents)
	compareOriginalEmptyArtifacts(t, dir, model)
}

func compareOriginalEmptyArtifacts(t *testing.T, javaDir, goDir string) {
	t.Helper()
	dumps, err := filepath.Glob(filepath.Join(javaDir, "states", "*", "tlc2.tool.EmptyTest.dot"))
	if err != nil || len(dumps) != 1 {
		t.Fatalf("expected one retained Java DOT dump, got %v: %v", dumps, err)
	}
	javaDump, err := os.ReadFile(dumps[0])
	if err != nil {
		t.Fatal(err)
	}
	goDump, err := os.ReadFile(filepath.Join(goDir, "empty-bridge.dot"))
	if err != nil || !reflect.DeepEqual(javaDump, goDump) {
		t.Fatalf("empty DOT mismatch: Java %q Go %q error %v", javaDump, goDump, err)
	}
	// The upstream setup vetoes cleanup; these files persist after @After.
	for _, pattern := range []string{"*.fp", "*.st"} {
		files, err := filepath.Glob(filepath.Join(filepath.Dir(dumps[0]), pattern))
		if err != nil || len(files) == 0 {
			t.Fatalf("Java cleanup unexpectedly removed %s: %v %v", pattern, files, err)
		}
	}
}

func compileFullRecorder(t *testing.T, root, classes string, deps []string, dir string) string {
	t.Helper()
	sourcePath := filepath.Join(root, "tlatools", "org.lamport.tlatools", "test", "tlc2", "TestMPRecorder.java")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	const hook = "public void record(int code, Object... objects) {"
	if strings.Count(string(source), hook) != 1 {
		t.Fatal("unexpected upstream recorder entry point")
	}
	instrumented := strings.Replace(string(source), hook, hook+"\n        FullRecorderCapture.capture(code, objects);", 1)
	sourceDir := filepath.Join(dir, "src", "tlc2")
	outputDir := filepath.Join(dir, "classes")
	if err := os.MkdirAll(sourceDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outputDir, 0o700); err != nil {
		t.Fatal(err)
	}
	recorderSource := filepath.Join(sourceDir, "TestMPRecorder.java")
	captureSource := filepath.Join(sourceDir, "FullRecorderCapture.java")
	if err := os.WriteFile(recorderSource, []byte(instrumented), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(captureSource, []byte(fullRecorderCaptureJava), 0o600); err != nil {
		t.Fatal(err)
	}
	cp := strings.Join(append([]string{classes}, deps...), string(os.PathListSeparator))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "javac", "-J-Xmx256m", "-proc:none", "-cp", cp, "-d", outputDir, recorderSource, captureSource)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile instrumented upstream recorder: %v\n%s", err, out)
	}
	return outputDir
}

func decodeFullRecorder(data string) ([]recorderdebug.Event, error) {
	if !strings.HasSuffix(data, "\n") {
		return nil, fmt.Errorf("incomplete Java recorder capture")
	}
	var events []recorderdebug.Event
	for line := range strings.SplitSeq(strings.TrimSuffix(data, "\n"), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 2 || fields[1] != "strings" {
			return nil, fmt.Errorf("unsupported Java recorder array: %q", line)
		}
		code, err := strconv.Atoi(fields[0])
		if err != nil {
			return nil, err
		}
		event := recorderdebug.Event{Code: code}
		for _, encoded := range fields[2:] {
			value, err := base64.StdEncoding.Strict().DecodeString(encoded)
			if err != nil {
				return nil, fmt.Errorf("unsupported Java recorder value: %q", encoded)
			}
			event.Values = append(event.Values, string(value))
		}
		events = append(events, event)
	}
	return events, nil
}

func recordCodes(events []recorderdebug.Event) []int {
	codes := make([]int, 0, len(events))
	for _, event := range events {
		codes = append(codes, event.Code)
	}
	return codes
}

func compareOriginalEmptyRecorded(t *testing.T, java, goEvents []recorderdebug.Event) {
	t.Helper()
	if err := validateOriginalEmptyRecorded(java, goEvents); err != nil {
		t.Fatal(err)
	}
}

func validateOriginalEmptyRecorded(java, goEvents []recorderdebug.Event) error {
	// The Java version and JVM banner have no Go-equivalent typed observations.
	// Go's SANY records are tool-only; the original Java setup is non-tool.
	if !reflect.DeepEqual(recordCodes(java), []int{2262, 2187, 2185, 2189, 2190, 2193, 2199, 2194, 2186}) ||
		!reflect.DeepEqual(recordCodes(goEvents), []int{2220, 2219, 2185, 2189, 2190, 2193, 2199, 2194, 2186}) {
		return fmt.Errorf("unexpected Java/Go events: %v / %v", recordCodes(java), recordCodes(goEvents))
	}
	if len(java[0].Values) != 1 || !strings.Contains(java[0].Values[0], "rev: 5a47802") || len(java[1].Values) != 16 || java[1].Values[0] != "1" || java[1].Values[12] != "0" {
		return fmt.Errorf("unexpected Java version or startup arguments: %+v %+v", java[0], java[1])
	}
	for i := 2; i < len(java); i++ {
		a, b := java[i], goEvents[i]
		if a.Code != b.Code {
			return fmt.Errorf("event %d: Java=%+v Go=%+v", i, a, b)
		}
		if a.Code == 2186 {
			// The original setup and Go use different duration renderers.
			if len(a.Values) != 1 || len(b.Values) != 1 || a.Values[0] == "" || b.Values[0] == "" {
				return fmt.Errorf("invalid finish values: %+v %+v", a, b)
			}
		} else if !reflect.DeepEqual(a.Values, b.Values) {
			return fmt.Errorf("event %d: Java=%+v Go=%+v", i, a, b)
		}
	}
	return nil
}

func TestOriginalEmptyFullRecorderRejectsDamagedCapture(t *testing.T) {
	for _, input := range []string{"", "2193\tstrings\tdmFsID0gMC4w", "2193\tobjects\tdmFsID0gMC4w\n", "2193\tstrings\t!unsupported-value\n", "bad\tstrings\n"} {
		if _, err := decodeFullRecorder(input); err == nil {
			t.Fatalf("accepted damaged Java capture %q", input)
		}
	}
}

func TestOriginalEmptyFullComparisonRejectsMissingOrChangedCalls(t *testing.T) {
	banner := make([]string, 16)
	banner[0], banner[12] = "1", "0"
	java := []recorderdebug.Event{
		{Code: 2262, Values: []string{"Version (rev: 5a47802)"}},
		{Code: 2187, Values: banner},
		{Code: 2185},
		{Code: 2189},
		{Code: 2190, Values: []string{"0", "s"}},
		{Code: 2193, Values: []string{"val = 0.0"}},
		{Code: 2199, Values: []string{"0", "0", "0"}},
		{Code: 2194, Values: []string{"1"}},
		{Code: 2186, Values: []string{"00s"}},
	}
	goEvents := []recorderdebug.Event{
		{Code: 2220},
		{Code: 2219},
		{Code: 2185},
		{Code: 2189},
		{Code: 2190, Values: []string{"0", "s"}},
		{Code: 2193, Values: []string{"val = 0.0"}},
		{Code: 2199, Values: []string{"0", "0", "0"}},
		{Code: 2194, Values: []string{"1"}},
		{Code: 2186, Values: []string{"1ms"}},
	}
	if err := validateOriginalEmptyRecorded(java, goEvents); err != nil {
		t.Fatal(err)
	}
	for _, changed := range [][]recorderdebug.Event{
		goEvents[:len(goEvents)-1], append(append([]recorderdebug.Event(nil), goEvents...), goEvents[0]),
		append([]recorderdebug.Event{{Code: 1000}}, goEvents[1:]...),
	} {
		if err := validateOriginalEmptyRecorded(java, changed); err == nil {
			t.Fatalf("accepted missing or reordered calls: %+v", changed)
		}
	}
	changed := append([]recorderdebug.Event(nil), goEvents...)
	changed[5].Values = []string{"val = 1.0"}
	if err := validateOriginalEmptyRecorded(java, changed); err == nil {
		t.Fatal("accepted changed probability")
	}
}
