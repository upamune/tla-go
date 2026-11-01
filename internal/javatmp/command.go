// Package javatmp isolates Java subprocess environments.
package javatmp

import (
	"os/exec"
	"strings"
)

// Apply removes _JAVA_OPTIONS, which the JVM processes after command-line
// options. Set Dir and any explicit Env before calling Apply. Use only for
// Java processes.
func Apply(cmd *exec.Cmd) *exec.Cmd {
	var result []string
	for _, entry := range cmd.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !strings.EqualFold(name, "_JAVA_OPTIONS") {
			result = append(result, entry)
		}
	}
	cmd.Env = result
	return cmd
}
