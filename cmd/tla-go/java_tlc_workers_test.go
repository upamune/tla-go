package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

const workerModel = "VARIABLES x, y\nInit == x = 0 /\\ y = 0\nNext == x' \\in 0..3 /\\ y' = (y + x) % 4\nInv == x \\in 0..3"

// Java TLC accepts any positive worker count and checkpoint interval. tla-go
// accepts them; unsupported checkpoint models continue without checkpointing.
func TestJavaTLCWorkerAndCheckpointOptions(t *testing.T) {
	for _, tc := range []struct {
		args    []string
		workers int
	}{
		{[]string{"-workers", "3"}, 3},
		{[]string{"-workers", "auto"}, runtime.NumCPU()},
		{[]string{"-workers", "1", "-checkpoint", "0"}, 1},
		{[]string{"-checkpoint", "5", "-workers", "+2"}, 2},
		{[]string{"-checkpoint", "35791"}, 1},
		{[]string{"-checkpoint", "71583"}, 1},
		{[]string{"-checkpoint", "268435456"}, 1},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, workerModel, "INIT Init\nNEXT Next\nINVARIANT Inv\n")
			var out, errout bytes.Buffer
			code := run(context.Background(), append(append([]string{"-tool"}, tc.args...), filepath.Join(dir, "M.tla")), &out, &errout)
			mode := fmt.Sprintf("with %d worker%s on %d cores.", tc.workers, plural(tc.workers), runtime.NumCPU())
			if code != 0 || !strings.Contains(out.String(), mode) || !strings.Contains(out.String(), "65 states generated, 16 distinct states found, 0 states left on queue.") {
				t.Fatalf("exit %d; stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runJavaToolMessages(t, jar, dir, append(append([]string{"-tool"}, tc.args...), "M.tla"), code)
				if got := errorBehaviorMessages(readToolMessages(out.String())); !reflect.DeepEqual(got, java) {
					t.Fatalf("messages differ\nJava: %q\nGo:   %q", java, got)
				}
			}
		})
	}
}

// printedOptionError is TLC.printErrorMsg's output for a parameter error
// without -tool, after the version banner that tla-go does not print.
func printedOptionError(message string) string {
	return "Error: " + message + "\nUsage: java tlc2.TLC [-option] inputfile\n"
}

func TestJavaTLCInvalidWorkerAndCheckpointOptions(t *testing.T) {
	for _, tc := range []struct {
		args    []string
		message string
	}{
		{[]string{"-workers", "0", "M.tla"}, "Error: at least one worker required."},
		{[]string{"-workers", "-2", "M.tla"}, "Error: at least one worker required."},
		{[]string{"-workers", "many", "M.tla"}, "Error: worker number or 'auto' required. But encountered many"},
		{[]string{"M.tla", "-workers"}, "Error: expect an integer or 'auto' for -workers option."},
		{[]string{"-checkpoint", "-1", "M.tla"}, "Error: expect a nonnegative integer for -checkpoint option."},
		{[]string{"-checkpoint", "35792", "M.tla"}, "Error: expect a nonnegative integer for -checkpoint option."},
		{[]string{"-checkpoint", "40000", "M.tla"}, "Error: expect a nonnegative integer for -checkpoint option."},
		{[]string{"-checkpoint", "soon", "M.tla"}, "Error: An integer for checkpoint interval is required. But encountered soon"},
		{[]string{"M.tla", "-checkpoint"}, "Error: checkpoint interval required."},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, workerModel, "INIT Init\nNEXT Next\n")
			var out, errout bytes.Buffer
			if code := run(context.Background(), tc.args, &out, &errout); code != 1 || errout.Len() != 0 || out.String() != printedOptionError(tc.message) {
				t.Fatalf("exit %d; stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				cmd := exec.Command("java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-jar", jar}, tc.args...)...)
				cmd.Dir = dir
				output, _ := cmd.CombinedOutput()
				if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 1 || !strings.Contains(string(output), tc.message) {
					t.Fatalf("Java exit %v; output:\n%s", cmd.ProcessState, output)
				}
			}
		})
	}
}

// With -tool before an invalid option, Java TLC reports message 1102, followed
// by its usage line, on standard output. An unwritable dump file is a
// parameter error as well, and so is a -userFile file that cannot be created.
func TestJavaTLCToolOptionErrors(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing", "states")
	missingUserFile := filepath.Join(t.TempDir(), "missing", "out.txt")
	for _, tc := range []struct {
		args          []string
		stdout, error string
	}{
		{[]string{"-tool", "-workers", "0", "M.tla"}, "", "Error: at least one worker required."},
		{[]string{"-tool", "M.tla", "-maxSetSize"}, "", "Error: maxSetSize required."},
		{[]string{"-tool", "-dump", missing, "M.tla"}, "Error: Unable to write to file " + missing + ".dump\n", "Error: Given file name " + missing + ".dump for dumping states invalid."},
		{[]string{"-tool", "-dump", "dot", missing, "M.tla"}, "Error: Unable to write to file " + missing + ".dot\n", "Error: Given file name " + missing + ".dot for dumping states invalid."},
		{[]string{"-tool", "-userFile", missingUserFile, "M.tla"}, "", "Error: Failed to create user output log file."},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, workerModel, "INIT Init\nNEXT Next\n")
			var out, errout bytes.Buffer
			args := slices.Clone(tc.args)
			args[slices.Index(args, "M.tla")] = filepath.Join(dir, "M.tla")
			code := run(context.Background(), args, &out, &errout)
			messages := readToolMessages(out.String())
			want := tc.error + "\nUsage: java tlc2.TLC [-option] inputfile"
			if code != 1 || errout.Len() != 0 || !strings.HasPrefix(out.String(), tc.stdout) || len(messages) != 2 || messages[1].code != 1102 || messages[1].body != want {
				t.Fatalf("exit %d; stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				cmd := exec.Command("java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-jar", jar}, tc.args...)...)
				cmd.Dir = dir
				output, _ := cmd.Output()
				java := readToolMessages(string(output))
				if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 1 || !strings.HasPrefix(string(output), tc.stdout) || len(java) != 2 || java[1] != messages[1] {
					t.Fatalf("Java exit %v; output:\n%s", cmd.ProcessState, output)
				}
			}
		})
	}
}

// TLC.java at 5a47802 evaluates the multiplication as signed Java int
// before assigning it to chkptDuration. In particular, positive overflow can
// wrap back to a small positive interval.
func TestCheckpointIntervalJavaIntArithmetic(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  time.Duration
	}{
		{"0", 0}, {"35791", 2147460000 * time.Millisecond}, {"71583", 12704 * time.Millisecond}, {"268435456", 0},
	} {
		var opts options
		if _, err := parseCheckpointOption(&opts, []string{"-checkpoint", tc.input}); err != nil || opts.checkpointInterval != tc.want {
			t.Fatalf("%s: got %v, err %v; want %v", tc.input, opts.checkpointInterval, err, tc.want)
		}
	}
}
