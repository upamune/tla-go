package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// InfiniteStateSpace and CheckpointWhenTimeBoundTest are from TLC 1.7.4,
// upstream commit 5a47802b5c391f59ecdd44117981f4ff8c0656ba. This checks
// selected stop events, not the upstream JUnit test or all of its assertions.
func TestJavaTLCTimedCheckpoint(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	data, err := os.ReadFile(jar)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != "936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88" {
		t.Fatal("TLC_JAR must be the pinned Java TLC 1.7.4 release")
	}
	dir := t.TempDir()
	spec := filepath.Join(dir, "InfiniteStateSpace.tla")
	source := "---- MODULE InfiniteStateSpace ----\nEXTENDS Naturals\nVARIABLES x,y\nSpec == x = 0 /\\ y \\in 1..1000 /\\ [][x'= x + 1 /\\ UNCHANGED y]_<<x,y>>\n====\n"
	if err := os.WriteFile(spec, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "InfiniteStateSpace.cfg"), []byte("SPECIFICATION Spec\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	binary := buildTimedStopTestCLI(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-Xmx256m", "-Dtlc2.TLC.stopAfter=1", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "-checkpoint", "35791", "-metadir", filepath.Join(dir, "java"), spec)
	javaOut, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Java TLC: %v\n%s", err, javaOut)
	}
	code, out, errout := runTimedStopTestCLI(ctx, binary, "1ns", "-tool", "-workers", "1", "-fp", "0", "-checkpoint", "35791", "-metadir", filepath.Join(dir, "go"), spec)
	if code != 255 || !strings.Contains(errout, "search incomplete") {
		t.Fatalf("tla-go: %d\n%s\n%s", code, out, errout)
	}
	for _, marker := range []string{"STARTMSG 2195:", "STARTMSG 2196:", "STARTMSG 2186:"} {
		if !bytes.Contains(javaOut, []byte(marker)) || !strings.Contains(out, marker) {
			t.Errorf("missing %s\nJava: %s\nGo: %s", marker, javaOut, out)
		}
	}
}
