package upstreamjunit

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/upamune/tla-go/internal/javatmp"
)

func TestRunJavaCommandIgnoresJavaOptionsOverride(t *testing.T) {
	requireJavaTool(t, "java")
	dir := t.TempDir()
	shared := filepath.Join(dir, "shared directory")
	if err := os.Mkdir(shared, 0o700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(shared, "Naturals.tla")
	if err := os.WriteFile(sentinel, []byte("sentinel"), 0o600); err != nil {
		t.Fatal(err)
	}
	options := `-Djava.io.tmpdir="` + shared + `" -Dtla.options=removed`
	t.Setenv("_JAVA_OPTIONS", options)
	t.Setenv("JAVA_TOOL_OPTIONS", "-Dtla.tool=kept -Djava.io.tmpdir="+dir)
	t.Setenv("JDK_JAVA_OPTIONS", "-Dtla.jdk=kept -Djava.io.tmpdir="+dir)
	result, err := runJavaCommand(dir, 30*time.Second, "-XshowSettings:properties", "-version")
	if err != nil || result.ExitCode != 0 {
		t.Fatalf("Java version: %+v, %v", result, err)
	}
	if javaTemporaryDirectory(t, result.Output) == shared || !strings.Contains(result.Output, "tla.tool = kept") || !strings.Contains(result.Output, "tla.jdk = kept") || strings.Contains(result.Output, "tla.options = removed") {
		t.Fatalf("unexpected JVM properties: %s", result.Output)
	}
	if os.Getenv("_JAVA_OPTIONS") != options {
		t.Fatal("parent environment changed")
	}
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Log("TLC_JAR not set; real SANY sentinel check skipped")
		return
	}
	data, err := os.ReadFile(jar)
	if err != nil || fmt.Sprintf("%x", sha256.Sum256(data)) != "936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88" {
		t.Fatalf("TLC_JAR must be the pinned 1.7.4 jar: %v", err)
	}
	jar, err = filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	module := "---- MODULE M ----\nEXTENDS Naturals\nF == 1 + 1\n====\n"
	if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(module), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err = runJavaCommand(dir, 30*time.Second, "-cp", jar, "tla2sany.SANY", "M.tla")
	if err != nil || result.ExitCode != 0 || !strings.Contains(result.Output, "Semantic processing of module M") {
		t.Fatalf("SANY: %+v, %v", result, err)
	}
	data, err = os.ReadFile(sentinel)
	if err != nil || string(data) != "sentinel" {
		t.Fatalf("shared sentinel changed: %q, %v", data, err)
	}
}

func requireJavaTool(t *testing.T, name string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, "-version")
	if name == "java" {
		javatmp.Apply(cmd)
	}
	if err := cmd.Run(); err != nil {
		t.Skipf("%s is not available: %v", name, err)
	}
}

func requireJava11(t *testing.T, name string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, "-version")
	javatmp.Apply(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("%s is not available: %v", name, err)
	}
	version := regexp.MustCompile(`(?m)^(?:openjdk|java|javac) (?:version )?"?(1\.)?(\d+)`).FindStringSubmatch(string(out))
	if len(version) != 3 {
		t.Skipf("cannot determine %s version: %s", name, out)
	}
	major, _ := strconv.Atoi(version[2])
	if version[1] != "" || major < 11 {
		t.Skipf("%s 11 or later required: %s", name, out)
	}
}

func javaTemporaryDirectory(t *testing.T, output string) string {
	t.Helper()
	for line := range strings.SplitSeq(output, "\n") {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), "java.io.tmpdir = "); ok {
			return value
		}
	}
	t.Fatalf("Java did not report its temporary directory: %s", output)
	return ""
}

func TestRunJavaCommandTemporaryDirectory(t *testing.T) {
	requireJava11(t, "java")
	root := filepath.Join(t.TempDir(), "java temporary root")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	working := t.TempDir()
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, root)
	}
	result, err := runJavaCommand(working, 30*time.Second, "-XshowSettings:properties", "-version")
	if err != nil || result.ExitCode != 0 {
		t.Fatalf("Java result %+v, error %v", result, err)
	}
	dir := javaTemporaryDirectory(t, result.Output)
	relative, err := filepath.Rel(root, dir)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) || !strings.Contains(dir, " ") {
		t.Fatalf("Java temporary directory %q outside %q: %v", dir, root, err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("Java temporary directory was not removed: %s (%v)", dir, err)
	}
	failed, err := runJavaCommand(working, 30*time.Second, "-this-is-not-a-java-option")
	if err != nil || failed.ExitCode == 0 {
		t.Fatalf("expected a nonzero Java exit: %+v, %v", failed, err)
	}
	timed, err := runJavaCommand(working, time.Nanosecond, "-version")
	if err == nil || !timed.TimedOut {
		t.Fatalf("expected timeout: %+v, %v", timed, err)
	}
	waitSource := filepath.Join(working, "Wait.java")
	waitBody := `public class Wait { public static void main(String[] args) throws Exception { java.nio.file.Files.writeString(java.nio.file.Path.of(args[0]), System.getProperty("java.io.tmpdir")); Thread.sleep(30000); } }`
	if err := os.WriteFile(waitSource, []byte(waitBody), 0o600); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(working, "started")
	timed, err = runJavaCommand(working, 5*time.Second, waitSource, marker)
	if err == nil || !timed.TimedOut {
		t.Fatalf("expected timeout after starting Java: %+v, %v", timed, err)
	}
	used, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("Java did not start before the timeout: %v", err)
	}
	if _, err := os.Stat(string(used)); !os.IsNotExist(err) {
		t.Fatalf("timed-out Java temporary directory was not removed: %s (%v)", used, err)
	}
	t.Setenv("PATH", filepath.Join(working, "no-java-here"))
	if _, err := runJavaCommand(working, time.Second, "-version"); err == nil {
		t.Fatal("expected a Java launch error")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("temporary directories remained after failure or timeout: %v, %v", entries, err)
	}
}

func TestRunJavaCommandConcurrentTemporaryDirectories(t *testing.T) {
	requireJavaTool(t, "java")
	working := t.TempDir()
	type outcome struct {
		result Result
		err    error
	}
	results := make(chan outcome, 8)
	workers := make(chan struct{}, 4)
	var group sync.WaitGroup
	for range 8 {
		group.Go(func() {
			workers <- struct{}{}
			defer func() { <-workers }()
			result, err := runJavaCommand(working, 30*time.Second, "-XshowSettings:properties", "-version")
			results <- outcome{result, err}
		})
	}
	group.Wait()
	close(results)
	seen := make(map[string]bool)
	for item := range results {
		if item.err != nil || item.result.ExitCode != 0 {
			t.Fatalf("Java result %+v, error %v", item.result, item.err)
		}
		dir := javaTemporaryDirectory(t, item.result.Output)
		if seen[dir] {
			t.Fatalf("duplicate Java temporary directory %q", dir)
		}
		seen[dir] = true
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("temporary directory still exists: %q, %v", dir, err)
		}
	}
}

func TestRunJavaCommandInvalidTemporaryRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(root, []byte("unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}
	working := t.TempDir()
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, root)
	}
	if _, err := runJavaCommand(working, time.Second, "-version"); err == nil {
		t.Fatal("accepted a temporary root that is a file")
	}
	data, err := os.ReadFile(root)
	if err != nil || string(data) != "unchanged" {
		t.Fatalf("temporary root changed: %q, %v", data, err)
	}
}

func TestRunJavaCaseConcurrentIsolation(t *testing.T) {
	for _, binary := range []string{"java", "javac"} {
		requireJava11(t, binary)
	}
	root := filepath.Join(t.TempDir(), "source and class paths with spaces")
	source := filepath.Join(root, "source", "tlc2")
	classes := filepath.Join(root, "class path")
	for _, dir := range []string{source, classes} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"TLC.java":        `package tlc2; public class TLC { public static void main(String[] args) { System.out.println(System.getProperty("java.io.tmpdir")); } }`,
		"TLCGlobals.java": `package tlc2; public class TLCGlobals { public static double livenessThreshold; }`,
	}
	args := []string{"-d", classes}
	for name, content := range files {
		path := filepath.Join(source, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		args = append(args, path)
	}
	if out, err := exec.Command("javac", args...).CombinedOutput(); err != nil {
		t.Fatalf("compile test classes: %v\n%s", err, out)
	}
	var cases []string
	for range 4 {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "java"), 0o700); err != nil {
			t.Fatal(err)
		}
		cases = append(cases, dir)
	}
	type outcome struct {
		result Result
		err    error
	}
	results := make(chan outcome, len(cases))
	var group sync.WaitGroup
	for _, dir := range cases {
		group.Go(func() {
			result, err := runJavaCase(dir, classes, "", 30*time.Second, nil)
			results <- outcome{result, err}
		})
	}
	group.Wait()
	close(results)
	seen := make(map[string]bool)
	for item := range results {
		if item.err != nil || item.result.ExitCode != 0 {
			t.Fatalf("Java case: %+v, %v", item.result, item.err)
		}
		dir := strings.TrimSpace(item.result.Output)
		if dir == "" || seen[dir] {
			t.Fatalf("missing or shared Java temporary directory %q", dir)
		}
		seen[dir] = true
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("temporary directory still exists: %q, %v", dir, err)
		}
	}
}
