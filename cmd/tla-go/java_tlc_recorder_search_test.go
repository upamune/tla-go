package main

import (
	"bytes"
	"context"
	"crypto/sha256"
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

// These are selected actual MPRecorder calls, not a whole-recorder comparison.
func TestJavaTLCRecorderSearchEvents(t *testing.T) {
	jar := pinnedFairnessEffectJar(t)
	if jar == "" {
		t.Skip("set TLC_JAR to pinned Java TLC 1.7.4")
	}
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	classes := compileSearchRecorder(t, jar)
	for _, tc := range []struct {
		name, body, cfg string
		exit            int
		workers         int
	}{
		{"empty", "", "", 0, 1},
		{"one", "VARIABLE x\nInit == x = 0\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n", 0, 1},
		{"duplicate initial", "VARIABLE x\nInit == (x = 0) \\/ (x = 0)\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n", 0, 1},
		{"initial progress", "VARIABLE x\nInit == x \\in 0..9\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n", 0, 1},
		{"effectful initial", "VARIABLE x\nInit == x = IF PrintT(\"once\") THEN 1 ELSE 2\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n", 0, 1},
		{"parallel initial", "VARIABLE x\nInit == x \\in 0..127\nNext == x' = x" + strings.Repeat(" + 0", 45), "INIT Init\nNEXT Next\n", 0, 4},
		{"initial invariant", "VARIABLE x\nInit == x = 0\nNext == UNCHANGED x\nInv == x = 1", "INIT Init\nNEXT Next\nINVARIANT Inv\n", 12, 1},
		{"assumption", "ASSUME FALSE\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n", 10, 1},
		{"deadlock", "VARIABLE x\nInit == x = 0\nNext == x < 1 /\\ x' = x + 1", "INIT Init\nNEXT Next\n", 11, 1},
		{"initial error", "VARIABLE x\nInit == x = 1 \\div 0\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n", 75, 1},
		{"next error", "VARIABLE x\nInit == x = 0\nNext == x' = 1 \\div 0", "INIT Init\nNEXT Next\n", 255, 1},
		{"semantic error", "Bad == Missing", "", 150, 1},
		{"syntax error", "Bad ==", "", 150, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			model := "---- MODULE M ----\nEXTENDS Naturals\n" + tc.body + "\n====\n"
			effectful := tc.name == "effectful initial"
			if effectful {
				model = strings.Replace(model, "EXTENDS Naturals\n", "EXTENDS Naturals, TLC\n", 1)
			}
			for name, content := range map[string]string{"M.tla": model, "M.cfg": tc.cfg} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(dir, "go.jsonl")
			t.Setenv("TLA_GO_DEBUG_RECORDER", path)
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", "-workers", strconv.Itoa(tc.workers), "-fp", "0", filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != tc.exit || errout.Len() != 0 {
				t.Fatalf("Go exit=%d stderr=%s stdout=%s", code, &errout, &out)
			}
			if effectful && strings.Count(out.String(), `"once"`) != 1 {
				t.Fatalf("Go effect was repeated or absent: %s", &out)
			}
			goEvents := readSearchRecorder(t, path)
			for _, tool := range []bool{true, false} {
				javaEvents := runSearchRecorderJava(t, jar, classes, dir, tc.exit, tc.workers, tool, effectful)
				want := searchComparable(goEvents, tool)
				if !reflect.DeepEqual(javaEvents, want) {
					t.Fatalf("tool=%t Java=%+v Go=%+v", tool, javaEvents, want)
				}
			}
		})
	}
}

const searchRecorderJava = `import tlc2.output.MP;
import tlc2.output.MPRecorder;
import java.nio.file.*;
import java.nio.charset.StandardCharsets;
import java.util.Arrays;
public class SearchRecorder {
  public static void main(String[] args) throws Exception {
    MP.setRecorder(new MPRecorder() {
      public synchronized void record(int code, Object... values) {
        if (!(code == 2185 || code == 2186 || code == 2189 || code == 2190 || code == 2191 || code == 2194 || code == 2199 || code == 2219 || code == 2220 || code == 2269)) return;
        String line = code + "\t" + values.getClass().getName();
        for (Object value : values) line += "\t" + value;
        try { Files.write(Paths.get(args[0]), (line + "\n").getBytes(StandardCharsets.UTF_8), StandardOpenOption.CREATE, StandardOpenOption.APPEND); }
        catch (Exception e) { throw new RuntimeException(e); }
      }
    });
    tlc2.TLC.main(Arrays.copyOfRange(args, 1, args.length));
  }
}`

func compileSearchRecorder(t *testing.T, jar string) string {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "SearchRecorder.java")
	if err := os.WriteFile(file, []byte(searchRecorderJava), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "javac", "-cp", jar, file).CombinedOutput(); err != nil {
		t.Fatalf("javac: %v %s", err, out)
	}
	return dir
}

func readSearchRecorder(t *testing.T, path string) []recorderdebug.Event {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	events, err := recorderdebug.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return events
}

func runSearchRecorderJava(t *testing.T, jar, classes, dir string, wantExit, workers int, tool, effectful bool) []string {
	t.Helper()
	mode := "human"
	if tool {
		mode = "tool"
	}
	path := filepath.Join(dir, "java-"+mode+".txt")
	args := []string{"-Xmx256m", "-XX:+UseParallelGC", "-cp", classes + string(os.PathListSeparator) + jar, "SearchRecorder", path, "-workers", strconv.Itoa(workers), "-fp", "0", "-metadir", filepath.Join(dir, "states-"+mode)}
	if tool {
		args = append(args, "-tool")
	}
	args = append(args, "M.tla")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil || cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != wantExit {
		t.Fatalf("Java tool=%t exit=%v err=%v output=%s", tool, cmd.ProcessState, err, out)
	}
	if effectful && bytes.Count(out, []byte(`"once"`)) != 1 {
		t.Fatalf("Java effect was repeated or absent: %s", out)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return normalizeSearchJava(t, string(data))
}

func normalizeSearchJava(t *testing.T, data string) []string {
	t.Helper()
	var result []string
	for line := range strings.SplitSeq(strings.TrimSuffix(data, "\n"), "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) < 2 || parts[1] != "[Ljava.lang.String;" {
			t.Fatalf("unsupported Java recorder array: %q", line)
		}
		if parts[0] == "2186" {
			if len(parts) != 3 || parts[2] == "" {
				t.Fatalf("invalid Java finish record: %q", line)
			}
			parts[2] = "TIME"
		}
		result = append(result, strings.Join(append(parts[:1], parts[2:]...), "\t"))
	}
	return result
}

func searchComparable(events []recorderdebug.Event, tool bool) []string {
	var result []string
	for _, event := range events {
		if !tool && (event.Code == 2220 || event.Code == 2219) {
			continue
		}
		values := event.Values
		if event.Code == 2186 {
			values = []string{"TIME"}
		}
		result = append(result, strings.Join(append([]string{strconv.Itoa(event.Code)}, values...), "\t"))
	}
	return result
}

func TestJavaTLCRecorderLoadFailures(t *testing.T) {
	jar := pinnedFairnessEffectJar(t)
	if jar == "" {
		t.Skip("set TLC_JAR to pinned Java TLC 1.7.4")
	}
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	classes := compileSearchRecorder(t, jar)
	modern := pinnedRecorderModernJar(t)
	for _, tc := range []struct {
		name, source, config string
		directory            bool
		goExit, javaExit     int
		want                 []int
	}{
		{"module name", "---- MODULE N ----\n====\n", "", false, 150, 150, []int{2220, 2219, 2185, 3002, 2186}},
		{"missing import", "---- MODULE M ----\nEXTENDS MissingModule\n====\n", "", false, 150, 150, []int{2220, 2219, 2185, 3002, 2186}},
		{"standard imports", "---- MODULE M ----\nEXTENDS Naturals, Sequences\nX == Len(<<1>>)\n====\n", "", false, 0, 0, []int{2220, 2219, 2185, 2186}},
		{"missing root", "", "", false, 151, 150, []int{2220, 2219, 2185, 3002, 2186}},
		{"root is directory", "", "", true, 151, 150, []int{2220, 2219, 2185, 3002, 2186}},
		{"invalid config", "---- MODULE M ----\n====\n", "INIT", false, 255, 255, []int{1000, 2186}},
		{"missing config", "---- MODULE M ----\n====\n", "<missing>", false, 255, 255, []int{1000, 2186}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			model := filepath.Join(dir, "M.tla")
			if tc.directory {
				if err := os.Mkdir(model, 0o700); err != nil {
					t.Fatal(err)
				}
			} else if tc.source != "" {
				if err := os.WriteFile(model, []byte(tc.source), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if tc.config != "<missing>" {
				if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte(tc.config), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(dir, "go.jsonl")
			t.Setenv("TLA_GO_DEBUG_RECORDER", path)
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", model}, &out, &errout)
			if code != tc.goExit || errout.Len() != 0 {
				t.Fatalf("Go exit=%d stdout=%s stderr=%s", code, &out, &errout)
			}
			javaEvents := runSearchRecorderJava(t, jar, classes, dir, tc.javaExit, 1, true, false)
			if got := searchComparable(readSearchRecorder(t, path), true); !reflect.DeepEqual(got, javaEvents) {
				t.Fatalf("recorder events Go=%v Java=%v", got, javaEvents)
			}
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-metadir", filepath.Join(dir, "display-states"), "M.tla"}
			java, javaCode := runJavaTLCArgs(t, jar, dir, args)
			if javaCode != tc.javaExit {
				t.Fatalf("Java exit=%d output=%s", javaCode, java)
			}
			if got := loadToolCodes(out.String()); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Go tool codes=%v want=%v", got, tc.want)
			}
			if got := loadToolCodes(java); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Java tool codes=%v want=%v", got, tc.want)
			}
			compareLoadToolBodies(t, out.String(), java)
			if modern != "" && tc.config != "INIT" && tc.config != "<missing>" {
				modernArgs := []string{"-tool", "-workers", "1", "-fp", "0", "-metadir", filepath.Join(dir, "modern-states"), "M.tla"}
				modernOutput, modernExit := runJavaTLCArgs(t, modern, dir, modernArgs)
				if modernExit != tc.javaExit {
					t.Fatalf("modern Java exit=%d output=%s", modernExit, modernOutput)
				}
				if got, want := sanyToolMessages(out.String()), sanyToolMessages(modernOutput); !reflect.DeepEqual(got, want) {
					t.Fatalf("SANY messages Go=%v modern Java=%v", got, want)
				}
			}
		})
	}
}

func pinnedRecorderModernJar(t *testing.T) string {
	t.Helper()
	jar := os.Getenv("TLC_MODERN_JAR")
	if jar == "" {
		return ""
	}
	data, err := os.ReadFile(jar)
	if err != nil || fmt.Sprintf("%x", sha256.Sum256(data)) != "011267fca25d0752cf18894dd585cd28b96960c112ea8a20b823ea770e7e10cd" {
		t.Fatalf("TLC_MODERN_JAR must be the pinned 1.8.0 jar: %v", err)
	}
	return jar
}

func loadToolCodes(output string) []int {
	var codes []int
	for _, message := range readToolMessages(output) {
		switch message.code {
		case 1000, 2220, 2219, 2185, 3002, 2186:
			codes = append(codes, message.code)
		}
	}
	return codes
}

func compareLoadToolBodies(t *testing.T, goOutput, javaOutput string) {
	t.Helper()
	goMessages, javaMessages := selectedLoadToolMessages(goOutput), selectedLoadToolMessages(javaOutput)
	if !reflect.DeepEqual(goMessages, javaMessages) {
		t.Errorf("load messages Go=%+v Java=%+v", goMessages, javaMessages)
	}
}

func selectedLoadToolMessages(output string) []toolMessage {
	var result []toolMessage
	for _, message := range readToolMessages(output) {
		switch message.code {
		case 1000, 2220, 2219, 2185, 3002, 2186:
			message.body = normalizeRecorderToolOutput(message.body)
			result = append(result, message)
		}
	}
	return result
}

func sanyToolMessages(output string) []toolMessage {
	var result []toolMessage
	for _, message := range readToolMessages(output) {
		if message.code == 2220 || message.code == 2219 {
			result = append(result, message)
		}
	}
	return result
}
