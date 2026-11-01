package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/upamune/tla-go/internal/javatmp"
)

func TestJavaTLCMultipleConfigs(t *testing.T) {
	for _, tc := range []struct {
		dir, model, config, counts string
		code                       int
	}{
		{"bidirectional", "BidirectionalTransitions", "BidirectionalTransitions1", "13/3/0", 0},
		{"bidirectional", "BidirectionalTransitions", "BidirectionalTransitions1Bx", "13/3/0", 13},
		{"bidirectional", "BidirectionalTransitions", "BidirectionalTransitions1By", "13/3/0", 13},
		{"bidirectional", "BidirectionalTransitions", "BidirectionalTransitions2", "9/4/0", 0},
		{"bidirectional", "BidirectionalTransitions", "BidirectionalTransitions2Cx", "9/4/0", 13},
		{"bidirectional", "BidirectionalTransitions", "BidirectionalTransitions2Cy", "9/4/0", 13},
		{"eval-order", "InitEvalOrder", "InitEvalOrder1", "2/1/0", 0},
		{"eval-order", "InitEvalOrder", "InitEvalOrder2", "2/1/0", 0},
		{"eval-order", "InitEvalOrder", "InitEvalOrder3", "2/1/0", 0},
		{"eval-order", "InitEvalOrder", "InitEvalOrder4", "2/1/0", 0},
		{"multi-trace", "BasicMultiTrace", "MC", "61/61/9", 11},
		{"multi-trace", "BasicMultiTrace", "MCActionProp", "12/12/9", 13},
		{"multi-trace", "BasicMultiTrace", "MCActionPropBadEval", "12/12/9", 75},
		{"multi-trace", "BasicMultiTrace", "MCBadInvInitState", "1/1/1", 75},
		{"multi-trace", "BasicMultiTrace", "MCBadInvNonInitState", "2/3/0", 75},
		{"multi-trace", "BasicMultiTrace", "MCInv", "22/22/9", 12},
		{"multi-trace", "BasicMultiTrace", "MCInvInitState", "", 12},
		{"multi-trace", "BasicMultiTrace", "MCLivenessProp", "61/61/9", 11},
		{"multi-trace", "BasicMultiTrace", "MCUnderspecInit", "", 255},
		{"multi-trace", "BasicMultiTrace", "MCUnderspecNext", "2/1/0", 75},
		{"multi-trace", "BasicMultiTrace", "MCWithActionConstraint", "51/41/0", 0},
		{"multi-trace", "BasicMultiTrace", "MCWithConstraint", "51/41/0", 0},
	} {
		t.Run(tc.config, func(t *testing.T) {
			dir := copyMultipleConfigFixtures(t, tc.dir)
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final", "-config", filepath.Join(dir, tc.config+".cfg"), filepath.Join(dir, tc.model+".tla")}
			var out, errout bytes.Buffer
			code := run(context.Background(), args, &out, &errout)
			checkMultipleConfigResult(t, code, out.String(), errout.String(), tc.code, tc.counts)
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runMultipleConfigJava(t, jar, dir, tc.model, tc.config, tc.code, tc.counts)
				if tc.dir == "bidirectional" && tc.code == 13 {
					compareRawToolTrace(t, java, out.String())
				}
				if tc.config == "MCBadInvNonInitState" || tc.config == "MCActionPropBadEval" {
					compareRawToolTrace(t, java, out.String())
					compareReplayStacks(t, java, out.String())
				}
			}
		})
	}
}

func checkMultipleConfigResult(t *testing.T, code int, output, errors string, wantCode int, counts string) {
	t.Helper()
	if code != wantCode {
		t.Fatalf("exit %d, want %d:\n%s\n%s", code, wantCode, output, errors)
	}
	if counts != "" {
		parts := strings.Split(counts, "/")
		want := parts[0] + " states generated, " + parts[1] + " distinct states found, " + parts[2] + " states left on queue"
		if !strings.Contains(output, want) {
			t.Fatalf("missing %q:\n%s\n%s", want, output, errors)
		}
	}
}

func copyMultipleConfigFixtures(t *testing.T, name string) string {
	t.Helper()
	source := filepath.Join("testdata", "java-tlc", "multiple-config", name)
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, entry := range entries {
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

func runMultipleConfigJava(t *testing.T, jar, dir, model, config string, wantCode int, counts string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "-lncheck", "final", "-config", config+".cfg", model+".tla")
	cmd.Dir = dir
	output, err := javatmp.Apply(cmd).CombinedOutput()
	if ctx.Err() != nil || cmd.ProcessState == nil {
		t.Fatalf("Java TLC: %v\n%s", err, output)
	}
	checkMultipleConfigResult(t, cmd.ProcessState.ExitCode(), string(output), "", wantCode, counts)
	return string(output)
}
