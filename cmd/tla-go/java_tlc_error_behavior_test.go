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
				"2. Line 15, column 12 to line 15, column 26 in etest4\n3. Line 15, column 12 to line 15, column 22 in etest4\n\n\n@!@!@ENDMSG 2103 @!@!@\n",
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

// Java TLC's 2103 text is CallStack.toString, which ends a list of frames
// with a blank line, followed by a newline.
func TestJavaTLCEvaluationStackLayout(t *testing.T) {
	for _, tc := range []struct{ name, init, want string }{
		{"frames", "x = SelectSeq([i \\in 1..20 |-> i], LAMBDA e : TRUE)", "1. Line 4, column 13 to line 4, column 59 in M\n\n\n@!@!@ENDMSG 2103 @!@!@\n"},
		{"empty", "x \\in {i \\in 1..20 : TRUE}", "    The error call stack is empty.\n\n@!@!@ENDMSG 2103 @!@!@\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeMaxSetSizeFixture(t, dir, "VARIABLE x\nInit == "+tc.init+"\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n")
			args := []string{"-tool", "-maxSetSize", "10"}
			var out, errout bytes.Buffer
			code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
			got := rawToolMessage(out.String(), 2103)
			if code != 75 || !strings.HasSuffix(got, tc.want) {
				t.Fatalf("exit %d, 2103 %q; stdout:\n%s", code, got, out.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				cmd := exec.Command("java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-jar", jar}, append(args, "M.tla")...)...)
				cmd.Dir = dir
				output, _ := cmd.Output()
				if java := rawToolMessage(string(output), 2103); java != got {
					t.Fatalf("2103 differs\nJava: %q\nGo:   %q", java, got)
				}
			}
		})
	}
}

// Java TLC generates states by continuation passing, so earlier conjuncts and
// SANY's nested inline conjunctions stay on the call stack, while top-level
// disjunctions and existential quantifiers are split into actions.
func TestJavaTLCEvaluationStackFrames(t *testing.T) {
	for _, tc := range []struct{ name, init, next, want string }{
		{"successor guard", "x = 0 /\\ y = 0", "x < 2 /\\ x' = x[1] /\\ y' = y", "0. Line 5, column 9 to line 5, column 36 in M\n1. Line 5, column 9 to line 5, column 26 in M\n2. Line 5, column 9 to line 5, column 13 in M\n3. Line 5, column 18 to line 5, column 26 in M\n4. Line 5, column 23 to line 5, column 26 in M\n"},
		{"successor assignment", "x = 0 /\\ y = 0", "x' = 1 /\\ y' = x[1]", "1. Line 5, column 9 to line 5, column 14 in M\n2. Line 5, column 19 to line 5, column 27 in M\n"},
		{"inline chain", "x = 0 /\\ y = 0", "x < 5 /\\ y < 5 /\\ x' = 1 /\\ y' = x[1]", "2. Line 5, column 9 to line 5, column 22 in M\n3. Line 5, column 9 to line 5, column 13 in M\n"},
		{"initial guard", "/\\ x = 0 /\\ x < 5 /\\ y = x[1]", "UNCHANGED <<x, y>>", "1. Line 4, column 12 to line 4, column 37 in M\n2. Line 4, column 12 to line 4, column 25 in M\n"},
		{"split existential", "x = 0 /\\ y = 0", "\\E i \\in {1} : x' = i /\\ y' = x[1]", "0. Line 5, column 24 to line 5, column 42 in M\n1. Line 5, column 24 to line 5, column 29 in M\n"},
		{"split disjunction", "x = 0 /\\ y = 0", "(x' = 1 /\\ y' = y) \\/ (x' = x /\\ y' = x[1])", "0. Line 5, column 32 to line 5, column 50 in M\n"},
		{"nested existential", "x = 0 /\\ y = 0", "x < 3 /\\ \\E i \\in {1} : x' = i /\\ y' = x[1]", "2. Line 5, column 18 to line 5, column 51 in M\n3. Line 5, column 33 to line 5, column 51 in M\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeMaxSetSizeFixture(t, dir, "VARIABLES x, y\nInit == "+tc.init+"\nNext == "+tc.next, "INIT Init\nNEXT Next\n")
			args := []string{"-tool", "-deadlock"}
			var out, errout bytes.Buffer
			code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
			got := rawToolMessage(out.String(), 2103)
			if !strings.Contains(got, tc.want) {
				t.Fatalf("exit %d, 2103 missing %q:\n%s", code, tc.want, out.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				cmd := exec.Command("java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-jar", jar}, append(args, "M.tla")...)...)
				cmd.Dir = dir
				output, _ := cmd.Output()
				if java := rawToolMessage(string(output), 2103); java != got {
					t.Fatalf("2103 differs\nJava: %q\nGo:   %q", java, got)
				}
			}
		})
	}
}

// rawToolMessage returns the bytes of the first tool message with code.
func rawToolMessage(output string, code int) string {
	start := strings.Index(output, fmt.Sprintf("@!@!@STARTMSG %d:", code))
	if start < 0 {
		return ""
	}
	end := fmt.Sprintf("@!@!@ENDMSG %d @!@!@\n", code)
	length := strings.Index(output[start:], end)
	if length < 0 {
		return ""
	}
	return output[start : start+length+len(end)]
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
		if slices.Contains([]int{1000, 2102, 2103, 2105, 2111, 2121, 2132, 2199, 2210, 2217}, message.code) {
			result = append(result, fmt.Sprintf("%d:%d:%s", message.code, message.level, message.body))
		}
	}
	return result
}
