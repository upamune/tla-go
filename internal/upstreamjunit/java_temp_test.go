package upstreamjunit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

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
	if _, err := exec.LookPath("java"); err != nil {
		t.Skip("Java is not installed")
	}
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
	if _, err := exec.LookPath("java"); err != nil {
		t.Skip("Java is not installed")
	}
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
		if _, err := exec.LookPath(binary); err != nil {
			t.Skipf("%s is not installed", binary)
		}
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
