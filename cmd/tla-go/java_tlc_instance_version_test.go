package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestJavaTLCParameterizedInstanceVersion(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"test61.tla", "test61.cfg", "test61a.tla"} {
		data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", filepath.Join(dir, "test61.tla")}, &out, &errout)
	if code != 0 || !strings.Contains(out.String(), "6 states generated, 5 distinct states found") {
		t.Fatalf("Go exit %d: %s %s", code, out.String(), errout.String())
	}
	for _, env := range []string{"TLC_JAR", "TLC_MODERN_JAR"} {
		jar := os.Getenv(env)
		if jar == "" {
			continue
		}
		cmd := exec.Command("java", "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "test61.tla")
		cmd.Dir = dir
		java, err := cmd.CombinedOutput()
		if cmd.ProcessState == nil || !instanceVersionResult(env, cmd.ProcessState.ExitCode(), string(java)) {
			t.Fatalf("%s: %v\n%s", env, err, java)
		}
	}
}

func instanceVersionResult(env string, code int, output string) bool {
	if code == 0 {
		return strings.Contains(output, "6 states generated, 5 distinct states found")
	}
	return env == "TLC_JAR" && code == 151 && strings.Contains(output, "The configuration file did not specify the initial state predicate")
}
