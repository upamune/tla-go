package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Java TLC removes a trailing .cfg from -config and then appends .cfg, so
// upstream tests such as LoopTestWeakFair pass bare configuration names and
// Github798ITest's "-config Github798I.tla" reads Github798I.tla.cfg.
func TestJavaTLCConfigName(t *testing.T) {
	for _, tc := range []struct {
		name, config string
		code         int
		want         string
	}{
		{"bare name", "Other", 11, "Deadlock reached."},
		{"cfg extension", "Other.cfg", 11, "Deadlock reached."},
		{"tla extension", "M.tla", 255, "configuration file M.tla.cfg:\nFile not found."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == x < 1 /\\ x' = x + 1\nInv == x < 1", "INIT Init\nNEXT Next\nINVARIANT Inv\n")
			if err := os.WriteFile(filepath.Join(dir, "Other.cfg"), []byte("INIT Init\nNEXT Next\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-config", tc.config, "M.tla"}
			messages, code, stderr := runGoToolInDir(t, dir, args)
			if code != tc.code || stderr != "" || !hasToolMessageContaining(messages, tc.want) {
				t.Fatalf("exit %d, want %d; messages %+v; stderr %s", code, tc.code, messages, stderr)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				if javaCode != code || !reflect.DeepEqual(optionMessages(readToolMessages(java)), optionMessages(messages)) {
					t.Fatalf("Java exit %d, Go exit %d\nJava: %s\nGo: %+v", javaCode, code, java, optionMessages(messages))
				}
			}
		})
	}
}

// Java TLC resolves a relative configuration name in the working directory,
// or in the specification's directory for an absolute specification path,
// and then in the specification's directory.
func TestJavaTLCConfigSearchPath(t *testing.T) {
	for _, tc := range []struct {
		name       string
		inWorkDir  bool
		absolute   bool
		code       int
		configured string
	}{
		{"specification directory", false, false, 11, "Deadlock reached."},
		{"working directory first", true, false, 12, "Invariant Inv is violated."},
		{"absolute specification", true, true, 11, "Deadlock reached."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			sub := filepath.Join(dir, "sub")
			if err := os.Mkdir(sub, 0o700); err != nil {
				t.Fatal(err)
			}
			writeDOTFixture(t, sub, "VARIABLE x\nInit == x = 0\nNext == x < 1 /\\ x' = x + 1\nInv == x < 1", "INIT Init\nNEXT Next\n")
			if err := os.Rename(filepath.Join(sub, "M.cfg"), filepath.Join(sub, "Other.cfg")); err != nil {
				t.Fatal(err)
			}
			if tc.inWorkDir {
				if err := os.WriteFile(filepath.Join(dir, "Other.cfg"), []byte("INIT Init\nNEXT Next\nINVARIANT Inv\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			spec := filepath.Join("sub", "M.tla")
			if tc.absolute {
				spec = filepath.Join(sub, "M.tla")
			}
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-config", "Other", spec}
			messages, code, stderr := runGoToolInDir(t, dir, args)
			if code != tc.code || stderr != "" || !hasToolMessageContaining(messages, tc.configured) {
				t.Fatalf("exit %d, want %d; messages %+v; stderr %s", code, tc.code, messages, stderr)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				if _, javaCode := runJavaTLCArgs(t, jar, dir, args); javaCode != code {
					t.Fatalf("Java exit %d, Go exit %d", javaCode, code)
				}
			}
		})
	}
}

// runGoToolInDir runs tla-go with args relative to dir, as Java TLC runs
// from the model directory.
func runGoToolInDir(t *testing.T, dir string, args []string) ([]toolMessage, int, string) {
	t.Helper()
	output, code, stderr := runGoInDir(t, dir, args)
	return readToolMessages(output), code, stderr
}

func runGoInDir(t *testing.T, dir string, args []string) (string, int, string) {
	t.Helper()
	t.Chdir(dir)
	var out, errout bytes.Buffer
	code := run(context.Background(), args, &out, &errout)
	return out.String(), code, errout.String()
}

func runJavaTLCArgs(t *testing.T, jar, dir string, args []string) (string, int) {
	t.Helper()
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	javaArgs := append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-cp", jar, "tlc2.TLC"}, args...)
	cmd := exec.CommandContext(ctx, "java", javaArgs...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if exit, ok := errors.AsType[*exec.ExitError](err); ok && ctx.Err() == nil {
		return string(output), exit.ExitCode()
	}
	if err != nil {
		t.Fatalf("Java TLC: %v\n%s", err, output)
	}
	return string(output), 0
}

// optionMessages keeps the tool messages that describe results, removing
// version, timing, and progress messages.
func optionMessages(messages []toolMessage) []toolMessage {
	selected := toolConsumerMessages(messages)
	for _, message := range messages {
		if message.code == 1000 && !strings.Contains(message.body, "@!@!@STARTMSG 2132:0 @!@!@") {
			selected = append(selected, message)
		}
	}
	return selected
}

func hasToolMessageContaining(messages []toolMessage, text string) bool {
	for _, message := range messages {
		if strings.Contains(message.body, text) {
			return true
		}
	}
	return false
}
