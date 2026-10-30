package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestJavaTLCTemporalTautology(t *testing.T) {
	for _, tc := range []struct {
		name, property string
	}{
		{"upstream EmptyOrderOfSolutions", "<>[]TRUE => TRUE"},
		{"empty quantifier", "~(\\E i \\in {} : <>[](x = i))"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == x = 0\nNext == x' = 1\nSpec == Init /\\ [][Next]_x\nProp == " + tc.property
			writeDOTFixture(t, dir, body, "SPECIFICATION Spec\nPROPERTY Prop\n")
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", filepath.Join(dir, "M.tla")}, &out, &errout)
			messages := readToolMessages(out.String())
			if code != 77 || errout.Len() != 0 || !hasToolMessage(messages, 2253) || hasToolMessage(messages, 2199) {
				t.Fatalf("exit %d; stdout: %s; stderr: %s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaToolMessages(t, jar, dir, code, messages)
			}
		})
	}
}

// The v1.8.0 release rejects this formula earlier than v1.7.4.
func TestJavaTLCModernTemporalQuantifier(t *testing.T) {
	jar := os.Getenv("TLC_MODERN_JAR")
	if jar == "" {
		t.Skip("TLC_MODERN_JAR is not set")
	}
	dir := t.TempDir()
	body := "VARIABLE x\nInit == x = 0\nNext == x' = 1\nSpec == Init /\\ [][Next]_x\nProp == ~(\\E i \\in {} : <>[](x = i))"
	writeDOTFixture(t, dir, body, "SPECIFICATION Spec\nPROPERTY Prop\n")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "M.tla")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil || cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 255 || !hasToolMessage(readToolMessages(string(output)), 2214) {
		t.Fatalf("Java TLC: %v\n%s", err, output)
	}
}

// EmptyOrderOfSolutions is an upstream Java TLC test-model from v1.7.4.
func TestJavaTLCUpstreamTemporalTautology(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"EmptyOrderOfSolutions.tla", "EmptyOrderOfSolutions.cfg"} {
		data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-tool", filepath.Join(dir, "EmptyOrderOfSolutions.tla")}, &out, &errout)
	if code != 77 || errout.Len() != 0 || !hasToolMessage(readToolMessages(out.String()), 2253) {
		t.Fatalf("exit %d: %s %s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		runJavaTLCForExit(t, jar, dir, "EmptyOrderOfSolutions", 77)
	}
}
