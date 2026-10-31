package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// etest4 and etest8 are unchanged models from the Java TLC v1.7.4 test suite,
// exercised upstream by ETest4 and ETest8.
func TestJavaTLCUpstreamErrorBehavior(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		code int
		want []string
	}{
		{"etest4", nil, 75, []string{
			"@!@!@STARTMSG 1000:1 @!@!@\nA non-function (a model value) was applied as a function.\nline 15, col 12 to line 15, col 22 of module etest4\n",
			"0. Line 13, column 9 to line 16, column 51 in etest4\n1. Line 13, column 12 to line 13, column 16 in etest4\n" +
				"2. Line 15, column 12 to line 15, column 26 in etest4\n3. Line 15, column 12 to line 15, column 22 in etest4\n",
			"0 states generated, 0 distinct states found, 0 states left on queue.",
		}},
		{"etest8", []string{"-simulate", "-seed", "1"}, 255, []string{
			"The exception was a tlc2.tool.EvalException\n: @!@!@STARTMSG 2132:0 @!@!@\nThe first argument of Assert evaluated to FALSE",
			"@!@!@STARTMSG 2121:1 @!@!@\nThe behavior up to this point is:",
			"3: <Next line 15, col 12 to line 16, col 22 of module etest8>\nx = \"d\"\n",
			"The number of states generated: 4\nSimulation using seed 1 and aril 0",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := copyJavaTLCFixture(t, tc.name)
			args := append(append([]string{"-tool", "-deadlock"}, tc.args...), filepath.Join(dir, tc.name+".tla"))
			var out, errout bytes.Buffer
			code := run(context.Background(), args, &out, &errout)
			for _, want := range tc.want {
				if code != tc.code || errout.Len() != 0 || !strings.Contains(out.String(), want) {
					t.Fatalf("exit %d, missing %q; stdout:\n%s\nstderr:\n%s", code, want, out.String(), errout.String())
				}
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				javaArgs := append(append([]string{"-tool", "-deadlock"}, tc.args...), tc.name+".tla")
				java := runJavaToolMessages(t, jar, dir, javaArgs, code)
				if got := errorBehaviorMessages(readToolMessages(out.String())); !reflect.DeepEqual(got, java) {
					t.Fatalf("messages differ\nJava: %q\nGo:   %q", java, got)
				}
			}
		})
	}
}

func copyJavaTLCFixture(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	for _, ext := range []string{".tla", ".cfg"} {
		data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", name+ext))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+ext), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func runJavaToolMessages(t *testing.T, jar, dir string, args []string, code int) []string {
	t.Helper()
	cmd := exec.Command("java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar}, args...)...)
	cmd.Dir = dir
	output, _ := cmd.CombinedOutput()
	if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code {
		t.Fatalf("Java exit %v, want %d; output:\n%s", cmd.ProcessState, code, output)
	}
	return errorBehaviorMessages(readToolMessages(string(output)))
}

// errorBehaviorMessages omits banners, timestamps, and progress reports.
func errorBehaviorMessages(messages []toolMessage) []string {
	var result []string
	for _, message := range messages {
		if slices.Contains([]int{1000, 2103, 2121, 2132, 2199, 2210, 2217}, message.code) {
			result = append(result, fmt.Sprintf("%d:%d:%s", message.code, message.level, message.body))
		}
	}
	return result
}
