package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/upamune/tla-go/internal/javatmp"
)

var metadataVersionLine = regexp.MustCompile(`(?m)^TLC2 Version [^\n]*\n`)

var metadataTimestamp = regexp.MustCompile(`\d{2}-\d{2}-\d{2}-\d{2}-\d{2}-\d{2}`)

// TLC creates the expanded dump during parameter handling, before SANY runs.
func TestJavaTLCMetadataDumpPathError(t *testing.T) {
	for _, tc := range []struct {
		name string
		tool bool
		dot  bool
		meta bool
	}{
		{"default-dot-tool", true, true, false},
		{"default-text-tool", true, false, false},
		{"explicit-dot-tool", true, true, true},
		{"default-dot-ordinary", false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			goDir, javaDir := t.TempDir(), t.TempDir()
			for _, dir := range []string{goDir, javaDir} {
				writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n")
			}
			args := metadataDumpErrorArgs(tc.tool, tc.dot, tc.meta, goDir)
			var out, errout bytes.Buffer
			code := run(context.Background(), args, &out, &errout)
			if code != 1 || errout.Len() != 0 || strings.Contains(out.String(), "Starting SANY") {
				t.Fatalf("Go exit %d stdout=%s stderr=%s", code, out.String(), errout.String())
			}
			if tc.tool {
				messages := readToolMessages(out.String())
				if len(messages) != 2 || messages[1].code != 1102 {
					t.Fatalf("Go option messages: %v", messages)
				}
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				javaArgs := metadataDumpErrorArgs(tc.tool, tc.dot, tc.meta, javaDir)
				cmd := exec.Command("java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-jar", jar}, javaArgs...)...)
				cmd.Dir = javaDir
				output, err := javatmp.Apply(cmd).CombinedOutput()
				if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code {
					t.Fatalf("Java exit %v output=%s", err, output)
				}
				got := normalizeMetadataError(out.String(), goDir)
				want := normalizeMetadataError(string(output), javaDir)
				if tc.tool {
					got = stripMetadataBanner(got)
					want = stripMetadataBanner(want)
				} else {
					want = metadataVersionLine.ReplaceAllString(want, "")
				}
				if got != want {
					t.Fatalf("Go output:\n%s\nJava output:\n%s", got, want)
				}
			}
		})
	}
}

func metadataDumpErrorArgs(tool, dot, meta bool, dir string) []string {
	var args []string
	if tool {
		args = append(args, "-tool")
	}
	args = append(args, "-workers", "1", "-checkpoint", "0")
	if meta {
		args = append(args, "-metadir", filepath.Join(dir, "meta"))
	}
	args = append(args, "-dump")
	if dot {
		args = append(args, "dot")
	}
	return append(args, "${metadir}/nonexistent/graph", filepath.Join(dir, "M.tla"))
}

func normalizeMetadataError(output, dir string) string {
	output = strings.ReplaceAll(output, dir, "<ROOT>")
	return metadataTimestamp.ReplaceAllString(output, "<TIME>")
}

func stripMetadataBanner(output string) string {
	start := strings.Index(output, "@!@!@STARTMSG 2262:0 @!@!@")
	end := strings.Index(output, "@!@!@ENDMSG 2262 @!@!@\n")
	if start < 0 || end < start {
		return output
	}
	return output[:start] + output[end+len("@!@!@ENDMSG 2262 @!@!@\n"):]
}

func TestMetadataDumpFailureDoesNotFallBackToUnresolvedPath(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n")
	if err := os.WriteFile(filepath.Join(dir, "states"), []byte("sentinel"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-tool", "-checkpoint", "1", "-dump", "dot", "${metadir}/graph", filepath.Join(dir, "M.tla")}, &out, &errout)
	if code != 255 || strings.Contains(out.String(), "Starting SANY") || !strings.Contains(errout.String(), "Cannot create metadata directory") {
		t.Fatalf("exit %d stdout=%s stderr=%s", code, out.String(), errout.String())
	}
	if content, err := os.ReadFile(filepath.Join(dir, "states")); err != nil || string(content) != "sentinel" {
		t.Fatalf("metadata root changed: %q %v", content, err)
	}
}
