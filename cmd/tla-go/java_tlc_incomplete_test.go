package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The source, expected diagnostics, traces and counts come from TLC's
// IncompleteNextTest and IncompleteNextMultipleActionsTest at v1.7.4.
func TestJavaTLCIncompleteNext(t *testing.T) {
	cases := []struct {
		name, message, label, initial, state string
	}{
		{"IncompleteNext", "the next-state action. The following variable is not assigned: y.", "Action line 6, col 30 to line 6, col 35 of module IncompleteNext", "/\\ x = 0\n/\\ y = 0", "/\\ x = 1\n/\\ y = null"},
		{"IncompleteNextMultipleActions", "action A1 of the next-state relation. The following variables are not assigned: y, z.", "A1 line 6, col 7 to line 6, col 15 of module IncompleteNextMultipleActions", "/\\ x = 0\n/\\ y = 0\n/\\ z = 0", "/\\ x = 1\n/\\ y = null\n/\\ z = null"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := copyIncompleteFixture(t, tc.name)
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, tc.name+".tla")}, &out, &errout)
			for _, want := range []string{
				"Error: Successor state is not completely specified by " + tc.message,
				"State 1: <Initial predicate>\n" + tc.initial,
				"State 2: <" + tc.label + ">\n" + tc.state,
				"2 states generated, 1 distinct states found, 0 states left on queue.",
				"The depth of the complete state graph search is 1.",
			} {
				if !strings.Contains(out.String(), want) {
					t.Errorf("missing %q in:\n%s", want, out.String())
				}
			}
			if code != 75 || errout.Len() != 0 {
				t.Fatalf("exit %d, stderr %q", code, errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runIncompleteJava(t, jar, dir, tc.name, 75)
				if got, want := incompleteOutput(out.String(), "Error: Successor state"), incompleteOutput(java, "Error: Successor state"); got != want {
					t.Fatalf("Go output:\n%s\nJava output:\n%s", got, want)
				}
			}
		})
	}
}

func TestJavaTLCIncompleteInit(t *testing.T) {
	dir := t.TempDir()
	model := `---- MODULE IncompleteInit ----
EXTENDS Integers
VARIABLES x, y
Init == x = 0
Next == UNCHANGED <<x,y>>
====`
	for name, content := range map[string]string{"IncompleteInit.tla": model, "IncompleteInit.cfg": "INIT Init\nNEXT Next\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{filepath.Join(dir, "IncompleteInit.tla")}, &out, &errout)
	want := "Error: current state is not a legal state\nWhile working on the initial state:\n/\\ x = 0\n/\\ y = null"
	if code != 255 || errout.Len() != 0 || !strings.Contains(out.String(), want) || strings.Contains(out.String(), "states generated") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java := runIncompleteJava(t, jar, dir, "IncompleteInit", 255)
		if got, want := incompleteOutput(out.String(), "Error: current state"), incompleteOutput(java, "Error: current state"); got != want {
			t.Fatalf("Go output:\n%s\nJava output:\n%s", got, want)
		}
	}
}

func TestJavaTLCIncompleteSplitActions(t *testing.T) {
	cases := []struct {
		name, next, diagnostic, location string
	}{
		{"ExistSingle", `\E n \in {1} : x' = n`, "the next-state action", "Next line 5, col 24 to line 5, col 29 of module ExistSingle"},
		{"ExistNext", `\E n \in {1,2} : x' = n`, "action Next of the next-state relation", "Next line 5, col 26 to line 5, col 31 of module ExistNext"},
		{"StateBound", `\E n \in {x,x+1} : x' = n`, "the next-state action", "Next line 5, col 9 to line 5, col 33 of module StateBound"},
		{"OrNext", `x' = 1 \/ x' = 2`, "action Next of the next-state relation", "Next line 5, col 9 to line 5, col 14 of module OrNext"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := fmt.Sprintf("---- MODULE %s ----\nEXTENDS Integers\nVARIABLES x, y\nInit == /\\ x = 0 /\\ y = 0\nNext == %s\n====\n", tc.name, tc.next)
			for name, content := range map[string]string{tc.name + ".tla": source, tc.name + ".cfg": "INIT Init\nNEXT Next\n"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, tc.name+".tla")}, &out, &errout)
			message := "Error: Successor state is not completely specified by " + tc.diagnostic + ". The following variable is not assigned: y."
			if code != 75 || errout.Len() != 0 || !strings.Contains(out.String(), message) || !strings.Contains(out.String(), "State 2: <"+tc.location+">") {
				t.Fatalf("exit %d, stdout %q, stderr %q", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runIncompleteJava(t, jar, dir, tc.name, 75)
				if got, want := incompleteOutput(out.String(), "Error: Successor state"), incompleteOutput(java, "Error: Successor state"); got != want {
					t.Fatalf("Go output:\n%s\nJava output:\n%s", got, want)
				}
			}
		})
	}
}

func TestJavaTLCIncompleteUnnamedActions(t *testing.T) {
	dir := t.TempDir()
	source := `---- MODULE OrSpec ----
EXTENDS Integers
VARIABLES x, y
Spec == /\ x = 0 /\ y = 0 /\ [][x' = 1 \/ x' = 2]_<<x,y>>
====`
	for name, content := range map[string]string{"OrSpec.tla": source, "OrSpec.cfg": "SPECIFICATION Spec\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{filepath.Join(dir, "OrSpec.tla")}, &out, &errout)
	message := "Error: Successor state is not completely specified by action UnnamedAction of the next-state relation. The following variable is not assigned: y."
	location := "State 2: <Action line 4, col 33 to line 4, col 38 of module OrSpec>"
	if code != 75 || errout.Len() != 0 || !strings.Contains(out.String(), message) || !strings.Contains(out.String(), location) {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java := runIncompleteJava(t, jar, dir, "OrSpec", 75)
		if got, want := incompleteOutput(out.String(), "Error: Successor state"), incompleteOutput(java, "Error: Successor state"); got != want {
			t.Fatalf("Go output:\n%s\nJava output:\n%s", got, want)
		}
	}
}

func copyIncompleteFixture(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	for _, ext := range []string{".tla", ".cfg"} {
		path := filepath.Join("..", "..", "checker", "testdata", "java-tlc", name+ext)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+ext), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func runIncompleteJava(t *testing.T, jar, dir, name string, code int) string {
	t.Helper()
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-workers", "1", "-fp", "0", name+".tla")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if ctx.Err() != nil || !ok || exit.ExitCode() != code {
		t.Fatalf("Java TLC: %v, context: %v\n%s", err, ctx.Err(), out)
	}
	return string(out)
}

func incompleteOutput(output, headline string) string {
	start := strings.Index(output, headline)
	if start < 0 {
		return ""
	}
	var lines []string
	for line := range strings.SplitSeq(output[start:], "\n") {
		if strings.HasPrefix(line, "Finished in ") {
			break
		}
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}
