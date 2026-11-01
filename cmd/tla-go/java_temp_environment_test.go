package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// JVM environment options have their own quoting rules, unlike argv. Verify
// both oracle paths when Go's temporary root contains spaces and quotes.
func TestJavaTLCTemporaryEnvironmentQuotedPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not allow quotes in path components")
	}
	if pinnedFairnessEffectJar(t) == "" {
		t.Skip("set TLC_JAR to pinned Java TLC 1.7.4")
	}
	root := filepath.Join(t.TempDir(), `quoted " and ' directory`)
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.v", "-test.run=^TestJavaTLC(TemporaryEnvironmentIsolation|StatechartBridgeTemporaryEnvironmentIsolation)$")
	cmd.Env = append(os.Environ(), "TMPDIR="+root, "TMP="+root, "TEMP="+root, "TLA_GO_STATECHART_BRIDGE=0")
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "--- PASS: TestJavaTLCTemporaryEnvironmentIsolation") ||
		!strings.Contains(string(out), "--- PASS: TestJavaTLCStatechartBridgeTemporaryEnvironmentIsolation") {
		t.Fatalf("quoted temporary path: %v\n%s", err, out)
	}
}

// A caller's _JAVA_OPTIONS must not override the oracle's isolated tmpdir.
func TestJavaTLCTemporaryEnvironmentIsolation(t *testing.T) {
	jar := pinnedFairnessEffectJar(t)
	if jar == "" {
		t.Skip("set TLC_JAR to pinned Java TLC 1.7.4")
	}
	dir := t.TempDir()
	shared := filepath.Join(dir, "shared")
	if err := os.Mkdir(shared, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(shared, "Naturals.tla")
	const sentinel = "do not overwrite\n"
	if err := os.WriteFile(marker, []byte(sentinel), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("_JAVA_OPTIONS", "-Djava.io.tmpdir=shared -Dreview.user.option=present")
	t.Setenv("JAVA_TOOL_OPTIONS", "-Djava.io.tmpdir=shared -Dreview.tool.option=present")
	t.Setenv("JDK_JAVA_OPTIONS", "-Djava.io.tmpdir=shared -Dreview.jdk.option=present")
	model := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\n====\n"
	for name, content := range map[string]string{"M.tla": model, "M.cfg": "INIT Init\nNEXT Next\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	output, code := runJavaTLCArgs(t, jar, dir, []string{"-tool", "-workers", "1", "-fp", "0", "M.tla"})
	assertSharedMarker(t, marker, sentinel)
	if code != 0 || !strings.Contains(output, "Model checking completed") || !preservedJavaOptions(output) {
		t.Fatalf("Java exit %d output %s", code, output)
	}
	if !strings.Contains(os.Getenv("_JAVA_OPTIONS"), "review.user.option") {
		t.Fatal("parent environment was changed")
	}
}

// The external statechart runner supplies its own JVM tmpdir before -cp. The
// bridge must keep that argument and remove the conflicting inherited option.
func TestJavaTLCStatechartBridgeTemporaryEnvironmentIsolation(t *testing.T) {
	jar := pinnedFairnessEffectJar(t)
	if jar == "" {
		t.Skip("set TLC_JAR to pinned Java TLC 1.7.4")
	}
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	shared, private := filepath.Join(dir, "shared"), filepath.Join(dir, "private")
	for _, path := range []string{shared, private} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	marker := filepath.Join(shared, "Naturals.tla")
	const sentinel = "do not overwrite\n"
	if err := os.WriteFile(marker, []byte(sentinel), 0o600); err != nil {
		t.Fatal(err)
	}
	model := filepath.Join(dir, "M.tla")
	if err := os.WriteFile(model, []byte("---- MODULE M ----\nEXTENDS Naturals\nX == 1 + 1\n====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("_JAVA_OPTIONS", "-Djava.io.tmpdir=shared")
	t.Setenv("JAVA_TOOL_OPTIONS", "-Djava.io.tmpdir=shared -Dreview.tool.option=present")
	t.Setenv("JDK_JAVA_OPTIONS", "-Djava.io.tmpdir=shared -Dreview.jdk.option=present")
	t.Setenv("TLA_GO_STATECHART_BRIDGE", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-Djava.io.tmpdir="+private, "-cp", jar, "tla2sany.xml.XMLExporter", "-o", "-I", dir, model)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	assertSharedMarker(t, marker, sentinel)
	if err != nil || !strings.Contains(string(output), "<") || !preservedJavaOptions(string(output)) {
		t.Fatalf("bridge: %v, output %s", err, output)
	}
}

func preservedJavaOptions(output string) bool {
	return !strings.Contains(output, "Picked up _JAVA_OPTIONS") &&
		strings.Contains(output, "review.tool.option=present") && strings.Contains(output, "review.jdk.option=present")
}

func assertSharedMarker(t *testing.T, path, want string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil || string(content) != want {
		t.Fatalf("shared marker %s: %q, %v", path, content, err)
	}
}
