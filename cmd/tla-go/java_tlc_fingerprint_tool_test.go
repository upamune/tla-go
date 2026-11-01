package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/upamune/tla-go/internal/javatmp"
)

// The models come from Java TLC v1.7.4's fingerprint failure tests.
func TestJavaTLCFingerprintToolFailures(t *testing.T) {
	for _, name := range []string{"EmptySubsetEq", "FingerprintExceptionInit", "FingerprintExceptionNext"} {
		t.Run(name, func(t *testing.T) {
			dir := copyIncompleteFixture(t, name)
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", filepath.Join(dir, name+".tla")}, &out, &errout)
			got := fingerprintToolMessages(readToolMessages(out.String()))
			if code != 75 || errout.Len() != 0 || !hasToolMessage(got, 2147) {
				t.Fatalf("exit %d; stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaFingerprintToolMessages(t, jar, dir, name, got)
			}
		})
	}
}

func TestJavaTLCInitialFingerprintUnexpectedToolCodes(t *testing.T) {
	dir := t.TempDir()
	body := "VARIABLE x\nInit == x = SUBSET ((SUBSET (1..3)) \\intersect (1..4))\nNext == UNCHANGED x"
	writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\n")
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout)
	got := fingerprintToolMessages(readToolMessages(out.String()))
	if code != 75 || errout.Len() != 0 || len(got) != 2 || got[0].code != 1000 || got[1].code != 2147 {
		t.Fatalf("exit %d; stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareJavaFingerprintToolMessages(t, jar, dir, "M", got)
	}
}

func fingerprintToolMessages(messages []toolMessage) []toolMessage {
	var selected []toolMessage
	for _, message := range messages {
		if message.code == 1000 || message.code == 2102 || message.code == 2147 {
			selected = append(selected, message)
		}
	}
	return selected
}

func compareJavaFingerprintToolMessages(t *testing.T, jar, dir, name string, got []toolMessage) {
	t.Helper()
	out := runJavaFingerprintTool(t, jar, dir, name)
	want := fingerprintToolMessages(readToolMessages(out))
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Go: %+v\nJava: %+v", got, want)
	}
}

func runJavaFingerprintTool(t *testing.T, jar, dir, name string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "-metadir", t.TempDir(), name+".tla")
	cmd.Dir = dir
	out, err := javatmp.Apply(cmd).CombinedOutput()
	if ctx.Err() != nil || cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 75 {
		t.Fatalf("Java TLC: %v\n%s", err, out)
	}
	return string(out)
}
