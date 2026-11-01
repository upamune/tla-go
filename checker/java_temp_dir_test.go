package checker

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestJavaNameResolutionUsesIsolatedTemporaryDirectories(t *testing.T) {
	if _, err := exec.LookPath("java"); err != nil {
		t.Skip("Java is not installed")
	}
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
