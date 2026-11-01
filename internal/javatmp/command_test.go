package javatmp

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func TestApplyEmptyEnvironmentDoesNotInherit(t *testing.T) {
	t.Setenv("TLA_JAVA_ENV_SENTINEL", "present")
	cmd := exec.Command(os.Args[0], "-test.run=^TestApplyEnvironmentChild$", "-test.v")
	cmd.Env = []string{"_JAVA_OPTIONS=-Dexample=1"}
	Apply(cmd)
	if cmd.Env == nil {
		t.Fatal("empty environment became nil")
	}
	output, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "environment child checked") {
		t.Fatalf("child inherited environment or failed: %v\n%s", err, output)
	}
}

func TestApplyEnvironmentNameCase(t *testing.T) {
	cmd := exec.Command("java")
	cmd.Env = []string{"_java_options=lowercase"}
	Apply(cmd)
	found := false
	for _, entry := range cmd.Env {
		found = found || entry == "_java_options=lowercase"
	}
	if found == (runtime.GOOS == "windows") {
		t.Fatalf("unexpected lowercase option in %v", cmd.Env)
	}
}

func TestApplyPreservesInvalidExplicitEnvironment(t *testing.T) {
	if runtime.GOOS == "plan9" {
		t.Skip("Plan 9 permits NULs in environment values")
	}
	for _, invalid := range []string{"BAD=contains\x00nul", "_JAVA_OPTIONS=contains\x00nul"} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestApplyEnvironmentChild$")
		cmd.Env = []string{invalid}
		Apply(cmd)
		if err := cmd.Run(); err == nil {
			t.Fatalf("expected invalid environment %q to reject execution", invalid)
		}
	}
}

func TestApplyEnvironmentChild(t *testing.T) {
	if !strings.Contains(strings.Join(os.Args, " "), "^TestApplyEnvironmentChild$") {
		t.Skip("child process only")
	}
	if os.Getenv("TLA_JAVA_ENV_SENTINEL") != "" || os.Getenv("_JAVA_OPTIONS") != "" {
		t.Fatal("child inherited parent variables")
	}
	t.Log("environment child checked")
}

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
