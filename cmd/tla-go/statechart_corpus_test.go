package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/upamune/tla-go/internal/javatmp"
)

// This optional test uses the actual statechart parser and corpus rather than
// reproducing its interpretation of the TLC protocol in Go.
func TestJavaTLCStatechartCorpus(t *testing.T) {
	root, jar := os.Getenv("STATECHART_DIR"), os.Getenv("TLC_JAR")
	if root == "" || jar == "" {
		t.Skip("set STATECHART_DIR and TLC_JAR to run statechart differential tests")
	}
	if _, err := exec.LookPath("bun"); err != nil {
		t.Fatal("statechart differential tests require bun")
	}
	entries, err := os.ReadDir(filepath.Join(root, "scripts", "test", "corpus"))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		count++
		t.Run(entry.Name(), func(t *testing.T) {
			dir := copyStatechartModel(t, filepath.Join(root, "scripts", "test", "corpus", entry.Name()))
			compareStatechartModel(t, root, jar, dir, entry.Name())
		})
	}
	if count == 0 {
		t.Fatal("statechart corpus contains no models")
	}
}

func TestJavaTLCStatechartMarks(t *testing.T) {
	root, jar := os.Getenv("STATECHART_DIR"), os.Getenv("TLC_JAR")
	if root == "" || jar == "" {
		t.Skip("set STATECHART_DIR and TLC_JAR to run statechart differential tests")
	}
	dir := t.TempDir()
	body := "VARIABLE x\nInit == x=0 /\\ PrintT(<<\"tla-go-test-tag\", <<x>>, {\"Rule\"}>>)\nNext == UNCHANGED x"
	writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\n")
	compareStatechartModel(t, root, jar, dir, "M")
}

func copyStatechartModel(t *testing.T, source string) string {
	t.Helper()
	dir := t.TempDir()
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tla") && !strings.HasSuffix(entry.Name(), ".cfg") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(source, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, entry.Name()), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func compareStatechartModel(t *testing.T, root, jar, dir, name string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	flags := []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final"}
	args := append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar}, flags...)
	cmd := exec.CommandContext(ctx, "java", append(args, name+".tla")...)
	cmd.Dir = dir
	javaOutput, javaErr := javatmp.Apply(cmd).CombinedOutput()
	if ctx.Err() != nil || cmd.ProcessState == nil {
		t.Fatalf("Java TLC: %v", javaErr)
	}
	var out, errout bytes.Buffer
	goCode := run(ctx, append(flags, filepath.Join(dir, name+".tla")), &out, &errout)
	if goCode != cmd.ProcessState.ExitCode() {
		t.Fatalf("exit codes differ: Java %d, Go %d\nGo stdout: %s\nGo stderr: %s", cmd.ProcessState.ExitCode(), goCode, out.String(), errout.String())
	}
	compareStatechartParsed(t, root, string(javaOutput), out.String()+errout.String())
}

func compareStatechartParsed(t *testing.T, root, java, goOutput string) {
	t.Helper()
	const script = `
import {pathToFileURL} from 'node:url';
import {join} from 'node:path';
const root = process.env.STATECHART_DIR;
const {readMessages} = await import(pathToFileURL(join(root, 'scripts/frontends/messages.ts')).href);
const {readVerdict} = await import(pathToFileURL(join(root, 'scripts/frontends/verdict.ts')).href);
const {readTrace} = await import(pathToFileURL(join(root, 'scripts/frontends/trace.ts')).href);
const {readMarks} = await import(pathToFileURL(join(root, 'scripts/frontends/marks.ts')).href);
const input = JSON.parse(await Bun.stdin.text());
const parse = output => { const parts = [...readMessages(output)]; return {verdict: readVerdict(parts), trace: readTrace(parts), marks: [...readMarks(parts, 'tla-go-test-tag')]}; };
console.log(JSON.stringify({java: parse(input.java), go: parse(input.go)}));`
	payload, err := json.Marshal(map[string]string{"java": java, "go": goOutput})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bun", "--eval", script)
	cmd.Env = append(os.Environ(), "STATECHART_DIR="+root)
	cmd.Stdin = bytes.NewReader(payload)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("statechart parser: %v\n%s", err, output)
	}
	var parsed struct{ Java, Go any }
	if err := json.Unmarshal(output, &parsed); err != nil {
		t.Fatalf("statechart parser output: %v\n%s", err, output)
	}
	if !reflect.DeepEqual(parsed.Java, parsed.Go) {
		t.Fatalf("statechart parser results differ\nJava: %s\nGo: %s", formatParsed(parsed.Java), formatParsed(parsed.Go))
	}
}

func formatParsed(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(data)
}
