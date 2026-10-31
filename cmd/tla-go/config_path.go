package main

import (
	"os"
	"path/filepath"
	"strings"
)

// configFilePath resolves a -config argument as Java TLC's
// SimpleFilenameToStream does. The .cfg extension is optional and is
// appended to any other name, including one ending in .tla. A relative name
// is searched in the working directory, or in the specification's directory
// when the specification path is absolute, then in the specification's
// directory. TLC.main searches the library paths only when the
// specification path is absolute or has no directory part. An absolute name
// is used as is, although Java TLC cannot find one when the specification
// path is also absolute. The first candidate is returned when none exists,
// so the error names the file.
func configFilePath(specPath, name string, libraries []string) string {
	name = strings.TrimSuffix(name, ".cfg") + ".cfg"
	if filepath.IsAbs(name) {
		return name
	}
	dir := filepath.Dir(specPath)
	candidates := []string{name, filepath.Join(dir, name)}
	if filepath.IsAbs(specPath) {
		candidates[0] = candidates[1]
	}
	if filepath.IsAbs(specPath) || !strings.ContainsRune(specPath, filepath.Separator) {
		for _, library := range libraries {
			candidates = append(candidates, filepath.Join(library, name))
		}
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return candidates[0]
}
