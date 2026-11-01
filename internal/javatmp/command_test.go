package javatmp

import (
	"strings"
	"testing"
)

func TestEnvironmentRemovesOnlyJavaOptions(t *testing.T) {
	t.Setenv("_JAVA_OPTIONS", "-Djava.io.tmpdir=/shared")
	t.Setenv("JAVA_TOOL_OPTIONS", "-Dtest.tool=present")
	t.Setenv("JDK_JAVA_OPTIONS", "-Dtest.jdk=present")
	seen := make(map[string]bool)
	for _, entry := range Environment() {
		name, value, _ := strings.Cut(entry, "=")
		seen[name] = value != ""
	}
	if seen["_JAVA_OPTIONS"] || !seen["JAVA_TOOL_OPTIONS"] || !seen["JDK_JAVA_OPTIONS"] {
		t.Fatalf("unexpected Java environment: %v", seen)
	}
}
