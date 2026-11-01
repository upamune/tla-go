package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/upamune/tla-go/internal/javatmp"
	"github.com/upamune/tla-go/internal/recorderdebug"
)

func TestJavaTLCRecorderDebugObservations(t *testing.T) {
	t.Setenv("TLA_GO_DEBUG_RECORDER", "")
	dir := t.TempDir()
	model := filepath.Join(dir, "M.tla")
	if err := os.WriteFile(model, []byte("---- MODULE M ----\n====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"-tool", "-workers", "1", "-fp", "0", model}
	var plain, plainErr bytes.Buffer
	if code := run(context.Background(), args, &plain, &plainErr); code != 0 || plainErr.Len() != 0 {
		t.Fatalf("plain exit=%d stdout=%s stderr=%s", code, &plain, &plainErr)
	}
	path := filepath.Join(dir, "recorder.jsonl")
	t.Setenv("TLA_GO_DEBUG_RECORDER", path)
	var debug, debugErr bytes.Buffer
	if code := run(context.Background(), args, &debug, &debugErr); code != 0 || debugErr.Len() != 0 {
		t.Fatalf("debug exit=%d stdout=%s stderr=%s", code, &debug, &debugErr)
	}
	if normalizeRecorderToolOutput(plain.String()) != normalizeRecorderToolOutput(debug.String()) {
		t.Fatalf("debug changed tool output:\nplain %s\ndebug %s", &plain, &debug)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	events, err := recorderdebug.Decode(f)
	events = coreRecorderEvents(events)
	if err != nil || len(events) != 2 || events[0].Code != 2199 || strings.Join(events[0].Values, ",") != "0,0,0" || events[1].Code != 2186 || !regexp.MustCompile(`^[0-9]+ms$`).MatchString(events[1].Values[0]) {
		t.Fatalf("events=%+v error=%v", events, err)
	}
	checkJavaRecorderObservations(t, dir, events, "1")
}

func coreRecorderEvents(events []recorderdebug.Event) []recorderdebug.Event {
	var result []recorderdebug.Event
	for _, event := range events {
		if event.Code == 2199 || event.Code == 2186 {
			result = append(result, event)
		}
	}
	return result
}

func normalizeRecorderToolOutput(s string) string {
	return regexp.MustCompile(`Finished in [0-9]+ms`).ReplaceAllString(normalizeTemporalProgress(s), "Finished in TIME")
}

func checkJavaRecorderObservations(t *testing.T, dir string, events []recorderdebug.Event, workers string) {
	t.Helper()
	jar := pinnedFairnessEffectJar(t)
	if jar == "" {
		t.Log("Java recorder comparison skipped without pinned TLC_JAR")
		return
	}
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	const source = `import tlc2.output.MP;
import tlc2.output.MPRecorder;
import java.nio.file.*;
import java.nio.charset.StandardCharsets;
import java.util.Arrays;
public class RecorderProbe {
  public static void main(String[] args) throws Exception {
    MP.setRecorder(new MPRecorder() {
      public synchronized void record(int code, Object... values) {
        if (code != 2199 && code != 2186) return;
        String line = code + "\t" + values.getClass().getName();
        for (Object value : values) line += "\t" + value;
        try { Files.write(Paths.get(args[0]), (line + "\n").getBytes(StandardCharsets.UTF_8), StandardOpenOption.CREATE, StandardOpenOption.APPEND); }
        catch (Exception e) { throw new RuntimeException(e); }
      }
    });
    tlc2.TLC.main(Arrays.copyOfRange(args, 1, args.length));
  }
}`
	file := filepath.Join(dir, "RecorderProbe.java")
	if err := os.WriteFile(file, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "javac", "-cp", jar, file).CombinedOutput(); err != nil {
		t.Fatalf("javac: %v %s", err, out)
	}
	result := filepath.Join(dir, "java-records")
	cmd := exec.CommandContext(ctx, "java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-cp", dir+string(os.PathListSeparator)+jar, "RecorderProbe", result, "-tool", "-workers", workers, "-fp", "0", "M.tla")
	cmd.Dir = dir
	if out, err := javatmp.Apply(cmd).CombinedOutput(); err != nil {
		t.Fatalf("Java recorder: %v %s", err, out)
	}
	data, err := os.ReadFile(result)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 || lines[0] != "2199\t[Ljava.lang.String;\t"+strings.Join(events[0].Values, "\t") || !regexp.MustCompile(`^2186\t\[Ljava\.lang\.String;\t[0-9]+ms$`).MatchString(lines[1]) {
		t.Fatalf("Java recorder differs: %q", data)
	}
}

func TestRecorderDebugProtectsExistingFiles(t *testing.T) {
	dir := t.TempDir()
	model := filepath.Join(dir, "M.tla")
	if err := os.WriteFile(model, []byte("---- MODULE M ----\n====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "existing.jsonl")
	if err := os.WriteFile(path, []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TLA_GO_DEBUG_RECORDER", path)
	var out, errout bytes.Buffer
	if code := run(context.Background(), []string{"-tool", model}, &out, &errout); code == 0 {
		t.Fatal("accepted an existing recorder file")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "preserve" {
		t.Fatalf("existing file changed: %q %v", data, err)
	}
}

func TestRecorderDebugRejectsNonModelModes(t *testing.T) {
	dir := t.TempDir()
	model := filepath.Join(dir, "M.tla")
	source := []byte("---- MODULE M ----\n====\n")
	if err := os.WriteFile(model, source, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, option := range []string{"-sany-metadata", "-version", "-help"} {
		t.Run(option, func(t *testing.T) {
			args := []string{"-tool", option, model}
			t.Setenv("TLA_GO_DEBUG_RECORDER", "")
			var plain, plainErr bytes.Buffer
			if code := run(context.Background(), args, &plain, &plainErr); code != 0 || plainErr.Len() != 0 {
				t.Fatalf("ordinary mode exit=%d stdout=%s stderr=%s", code, &plain, &plainErr)
			}
			path := filepath.Join(t.TempDir(), "recorder.jsonl")
			output := filepath.Join(t.TempDir(), "user.txt")
			t.Setenv("TLA_GO_DEBUG_RECORDER", path)
			var out, errout bytes.Buffer
			code := run(context.Background(), append(args, "-userFile", output), &out, &errout)
			if code == 0 || !strings.Contains(errout.String(), "requires one model in model-checking mode") {
				t.Fatalf("unsupported mode exit=%d stdout=%s stderr=%s", code, &out, &errout)
			}
			for _, name := range []string{path, output} {
				if _, err := os.Lstat(name); !os.IsNotExist(err) {
					t.Fatalf("unsupported mode created %s: %v", name, err)
				}
			}
		})
	}
	data, err := os.ReadFile(model)
	if err != nil || !bytes.Equal(data, source) {
		t.Fatalf("model changed: %q %v", data, err)
	}
}

func TestRecorderDebugRejectsTranslationBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join("..", "..", "pcal", "testdata", "upstream", "tlatools", "org.lamport.tlatools", "test-model", "pcal", "CallReturn1.tla")
	source, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	translation := bytes.Index(source, []byte(`\* BEGIN TRANSLATION`))
	if translation < 0 {
		t.Fatal("upstream fixture has no translation marker")
	}
	source = append(bytes.Clone(source[:translation]), []byte("====\n")...)
	spec := filepath.Join(dir, "CallReturn1.tla")
	old := filepath.Join(dir, "CallReturn1.old")
	cfg := filepath.Join(dir, "CallReturn1.cfg")
	recorder := filepath.Join(dir, "recorder.jsonl")
	for name, data := range map[string][]byte{spec: source, old: []byte("old sentinel"), cfg: []byte("config sentinel")} {
		if err := os.WriteFile(name, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	args := []string{"-translate", "-unixEOL", "-wf", "-termination", spec}
	t.Setenv("TLA_GO_DEBUG_RECORDER", recorder)
	var out, errout bytes.Buffer
	if code := run(context.Background(), args, &out, &errout); code != 255 || out.Len() != 0 || !strings.Contains(errout.String(), recorderModeError) {
		t.Fatalf("debug translation exit=%d stdout=%s stderr=%s", code, &out, &errout)
	}
	for name, want := range map[string][]byte{spec: source, old: []byte("old sentinel"), cfg: []byte("config sentinel")} {
		got, err := os.ReadFile(name)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("debug translation changed %s: %v", name, err)
		}
	}
	if _, err := os.Lstat(recorder); !os.IsNotExist(err) {
		t.Fatalf("debug translation created recorder output: %v", err)
	}
	t.Setenv("TLA_GO_DEBUG_RECORDER", "")
	out.Reset()
	errout.Reset()
	if code := run(context.Background(), args, &out, &errout); code != 0 || errout.Len() != 0 || !strings.Contains(out.String(), "Translation completed.") {
		t.Fatalf("ordinary translation exit=%d stdout=%s stderr=%s", code, &out, &errout)
	}
	translated, err := os.ReadFile(spec)
	if err != nil || bytes.Equal(translated, source) {
		t.Fatalf("ordinary translation did not update spec: %v", err)
	}
	backup, err := os.ReadFile(old)
	if err != nil || !bytes.Equal(backup, source) {
		t.Fatalf("ordinary translation did not save original: %v", err)
	}
	if _, err := os.Lstat(recorder); !os.IsNotExist(err) {
		t.Fatalf("ordinary translation created recorder output: %v", err)
	}
}

func TestJavaTLCRecorderDebugParallel(t *testing.T) {
	dir := t.TempDir()
	model := filepath.Join(dir, "M.tla")
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x \\in 0..127\nNext == x' = x" + strings.Repeat(" + 0", 45) + "\n====\n"
	if err := os.WriteFile(model, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TLA_GO_DEBUG_RECORDER", "")
	var plain, plainErr bytes.Buffer
	args := []string{"-tool", "-workers", "4", model}
	if code := run(context.Background(), args, &plain, &plainErr); code != 0 || plainErr.Len() != 0 {
		t.Fatalf("plain exit=%d stdout=%s stderr=%s", code, &plain, &plainErr)
	}
	t.Setenv("TLA_GO_DEBUG_RECORDER", filepath.Join(dir, "parallel.jsonl"))
	var out, errout bytes.Buffer
	code := run(context.Background(), args, &out, &errout)
	if code != 0 || errout.Len() != 0 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, &out, &errout)
	}
	if normalizeRecorderToolOutput(plain.String()) != normalizeRecorderToolOutput(out.String()) {
		t.Fatalf("parallel debug changed tool output: plain %s debug %s", &plain, &out)
	}
	f, err := os.Open(filepath.Join(dir, "parallel.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	events, err := recorderdebug.Decode(f)
	events = coreRecorderEvents(events)
	if err != nil || len(events) != 2 || strings.Join(events[0].Values, ",") != "256,128,0" || events[1].Code != 2186 {
		t.Fatalf("parallel events=%+v error=%v", events, err)
	}
	checkJavaRecorderObservations(t, dir, events, "4")
}

func TestRecorderDebugRejectsCollidingPaths(t *testing.T) {
	dir := t.TempDir()
	model := filepath.Join(dir, "M.tla")
	if err := os.WriteFile(model, []byte("---- MODULE M ----\n====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "N.tla"), []byte("---- MODULE N ----\n====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, path string
		args       []string
	}{
		{"model", model, []string{"-tool", model}},
		{"dump", filepath.Join(dir, "out.dump"), []string{"-tool", "-dump", filepath.Join(dir, "out.dump"), model}},
		{"user file", filepath.Join(dir, "user.txt"), []string{"-tool", "-userFile", filepath.Join(dir, "user.txt"), model}},
		{"runtime trace", filepath.Join(dir, "trace.out"), []string{"-tool", "-debug-trace", filepath.Join(dir, "trace.out"), model}},
		{"simulation", filepath.Join(dir, "simulation.json"), []string{"-tool", "-simulate", model}},
		{"dfid", filepath.Join(dir, "dfid.json"), []string{"-tool", "-dfid", "2", model}},
		{"checkpoint", filepath.Join(dir, "checkpoint.json"), []string{"-tool", "-checkpoint", "1", model}},
		{"recovery", filepath.Join(dir, "recovery.json"), []string{"-tool", "-recover", filepath.Join(dir, "states"), model}},
		{"glob", filepath.Join(dir, "glob.json"), []string{"-tool", filepath.Join(dir, "*.tla")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TLA_GO_DEBUG_RECORDER", tc.path)
			var out, errout bytes.Buffer
			if code := run(context.Background(), tc.args, &out, &errout); code == 0 {
				t.Fatalf("accepted unsafe recorder path %q", tc.path)
			}
			if tc.path != model {
				if _, err := os.Lstat(tc.path); !os.IsNotExist(err) {
					t.Fatalf("unexpected diagnostic artifact: %v", err)
				}
			}
		})
	}
	target, alias := filepath.Join(dir, "target.txt"), filepath.Join(dir, "alias.txt")
	if err := os.Symlink(target, alias); err == nil {
		t.Run("dangling symlink", func(t *testing.T) {
			t.Setenv("TLA_GO_DEBUG_RECORDER", target)
			var out, errout bytes.Buffer
			if code := run(context.Background(), []string{"-tool", "-userFile", alias, model}, &out, &errout); code == 0 {
				t.Fatal("accepted a diagnostic path aliased by user output")
			}
			if _, err := os.Stat(target); !os.IsNotExist(err) {
				t.Fatalf("diagnostic target was created: %v", err)
			}
		})
	}
	data, err := os.ReadFile(model)
	if err != nil || string(data) != "---- MODULE M ----\n====\n" {
		t.Fatalf("model changed: %q %v", data, err)
	}
}

func TestRecorderDebugPropagatesWriteAndCloseFailure(t *testing.T) {
	for _, write := range []func(*recorderConfig){
		func(recorder *recorderConfig) { recordStrings(recorder, 2186, "1ms") },
		func(recorder *recorderConfig) {
			recordOmission(recorder, 2193, recorderdebug.SuccessProbabilityOmission)
		},
	} {
		f, err := os.CreateTemp(t.TempDir(), "recorder")
		if err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		profile := &profileFile{file: f}
		write(&recorderConfig{file: profile})
		if profile.err == nil || (&profiles{recorder: profile}).close() == nil {
			t.Fatal("recorder write or close error was discarded")
		}
	}
}

func TestRecorderDebugConcurrentWrites(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "recorder")
	if err != nil {
		t.Fatal(err)
	}
	profile := &profileFile{file: f}
	if err := startRecorderOutput(profile); err != nil {
		t.Fatal(err)
	}
	recorder := &recorderConfig{file: profile}
	var group sync.WaitGroup
	for range 8 {
		group.Go(func() {
			for range 40 {
				recordStrings(recorder, 2189)
			}
		})
	}
	group.Wait()
	if err := (&profiles{recorder: profile}).close(); err != nil {
		t.Fatal(err)
	}
	r, err := os.Open(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	events, err := recorderdebug.Decode(r)
	if err != nil || len(events) != 320 {
		t.Fatalf("events=%d error=%v", len(events), err)
	}
}

func TestRecorderDebugCloseFailureAlone(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "recorder")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	profile := &profileFile{file: f}
	if profile.err != nil {
		t.Fatal("fixture unexpectedly contains a write error")
	}
	if err := (&profiles{recorder: profile}).close(); err == nil || !strings.Contains(err.Error(), "close ") || strings.Contains(err.Error(), "write ") {
		t.Fatalf("close-only error not reported: %v", err)
	}
}

func TestRecorderDebugCloseFailureThroughCLI(t *testing.T) {
	t.Setenv("TLA_GO_DEBUG_RECORDER", "")
	for _, tc := range []struct {
		name, body string
		want       int
	}{
		{"success", "", 255},
		{"semantic error", "Bad == Missing\n", 150},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			model := filepath.Join(dir, "M.tla")
			if err := os.WriteFile(model, []byte("---- MODULE M ----\n"+tc.body+"====\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "M.cfg"), nil, 0o600); err != nil {
				t.Fatal(err)
			}
			f, err := os.CreateTemp(dir, "recorder")
			if err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
			profile := &profileFile{file: f}
			start := func(options) (*profiles, error) { return &profiles{recorder: profile}, nil }
			var out, errout bytes.Buffer
			code := runWithProfileStarter(context.Background(), []string{"-tool", model}, strings.NewReader(""), &out, &errout, start)
			if code != tc.want || profile.err != nil || !hasToolMessage(readToolMessages(out.String()), 2186) || !strings.Contains(errout.String(), "close ") || strings.Contains(errout.String(), "write ") {
				t.Fatalf("exit=%d write error=%v stdout=%s stderr=%s", code, profile.err, &out, &errout)
			}
		})
	}
}

func TestRecorderDebugFailureIsIncomplete(t *testing.T) {
	dir := t.TempDir()
	model := filepath.Join(dir, "M.tla")
	if err := os.WriteFile(model, []byte("---- MODULE M ----\nBad == Missing\n====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"-tool", model}
	t.Setenv("TLA_GO_DEBUG_RECORDER", "")
	var plain, plainErr bytes.Buffer
	plainCode := run(context.Background(), args, &plain, &plainErr)
	path := filepath.Join(dir, "error.jsonl")
	t.Setenv("TLA_GO_DEBUG_RECORDER", path)
	var debug, debugErr bytes.Buffer
	code := run(context.Background(), args, &debug, &debugErr)
	if code != 150 || code != plainCode || plainErr.String() != debugErr.String() || normalizeRecorderToolOutput(plain.String()) != normalizeRecorderToolOutput(debug.String()) {
		t.Fatalf("plain exit=%d stdout=%s stderr=%s; debug exit=%d stdout=%s stderr=%s", plainCode, &plain, &plainErr, code, &debug, &debugErr)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	events, err := recorderdebug.Decode(f)
	events = coreRecorderEvents(events)
	if err != nil || len(events) != 1 || events[0].Code != 2186 {
		t.Fatalf("incomplete error observations=%+v error=%v", events, err)
	}
}

func TestRecorderDebugDoesNotRepeatEffects(t *testing.T) {
	dir := t.TempDir()
	model := filepath.Join(dir, "M.tla")
	source := "---- MODULE M ----\nEXTENDS TLC\nVARIABLE x\nInit == x = IF PrintT(\"once\") THEN 1 ELSE 2\nNext == UNCHANGED x\n====\n"
	if err := os.WriteFile(model, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"-tool", model}
	t.Setenv("TLA_GO_DEBUG_RECORDER", "")
	var plain, plainErr bytes.Buffer
	plainCode := run(context.Background(), args, &plain, &plainErr)
	t.Setenv("TLA_GO_DEBUG_RECORDER", filepath.Join(dir, "effects.jsonl"))
	var debug, debugErr bytes.Buffer
	code := run(context.Background(), args, &debug, &debugErr)
	if code != 0 || code != plainCode || plainErr.String() != debugErr.String() || strings.Count(debug.String(), `"once"`) != 1 || normalizeRecorderToolOutput(plain.String()) != normalizeRecorderToolOutput(debug.String()) {
		t.Fatalf("plain exit=%d stdout=%s stderr=%s; debug exit=%d stdout=%s stderr=%s", plainCode, &plain, &plainErr, code, &debug, &debugErr)
	}
}
