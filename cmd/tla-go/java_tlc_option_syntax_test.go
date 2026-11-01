package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Java TLC 1.7.4 (5a47802): argument errors stop at the first failing
// option; -tool changes framing only when it was already encountered.
func TestJavaTLCOptionSyntaxErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
		body string
		tool bool
	}{
		{"equals", []string{"-coverage=0", "M.tla"}, "Error: unrecognized option: -coverage=0", false},
		{"double dash", []string{"--coverage", "0", "M.tla"}, "Error: unrecognized option: --coverage", false},
		{"unknown", []string{"-unknown", "M.tla"}, "Error: unrecognized option: -unknown", false},
		{"tool unknown", []string{"-tool", "-unknown", "M.tla"}, "Error: unrecognized option: -unknown", true},
		{"late tool", []string{"-unknown", "-tool", "M.tla"}, "Error: unrecognized option: -unknown", false},
		{"unknown before translate", []string{"-tool", "-unknown", "-translate", "M.tla"}, "Error: unrecognized option: -unknown", true},
		{"translate as value after unknown", []string{"-tool", "-unknown", "-config", "-translate", "M.tla"}, "Error: unrecognized option: -unknown", true},
		{"translate as path value", []string{"-config", "-translate", "-unknown", "M.tla"}, "Error: unrecognized option: -unknown", false},
		{"translate as invalid value", []string{"-tool", "-coverage", "-translate", "M.tla"}, "Error: An integer for coverage report interval required. But encountered -translate", true},
		{"missing coverage", []string{"-coverage"}, "Error: coverage report interval required.", false},
		{"tool missing coverage", []string{"-tool", "-coverage"}, "Error: coverage report interval required.", true},
		{"option as value", []string{"-coverage", "-tool", "M.tla"}, "Error: An integer for coverage report interval required. But encountered -tool", false},
		{"space", []string{"-coverage", " 0", "M.tla"}, "Error: An integer for coverage report interval required. But encountered  0", false},
		{"overflow", []string{"-coverage", "2147483648", "M.tla"}, "Error: An integer for coverage report interval required. But encountered 2147483648", false},
		{"negative overflow", []string{"-coverage", "-2147483649", "M.tla"}, "Error: An integer for coverage report interval required. But encountered -2147483649", false},
		{"wrapped negative interval", []string{"-coverage", "35792", "M.tla"}, "Error: expect a nonnegative integer for -coverage option.", false},
		{"negative", []string{"-tool", "-coverage", "-1", "M.tla"}, "Error: expect a nonnegative integer for -coverage option.", true},
		{"missing workers", []string{"-workers"}, "Error: expect an integer or 'auto' for -workers option.", false},
		{"tool invalid workers", []string{"-tool", "-workers", "0", "M.tla"}, "Error: at least one worker required.", true},
		{"tool missing config", []string{"-tool", "-config"}, "Error: expect a file name for -config option.", true},
	}
	jar := os.Getenv("TLC_JAR")
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := optionSyntaxModel(t)
			out, code, stderr := runGoInDir(t, dir, tc.args)
			want := optionErrorBody(tc.body, tc.tool)
			if code != 1 || stderr != "" || stripOptionVersion(out, tc.tool) != want {
				t.Fatalf("Go exit %d stdout %q stderr %q, want %q", code, out, stderr, want)
			}
			if jar != "" {
				jout, jerr, jcode := optionSyntaxJava(t, jar, dir, tc.args)
				if jcode != code || jerr != stderr || stripOptionVersion(jout, tc.tool) != want {
					t.Fatalf("Java exit %d stdout %q stderr %q, want %q", jcode, jout, jerr, want)
				}
			}
		})
	}
}

// tla-go deliberately adds its translator usage on stderr for these two
// errors. The TLC error on stdout and exit status still match Java.
func TestJavaTLCTranslateHint(t *testing.T) {
	for _, tc := range []struct {
		args []string
		body string
		tool bool
	}{
		{[]string{"-tool", "-translate", "M.tla"}, "Error: unrecognized option: -translate", true},
		{[]string{"-unixEOL", "-translate", "M.tla"}, "Error: unrecognized option: -unixEOL", false},
		{[]string{"-unixEOL", "M.tla"}, "Error: unrecognized option: -unixEOL", false},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			dir := optionSyntaxModel(t)
			out, code, stderr := runGoInDir(t, dir, tc.args)
			want := optionErrorBody(tc.body, tc.tool)
			var usageText bytes.Buffer
			usage(&usageText)
			if code != 1 || stripOptionVersion(out, tc.tool) != want || stderr != usageText.String() {
				t.Fatalf("Go exit %d stdout %q stderr %q", code, out, stderr)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				jout, jerr, jcode := optionSyntaxJava(t, jar, dir, tc.args)
				if jcode != code || jerr != "" || stripOptionVersion(jout, tc.tool) != want {
					t.Fatalf("Java exit %d stdout %q stderr %q", jcode, jout, jerr)
				}
			}
		})
	}
}

func optionErrorBody(body string, tool bool) string {
	body += "\nUsage: java tlc2.TLC [-option] inputfile\n"
	if tool {
		return "@!@!@STARTMSG 1102:1 @!@!@\n" + body + "@!@!@ENDMSG 1102 @!@!@\n"
	}
	return "Error: " + body
}

// Version messages are implementation-specific. Java emits one in normal
// mode too; Go currently emits one only in tool mode.
func stripOptionVersion(output string, tool bool) string {
	if tool {
		if _, after, ok := strings.Cut(output, "@!@!@ENDMSG 2262 @!@!@\n"); ok {
			return after
		}
		return output
	}
	if strings.HasPrefix(output, "TLC2 Version ") {
		if _, after, ok := strings.Cut(output, "\n"); ok {
			return after
		}
	}
	return output
}

func optionSyntaxModel(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, contents := range map[string]string{"M.tla": "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x \\in 0..1\nA == x' = x\nB == x' = x\nNext == A \\/ B\n====\n", "M.cfg": "INIT Init\nNEXT Next\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func optionSyntaxJava(t *testing.T, jar, dir string, args []string) (string, string, int) {
	t.Helper()
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", append([]string{"-Xmx256m", "-XX:+UseParallelGC", "-cp", jar, "tlc2.TLC"}, args...)...)
	cmd.Dir = dir
	var out, errout bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errout
	err = cmd.Run()
	if err == nil {
		return out.String(), errout.String(), 0
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && ctx.Err() == nil {
		return out.String(), errout.String(), exit.ExitCode()
	}
	t.Fatalf("Java: %v stdout %s stderr %s", err, out.String(), errout.String())
	return "", "", 0
}

func TestJavaTLCCoverageIntegerSyntax(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	for _, input := range []string{"0", "+0", "-0", "000", "-000", "1073741824", "-2147483648", "-2147483647"} {
		t.Run(input, func(t *testing.T) {
			dir := optionSyntaxModel(t)
			args := []string{"-workers", "1", "-coverage", input, "M.tla"}
			out, code, stderr := runGoInDir(t, dir, args)
			if code != 0 || stderr != "" || !strings.Contains(out, "The coverage statistics at ") {
				t.Fatalf("Go exit %d stdout %s stderr %s", code, out, stderr)
			}
			if jar != "" {
				jout, jerr, jcode := optionSyntaxJava(t, jar, dir, args)
				if jcode != 0 || jerr != "" || coverageReport(t, out) != coverageReport(t, jout) {
					t.Fatalf("Java exit %d stdout %s stderr %s", jcode, jout, jerr)
				}
			}
		})
	}
}

// Java Integer.parseInt accepts BMP Unicode decimal digits; strconv.ParseInt
// does not. Keep this observed difference explicit until all integer options
// can share a safe Java-compatible parser.
func TestJavaTLCCoverageUnicodeDigitDifference(t *testing.T) {
	dir := optionSyntaxModel(t)
	args := []string{"-workers", "1", "-coverage", "٠", "M.tla"}
	out, code, stderr := runGoInDir(t, dir, args)
	if code != 1 || stderr != "" || !strings.Contains(out, "But encountered ٠") {
		t.Fatalf("Go exit %d stdout %s stderr %s", code, out, stderr)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		jout, jerr, jcode := optionSyntaxJava(t, jar, dir, args)
		if jcode != 0 || jerr != "" || !strings.Contains(jout, "The coverage statistics at ") {
			t.Fatalf("Java exit %d stdout %s stderr %s", jcode, jout, jerr)
		}
	}
}
