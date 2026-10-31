package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Java TLC's -generateSpecTE writes SpecTE.tla and SpecTE.cfg after an error
// trace. The java-SpecTE fixtures come from Java TLC 1.7.4; see
// testdata/spec-te/README.md.
func TestJavaTLCGenerateSpecTE(t *testing.T) {
	for _, tc := range []struct {
		name  string
		extra []string
		code  int
		java  bool
	}{
		{"invariant", nil, 12, true},
		{"liveness", nil, 13, false},
		{"stuttering", nil, 13, true},
		{"deadlock", nil, 11, true},
		{"action", nil, 13, true},
		{"assertion", nil, 14, true},
		{"constants", nil, 12, true},
		{"monolith", nil, 12, true},
		{"nomonolith", []string{"nomonolith"}, 12, true},
		{"instance", nil, 12, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture, err := filepath.Abs(filepath.Join("testdata", "spec-te", tc.name))
			if err != nil {
				t.Fatal(err)
			}
			dir := copySpecTEFixture(t, fixture)
			args := append(append([]string{"-generateSpecTE"}, tc.extra...), "-workers", "1", "M")
			output, code, stderr := runGoInDir(t, dir, args)
			if code != tc.code || stderr != "" || !strings.Contains(output, "@!@!@STARTMSG 1000:0 @!@!@\nThe file "+filepath.Join(dir, "SpecTE.cfg")+" has been created.\n") {
				t.Fatalf("exit %d; stdout:\n%s\nstderr:\n%s", code, output, stderr)
			}
			if tla := readSpecTE(t, dir, "SpecTE.tla"); !strings.HasPrefix(tla, output) {
				t.Fatalf("SpecTE.tla does not start with the tool output:\n%s", tla)
			}
			for _, file := range []string{"SpecTE.tla", "SpecTE.cfg"} {
				want := readSpecTE(t, fixture, "java-"+file)
				if got := normalizeSpecTE(readSpecTE(t, dir, file)); got != want {
					t.Errorf("%s differs from Java TLC\ngot:\n%s\nwant:\n%s", file, got, want)
				}
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" && tc.java {
				compareJavaSpecTE(t, jar, fixture, append(append([]string{"-generateSpecTE"}, tc.extra...), "-workers", "1", "M"))
			}
		})
	}
}

// Java TLC writes no SpecTE files unless model checking prints a trace. When
// the first error has no trace, it reports that it could not parse its output
// (and then stops responding, which tla-go does not reproduce).
func TestGenerateSpecTEWithoutTrace(t *testing.T) {
	for _, tc := range []struct {
		name, cfg string
		extra     []string
		code      int
		failure   bool
	}{
		{"initial state", "INIT Init\nNEXT Next\nINVARIANT Zero\n", nil, 12, true},
		{"success", "INIT Init\nNEXT Next\nINVARIANT Small\n", nil, 0, false},
		{"simulation", "INIT Init\nNEXT Next\nINVARIANT One\n", []string{"-simulate", "num=1", "-depth", "5", "-seed", "1"}, 12, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == (x < 2 /\\ x' = x + 1) \\/ (x = 2 /\\ UNCHANGED x)\nZero == x > 0\nOne == x < 1\nSmall == x < 3", tc.cfg)
			output, code, stderr := runGoInDir(t, dir, append(append([]string{"-generateSpecTE"}, tc.extra...), "M.tla"))
			failure := strings.HasSuffix(output, "@!@!@STARTMSG 1000:0 @!@!@\n"+specTEParseFailure+"\n@!@!@ENDMSG 1000 @!@!@\n")
			if _, err := os.Stat(filepath.Join(dir, "SpecTE.tla")); code != tc.code || stderr != "" || failure != tc.failure || !os.IsNotExist(err) {
				t.Fatalf("exit %d, SpecTE %v; stdout:\n%s\nstderr:\n%s", code, err, output, stderr)
			}
		})
	}
}

// With -continue, the trace of the first reported violation is used.
func TestGenerateSpecTEContinue(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == x < 3 /\\ x' = x + 1\nInv == x < 1", "INIT Init\nNEXT Next\nINVARIANT Inv\nCHECK_DEADLOCK FALSE\n")
	output, code, stderr := runGoInDir(t, dir, []string{"-generateSpecTE", "-continue", "M.tla"})
	tla := readSpecTE(t, dir, "SpecTE.tla")
	if code != 0 || stderr != "" || !strings.Contains(tla, "<<\n([x |-> 0]),\n([x |-> 1])\n>>") {
		t.Fatalf("exit %d; stdout:\n%s\nSpecTE.tla:\n%s", code, output, tla)
	}
}

func copySpecTEFixture(t *testing.T, source string) string {
	t.Helper()
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "java-") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(source, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, entry.Name()), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func readSpecTE(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

var (
	specTEIdentifier = regexp.MustCompile(`def_ov_\d+`)
	specTEDate       = regexp.MustCompile(`\\\* Generated on .*`)
)

// normalizeSpecTE removes the tool output and replaces the time-based
// identifier and date, as in the fixtures.
func normalizeSpecTE(text string) string {
	const finished = "@!@!@ENDMSG 2186 @!@!@\n"
	if _, after, ok := strings.Cut(text, finished); ok {
		text = after
	}
	text = specTEIdentifier.ReplaceAllString(text, "def_ov_ID")
	return specTEDate.ReplaceAllString(text, `\* Generated on DATE`)
}

// compareJavaSpecTE reruns Java TLC on a fixture. Its SpecTE generation races
// with its output parser and can omit states or stop responding, so a few
// attempts are allowed.
func compareJavaSpecTE(t *testing.T, jar, fixture string, args []string) {
	t.Helper()
	want := readSpecTE(t, fixture, "java-SpecTE.tla")
	var got string
	for range 3 {
		dir := copySpecTEFixture(t, fixture)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		cmd := exec.CommandContext(ctx, "java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-cp", jar, "tlc2.TLC"}, args...)...)
		cmd.Dir = dir
		_ = cmd.Run()
		cancel()
		if data, err := os.ReadFile(filepath.Join(dir, "SpecTE.tla")); err == nil {
			if got = normalizeSpecTE(string(data)); got == want {
				return
			}
		}
	}
	t.Fatalf("Java TLC SpecTE.tla differs from the fixture:\n%s", got)
}
