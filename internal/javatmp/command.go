// Package javatmp starts Java processes without a JVM option that can
// override their explicitly selected temporary directory.
package javatmp

import (
	"context"
	"os"
	"os/exec"
	"strings"
)

// Environment preserves the current environment except _JAVA_OPTIONS, which
// the JVM processes after command-line options. Use only for Java processes.
func Environment() []string {
	var result []string
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !strings.EqualFold(name, "_JAVA_OPTIONS") {
			result = append(result, entry)
		}
	}
	return result
}

// Command constructs a Java command with the isolated Java environment.
func Command(args ...string) *exec.Cmd {
	cmd := exec.Command("java", args...)
	cmd.Env = Environment()
	return cmd
}

// CommandContext constructs a context-bound Java command with the isolated Java environment.
func CommandContext(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "java", args...)
	cmd.Env = Environment()
	return cmd
}
