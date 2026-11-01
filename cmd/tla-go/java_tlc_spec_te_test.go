package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
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
		{"long", nil, 12, true},
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
				compareJavaSpecTE(t, jar, fixture, append(append([]string{"-generateSpecTE"}, tc.extra...), "-workers", "1", "M"), tc.code)
			}
		})
	}
}

// Java TLC writes no SpecTE files unless breadth-first model checking prints
// a trace. When the first error has no trace, and after an error in
// simulation or depth-first mode, it reports that it could not parse its
// output (and then stops responding, which tla-go does not reproduce).
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
		{"depth-first", "INIT Init\nNEXT Next\nINVARIANT One\n", []string{"-dfid", "5"}, 12, true},
		{"depth-first success", "INIT Init\nNEXT Next\nINVARIANT Small\n", []string{"-dfid", "5"}, 0, false},
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
	dir := t.TempDir()
	copySpecTEFixtureTo(t, source, dir)
	return dir
}

func copySpecTEFixtureTo(t *testing.T, source, dir string) {
	t.Helper()
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatal(err)
	}
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
func compareJavaSpecTE(t *testing.T, jar, fixture string, args []string, expectedExit int) {
	t.Helper()
	want := readSpecTE(t, fixture, "java-SpecTE.tla")
	partial, err := os.ReadFile(filepath.Join(fixture, "java-partial-SpecTE.tla"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var pinned bool
	if len(partial) != 0 {
		pinned, err = isPinnedSpecTEJar(jar)
		if err != nil {
			t.Fatal(err)
		}
	}
	var got string
	var attemptErr error
	partialSeen := false
	for range 3 {
		// Java's trace-output parser can lose states with long test paths.
		// Keep its temporary paths short without changing the model or output.
		dir, err := os.MkdirTemp("", "specte-")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(dir) })
		copySpecTEFixtureTo(t, fixture, dir)
		tmp, err := os.MkdirTemp("", "tlc-")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(tmp) })
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		cmd := exec.CommandContext(ctx, "java", append([]string{"-Djava.io.tmpdir=" + tmp, "-Xmx256m", "-XX:+UseParallelGC", "-cp", jar, "tlc2.TLC"}, args...)...)
		cmd.Dir = dir
		runErr := cmd.Run()
		contextErr := ctx.Err()
		cancel()
		if err := checkJavaSpecTEExit(runErr, contextErr, expectedExit); err != nil {
			attemptErr = err
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, "SpecTE.tla"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatalf("read Java TLC SpecTE.tla: %v", err)
		}
		got = normalizeSpecTE(string(data))
		if got == want {
			return
		}
		if matchesJavaSpecTEPartial(got, partial, pinned) {
			partialSeen = true
		} else {
			t.Fatalf("Java TLC SpecTE.tla differs from the known complete and partial fixtures:\n%s", got)
		}
	}
	if partialSeen {
		t.Log("Java TLC generated only the known partial trace in completed attempts; the live comparison is partial, not exact compatibility. The complete fixture remains the tla-go comparison.")
		return
	}
	if attemptErr != nil {
		t.Fatalf("Java TLC SpecTE produced no acceptable output; last failed attempt: %v", attemptErr)
	}
	t.Fatalf("Java TLC SpecTE.tla differs from the fixture:\n%s", got)
}

func checkJavaSpecTEExit(runErr, contextErr error, expected int) error {
	if contextErr != nil {
		return contextErr
	}
	if runErr == nil {
		if expected == 0 {
			return nil
		}
		return fmt.Errorf("exit 0, want %d", expected)
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) && exitErr.ExitCode() == expected {
		return nil
	}
	return fmt.Errorf("want exit %d: %w", expected, runErr)
}

func isPinnedSpecTEJar(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)) == "936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88", nil
}

func matchesJavaSpecTEPartial(got string, partial []byte, pinned bool) bool {
	return pinned && len(partial) != 0 && got == string(partial)
}

func TestJavaSpecTEKnownPartialFixture(t *testing.T) {
	dir := filepath.Join("testdata", "spec-te", "instance")
	partial := []byte(readSpecTE(t, dir, "java-partial-SpecTE.tla"))
	full := readSpecTE(t, dir, "java-SpecTE.tla")
	for _, tc := range []struct {
		name, text string
		want       bool
	}{
		{"known partial", string(partial), true},
		{"complete", full, false},
		{"wrong state", strings.Replace(string(partial), "([x |-> 1])", "([x |-> 999])", 1), false},
		{"missing action", strings.Replace(string(partial), "Next_sa_0 ==", "", 1), false},
		{"changed extends", strings.Replace(string(partial), "Toolbox, TLC", "TLC, Toolbox", 1), false},
		{"extra data", string(partial) + "unexpected\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name != "known partial" && tc.text == string(partial) {
				t.Fatal("test mutation did not change the fixture")
			}
			if got := matchesJavaSpecTEPartial(tc.text, partial, true); got != tc.want {
				t.Fatalf("matches = %t, want %t", got, tc.want)
			}
		})
	}
	if matchesJavaSpecTEPartial("", nil, true) {
		t.Fatal("missing partial fixture matched empty output")
	}
	if matchesJavaSpecTEPartial(string(partial), partial, false) {
		t.Fatal("unverified jar matched the 1.7.4 partial fixture")
	}
}

func TestJavaSpecTEExpectedExit(t *testing.T) {
	const marker = "TLA_GO_SPEC_TE_EXIT_FIXTURE"
	if os.Getenv(marker) == "exit" {
		os.Exit(12)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestJavaSpecTEExpectedExit$")
	cmd.Env = append(os.Environ(), marker+"=exit")
	exitErr := cmd.Run()
	if err := checkJavaSpecTEExit(exitErr, nil, 12); err != nil {
		t.Fatalf("expected exit rejected: %v", err)
	}
	if err := checkJavaSpecTEExit(exitErr, nil, 13); err == nil {
		t.Fatal("unexpected exit accepted")
	}
	if err := checkJavaSpecTEExit(exitErr, context.DeadlineExceeded, 12); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timed-out process accepted: %v", err)
	}
	if err := checkJavaSpecTEExit(nil, nil, 12); err == nil {
		t.Fatal("zero exit accepted for an error case")
	}
	if err := checkJavaSpecTEExit(errors.New("start failed"), nil, 12); err == nil {
		t.Fatal("failed process start accepted")
	}
	if err := checkJavaSpecTEExit(nil, nil, 0); err != nil {
		t.Fatalf("successful zero exit rejected: %v", err)
	}
}

func TestJavaSpecTECanceledArtifact(t *testing.T) {
	const marker = "TLA_GO_SPEC_TE_ARTIFACT_FIXTURE"
	if path := os.Getenv(marker); path != "" {
		partial, err := os.ReadFile(filepath.Join("testdata", "spec-te", "instance", "java-partial-SpecTE.tla"))
		if err != nil {
			os.Exit(1)
		}
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, partial, 0o600); err != nil {
			os.Exit(1)
		}
		if err := os.Rename(tmp, path); err != nil {
			os.Exit(1)
		}
		time.Sleep(30 * time.Second)
		os.Exit(12)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "SpecTE.tla")
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestJavaSpecTECanceledArtifact$")
	cmd.Env = append(os.Environ(), marker+"="+path)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(path); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			_ = cmd.Wait()
			t.Fatal("artifact fixture did not start")
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()
	runErr := cmd.Wait()
	if err := checkJavaSpecTEExit(runErr, ctx.Err(), 12); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled process with an artifact accepted: %v", err)
	}
	partial := []byte(readSpecTE(t, filepath.Join("testdata", "spec-te", "instance"), "java-partial-SpecTE.tla"))
	if data, err := os.ReadFile(path); err != nil || !matchesJavaSpecTEPartial(string(data), partial, true) {
		t.Fatalf("artifact missing: %q, %v", data, err)
	}
}

func TestJavaSpecTEJarPin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "other.jar")
	if err := os.WriteFile(path, []byte("not the pinned jar"), 0o600); err != nil {
		t.Fatal(err)
	}
	if pinned, err := isPinnedSpecTEJar(path); err != nil || pinned {
		t.Fatalf("unrelated jar: pinned=%t, err=%v", pinned, err)
	}
	if _, err := isPinnedSpecTEJar(filepath.Join(t.TempDir(), "missing.jar")); err == nil {
		t.Fatal("missing jar was accepted")
	}
}
