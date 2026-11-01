package javatmp

import (
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func TestApplyPreservesEnvironmentAndWorkingDirectory(t *testing.T) {
	t.Setenv("_JAVA_OPTIONS", "-Djava.io.tmpdir=/shared")
	t.Setenv("JAVA_TOOL_OPTIONS", "-Dtest.tool=present")
	t.Setenv("JDK_JAVA_OPTIONS", "-Dtest.jdk=present")
	cmd := exec.Command("java")
	dir := t.TempDir()
	cmd.Dir = dir
	Apply(cmd)
	seen := make(map[string]string)
	for _, entry := range cmd.Env {
		name, value, _ := strings.Cut(entry, "=")
		seen[name] = value
	}
	if _, exists := seen["_JAVA_OPTIONS"]; exists || seen["JAVA_TOOL_OPTIONS"] == "" || seen["JDK_JAVA_OPTIONS"] == "" {
		t.Fatalf("unexpected Java environment: %v", seen)
	}
	if runtime.GOOS != "windows" && runtime.GOOS != "plan9" && seen["PWD"] != dir {
		t.Fatalf("child PWD %q does not match %q", seen["PWD"], dir)
	}
}
