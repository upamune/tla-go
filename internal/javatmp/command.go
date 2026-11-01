// Package javatmp isolates Java subprocess environments.
package javatmp

import (
	"os/exec"
	"runtime"
	"strings"
)

// Apply removes _JAVA_OPTIONS, which the JVM processes after command-line
// options. Set Dir and any explicit Env before calling Apply. Use only for
// Java processes.
func Apply(cmd *exec.Cmd) *exec.Cmd {
	environ := cmd.Env
	if environ == nil {
		environ = cmd.Environ()
	}
	result := make([]string, 0, len(environ))
	for _, entry := range environ {
		name, _, _ := strings.Cut(entry, "=")
		isOptions := name == "_JAVA_OPTIONS" || (runtime.GOOS == "windows" && strings.EqualFold(name, "_JAVA_OPTIONS"))
		if !isOptions {
			result = append(result, entry)
		}
	}
	cmd.Env = result
	return cmd
}
