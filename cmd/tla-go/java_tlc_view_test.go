package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/upamune/tla-go/internal/javatmp"
)

// The expected states are those asserted by the upstream ViewMapTest.
func TestJavaTLCUpstreamViewMap(t *testing.T) {
	dir := copyJavaTLCFixture(t, "ViewMap")
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-tool", "-deadlock", "-view", filepath.Join(dir, "ViewMap.tla")}, &out, &errout)
	trace := []string{
		"/\\ buffer = <<>>\n/\\ waitset = {}", "/\\ buffer = <<>>\n/\\ waitset = {c1}", "/\\ buffer = <<>>\n/\\ waitset = {c1, c2}",
		"/\\ buffer = <<\"d\">>\n/\\ waitset = {c2}", "/\\ buffer = <<\"d\">>\n/\\ waitset = {c2, p1}", "/\\ buffer = << >>\n/\\ waitset = {p1}",
		"/\\ buffer = << >>\n/\\ waitset = {c1, p1}", "/\\ buffer = << >>\n/\\ waitset = {c1, c2, p1}",
	}
	var states []string
	for _, message := range readToolMessages(out.String()) {
		if message.code == 2217 {
			_, state, _ := strings.Cut(message.body, "\n")
			states = append(states, state)
		}
	}
	if code != 12 || errout.Len() != 0 || strings.Join(states, "|") != strings.Join(trace, "|") {
		t.Fatalf("exit %d, states %q; stdout:\n%s\nstderr:\n%s", code, states, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareJavaViewTrace(t, jar, dir, "ViewMap", out.String(), code)
	}
}

// Java TLC labels a tuple-valued view by the constants and variables it
// mentions, in variable order, and prints any other view value directly.
func TestJavaTLCViewStates(t *testing.T) {
	for _, tc := range []struct{ name, view, want string }{
		{"variable", "x", "\n1\n"},
		{"singleton tuple", "<<x>>", "\n/\\ x = 1\n\n"},
		{"reordered tuple", "<<y, x>>", "\n/\\ x = 0\n/\\ y = 1\n\n"},
		{"constant", "<<x, K>>", "\n/\\ K = 1\n/\\ x = 7\n\n"},
		{"record", "[a |-> x, b |-> y]", "\n[a |-> 1, b |-> 0]\n"},
		{"let", "LET z == x IN <<z, y>>", "\n<<1, 0>>\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "CONSTANT K\nVARIABLES x, y\nInit == x = 0 /\\ y = 0\nNext == x < 3 /\\ x' = x + 1 /\\ y' = y\nInv == x < 2\nV == " + tc.view
			writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nINVARIANT Inv\nCONSTANT K = 7\nVIEW V\n")
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", "-view", filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != 12 || errout.Len() != 0 || !strings.Contains(out.String(), "of module M>"+tc.want) {
				t.Fatalf("exit %d, missing %q; stdout:\n%s\nstderr:\n%s", code, tc.want, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaViewTrace(t, jar, dir, "M", out.String(), code)
			}
		})
	}
}

func compareJavaViewTrace(t *testing.T, jar, dir, model, got string, code int) {
	t.Helper()
	cmd := exec.Command("java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-tool", "-deadlock", "-view", model+".tla")
	cmd.Dir = dir
	output, _ := javatmp.Apply(cmd).CombinedOutput()
	if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code {
		t.Fatalf("Java exit %v, want %d:\n%s", cmd.ProcessState, code, output)
	}
	compareRawToolTrace(t, string(output), got)
}
