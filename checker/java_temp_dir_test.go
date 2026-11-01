package checker

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/upamune/tla-go/internal/javatmp"
)

func TestJavaNameResolutionUsesIsolatedTemporaryDirectories(t *testing.T) {
	requireWorkingJava(t)
	var directories [2]string
	for i := range directories {
		out, code := runNameResolutionJava(t, t.TempDir(), "-XshowSettings:properties", "-version")
		if code != 0 {
			t.Fatalf("Java exit %d: %s", code, out)
		}
		for line := range strings.SplitSeq(out, "\n") {
			if value, ok := strings.CutPrefix(strings.TrimSpace(line), "java.io.tmpdir = "); ok {
				directories[i] = value
				break
			}
		}
		if info, err := os.Stat(directories[i]); err != nil || !info.IsDir() {
			t.Fatalf("temporary directory %q: %v", directories[i], err)
		}
	}
	if directories[0] == directories[1] {
		t.Fatalf("Java invocations share a temporary directory: %q", directories[0])
	}
}

func TestJavaNameResolutionTemporaryDirectoryWithSpaces(t *testing.T) {
	requireWorkingJava(t)
	const marker = "TLA_GO_JAVA_TEMP_SPACE_CHILD"
	const rootKey = "TLA_GO_JAVA_TEMP_SPACE_ROOT"
	if os.Getenv(marker) == "1" {
		out, code := runNameResolutionJava(t, t.TempDir(), "-XshowSettings:properties", "-version")
		if code != 0 {
			t.Fatalf("Java exit %d: %s", code, out)
		}
		for line := range strings.SplitSeq(out, "\n") {
			if value, ok := strings.CutPrefix(strings.TrimSpace(line), "java.io.tmpdir = "); ok {
				relative, err := filepath.Rel(os.Getenv(rootKey), value)
				if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) || !strings.Contains(value, " ") {
					t.Fatalf("Java temporary directory %q is not inside the requested directory: %v", value, err)
				}
				return
			}
		}
		t.Fatalf("Java did not report a temporary directory: %s", out)
	}
	root := filepath.Join(t.TempDir(), "java temporary directory")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestJavaNameResolutionTemporaryDirectoryWithSpaces$")
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		switch strings.ToUpper(key) {
		case "TMPDIR", "TMP", "TEMP", marker, rootKey:
		default:
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "TMPDIR="+root, "TMP="+root, "TEMP="+root, marker+"=1", rootKey+"="+root)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("space-path subprocess: %v\n%s", err, out)
	}
}

func requireWorkingJava(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := javatmp.Apply(exec.CommandContext(ctx, "java", "-version")).Run(); err != nil {
		t.Skipf("Java is not available: %v", err)
	}
}

func TestJavaNameResolutionConcurrentTemporaryDirectories(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("set TLC_JAR to pinned Java TLC 1.7.4")
	}
	data, err := os.ReadFile(jar)
	if err != nil || fmt.Sprintf("%x", sha256.Sum256(data)) != "936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88" {
		t.Fatalf("TLC_JAR must be the pinned 1.7.4 jar: %v", err)
	}
	jar, err = filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	var seen sync.Map
	workers := make(chan struct{}, 4)
	for i := range 8 {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			workers <- struct{}{}
			defer func() { <-workers }()
			dir := t.TempDir()
			source := "---- MODULE M ----\nEXTENDS Naturals\nF == 1 + 1\n====\n"
			if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			out, code := runNameResolutionJava(t, dir, "-XshowSettings:properties", "-cp", jar, "tla2sany.SANY", "M.tla")
			if code != 0 || strings.Contains(out, "Fatal errors") || strings.Contains(out, "Semantic errors:") || !strings.Contains(out, "Semantic processing of module M") {
				t.Fatalf("Java exit %d: %s", code, out)
			}
			for line := range strings.SplitSeq(out, "\n") {
				if value, ok := strings.CutPrefix(strings.TrimSpace(line), "java.io.tmpdir = "); ok {
					if _, loaded := seen.LoadOrStore(value, true); loaded {
						t.Fatalf("Java invocations share a temporary directory: %q", value)
					}
					return
				}
			}
			t.Fatalf("Java did not report a temporary directory: %s", out)
		})
	}
}
