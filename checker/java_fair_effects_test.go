package checker

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/upamune/tla-go/internal/javatmp"
)

// Java TLC v1.7.4, commit 5a47802b5c391f59ecdd44117981f4ff8c0656ba.
// These characterize a known side-effect divergence without asserting the
// incorrect Go output as the desired result. Set TLC_JAR to run the oracle.
func TestJavaTLCFairnessEffectsLifecycle(t *testing.T) {
	cases := []struct {
		name, init, effect, fair, property, constraint string
		printed                                        []string
		status                                         Status
		generated, distinct                            int
		comparable                                     bool
	}{
		{"weak-violation", `x = 0`, `PrintT(<<x,x'>>)`, `Next`, `<>(x = 2)`, "", []string{"<<0, 1>>", "<<0, 1>>", "<<0, 1>>", "<<0, 1>>", "<<1, 0>>", "<<1, 0>>", "<<1, 0>>", "<<1, 0>>", "<<0, 1>>", "<<1, 0>>"}, LivenessViolation, 3, 2, false},
		{"weak-satisfied", `x = 0`, `PrintT(<<x,x'>>)`, `Next`, `<>(x = 1)`, "", []string{"<<0, 1>>", "<<0, 1>>", "<<0, 1>>", "<<0, 1>>", "<<1, 0>>", "<<1, 0>>", "<<1, 0>>", "<<1, 0>>"}, Success, 3, 2, false},
		{"state-expression", `x = 0`, `PrintT(x)`, `Next`, `<>(x = 2)`, "", []string{"0", "0", "0", "0", "1", "1", "1", "1", "0", "1"}, LivenessViolation, 3, 2, false},
		{"print", `x = 0`, `Print(<<x,x'>>, TRUE)`, `Next`, `<>(x = 2)`, "", []string{"<<0, 1>>  TRUE", "<<0, 1>>  TRUE", "<<0, 1>>  TRUE", "<<0, 1>>  TRUE", "<<1, 0>>  TRUE", "<<1, 0>>  TRUE", "<<1, 0>>  TRUE", "<<1, 0>>  TRUE", "<<0, 1>>  TRUE", "<<1, 0>>  TRUE"}, LivenessViolation, 3, 2, false},
		{"no-fairness", `x = 0`, `PrintT(<<x,x'>>)`, ``, `<>(x = 2)`, "", []string{"<<0, 1>>", "<<1, 0>>"}, LivenessViolation, 3, 2, true},
		{"disabled-fairness", `x = 0`, `PrintT(<<x,x'>>)`, `/\ x = 3 /\ Next`, `<>(x = 2)`, "", []string{"<<0, 1>>", "<<1, 0>>", "<<0, 1>>", "<<1, 0>>"}, LivenessViolation, 3, 2, false},
		{"constraint", `x = 0`, `PrintT(<<x,x'>>)`, `Next`, `<>(x = 2)`, "CONSTRAINT C", []string{"<<0, 1>>", "<<0, 1>>"}, Success, 2, 1, true},
		{"action-constraint", `x = 0`, `PrintT(<<x,x'>>)`, `Next`, `<>(x = 2)`, "ACTION_CONSTRAINT AC", []string{"<<0, 1>>", "<<0, 1>>"}, Success, 2, 1, true},
		{"register", `x = 0 /\ TLCSet(0, 0)`, `/\ TLCSet(0,TLCGet(0)+1) /\ PrintT(<<x,x',TLCGet(0)>>)`, `Next`, `<>(x = 2)`, "", []string{"<<0, 1, 1>>", "<<0, 1, 2>>", "<<0, 1, 3>>", "<<0, 1, 4>>", "<<1, 0, 5>>", "<<1, 0, 6>>", "<<1, 0, 7>>", "<<1, 0, 8>>", "<<0, 1, 1>>", "<<1, 0, 2>>"}, LivenessViolation, 3, 2, false},
		{"pure-violation", `x = 0`, `TRUE`, `Next`, `<>(x = 2)`, "", nil, LivenessViolation, 3, 2, true},
		{"pure-satisfied", `x = 0`, `TRUE`, `Next`, `<>(x = 1)`, "", nil, Success, 3, 2, true},
		{"register-read", `x = 0 /\ TLCSet(0, 42)`, `PrintT(<<x,x',TLCGet(0)>>)`, `Next`, `<>(x = 2)`, "", []string{"<<0, 1, 42>>", "<<0, 1, 42>>", "<<0, 1, 42>>", "<<0, 1, 42>>", "<<1, 0, 42>>", "<<1, 0, 42>>", "<<1, 0, 42>>", "<<1, 0, 42>>", "<<0, 1, 42>>", "<<1, 0, 42>>"}, LivenessViolation, 3, 2, false},
	}
	jar := os.Getenv("TLC_JAR")
	if jar != "" {
		data, err := os.ReadFile(jar)
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != "936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88" {
			t.Fatalf("TLC_JAR is not the pinned v1.7.4 jar: sha256 %s", got)
		}
		jar, err = filepath.Abs(jar)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fairness := ""
			if tc.fair != "" {
				fairness = " /\\ WF_x(Fair)"
			}
			fair := tc.fair
			if fair == "" {
				fair = "Next"
			}
			source := fmt.Sprintf("---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == %s\nNext == /\\ x' = (x + 1) %% 2 /\\ %s\nFair == %s\nSpec == Init /\\ [][Next]_x%s\nP == %s\nC == x = 0\nAC == x' = 0\n====\n", tc.init, tc.effect, fair, fairness, tc.property)
			cfg := "SPECIFICATION Spec\nPROPERTY P\nCHECK_DEADLOCK FALSE\n" + tc.constraint + "\n"
			var printed []string
			got, err := CheckWithOptions(context.Background(), source, cfg, Options{OnPrint: func(s string) { printed = append(printed, s) }})
			if err != nil || got.Status != tc.status || got.GeneratedStates != tc.generated || got.DistinctStates != tc.distinct {
				t.Fatalf("Go result %+v, %v", got, err)
			}
			if tc.comparable && !reflect.DeepEqual(printed, tc.printed) {
				t.Fatalf("Go output %q; Java output %q", printed, tc.printed)
			}
			if !tc.comparable && !reflect.DeepEqual(printed, tc.printed) {
				t.Logf("known divergence: Go %q; Java %q", printed, tc.printed)
			}
			if jar == "" {
				return
			}
			for _, tool := range []bool{false, true} {
				dir := t.TempDir()
				if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(source), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte(cfg), 0o600); err != nil {
					t.Fatal(err)
				}
				args := []string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-workers", "1", "-fp", "0", "-metadir", filepath.Join(dir, "meta")}
				if tool {
					args = append(args, "-tool")
				}
				args = append(args, "M.tla")
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				cmd := exec.CommandContext(ctx, "java", args...)
				cmd.Dir = dir
				javatmp.Apply(cmd)
				out, runErr := cmd.CombinedOutput()
				cancel()
				wantExit := 0
				if tc.status == LivenessViolation {
					wantExit = 13
				}
				exit := 0
				if runErr != nil {
					if e, ok := runErr.(*exec.ExitError); ok {
						exit = e.ExitCode()
					} else {
						t.Fatal(runErr)
					}
				}
				var lines []string
				for line := range strings.SplitSeq(string(out), "\n") {
					if strings.HasPrefix(line, "<<") || line == "0" || line == "1" {
						lines = append(lines, line)
					}
				}
				stats := fmt.Sprintf("%d states generated, %d distinct states found", tc.generated, tc.distinct)
				if exit != wantExit || !strings.Contains(string(out), stats) || !reflect.DeepEqual(lines, tc.printed) {
					t.Fatalf("tool=%v exit=%d output=%q want=%q\n%s", tool, exit, lines, tc.printed, out)
				}
			}
		})
	}
}

// Writes can alter subsequent generation and even make a liveness check fail.
// Keep these oracle cases separate from the lifecycle cases, whose Go state
// counts and verdicts are asserted to match Java.
func TestJavaTLCFairnessRegisterSensitiveOracle(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("set TLC_JAR to the pinned Java TLC 1.7.4 jar")
	}
	data, err := os.ReadFile(jar)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != "936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88" {
		t.Fatalf("TLC_JAR is not the pinned v1.7.4 jar: sha256 %s", got)
	}
	jar, err = filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, guard string
		exit        int
		printed     []string
	}{
		{"guard", `TLCGet(0) < 4`, 13, []string{"<<0, 1, 1>>", "<<0, 1, 2>>", "<<0, 1, 3>>", "<<0, 1, 4>>", "<<1, 0, 5>>", "<<1, 0, 6>>", "<<0, 1, 1>>"}},
		{"assertion", `Assert(TLCGet(0) < 4, "limit")`, 14, []string{"<<0, 1, 1>>", "<<0, 1, 2>>", "<<0, 1, 3>>", "<<0, 1, 4>>", "<<0, 1, 5>>"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := fmt.Sprintf("---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x = 0 /\\ TLCSet(0, 0)\nNext == /\\ x' = (x + 1) %% 2 /\\ TLCSet(0,TLCGet(0)+1) /\\ PrintT(<<x,x',TLCGet(0)>>) /\\ %s\nSpec == Init /\\ [][Next]_x /\\ WF_x(Next)\nP == <>(x = 3)\n====\n", tc.guard)
			cfg := "SPECIFICATION Spec\nPROPERTY P\nCHECK_DEADLOCK FALSE\n"
			var goPrinted []string
			got, goErr := CheckWithOptions(context.Background(), source, cfg, Options{OnPrint: func(s string) { goPrinted = append(goPrinted, s) }})
			t.Logf("Go result %+v, error %v, output %q", got, goErr, goPrinted)
			for _, tool := range []bool{false, true} {
				for range 3 {
					dir := t.TempDir()
					if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(source), 0o600); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte(cfg), 0o600); err != nil {
						t.Fatal(err)
					}
					args := []string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-workers", "1", "-fp", "0", "-metadir", filepath.Join(dir, "meta")}
					if tool {
						args = append(args, "-tool")
					}
					args = append(args, "M.tla")
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					cmd := exec.CommandContext(ctx, "java", args...)
					cmd.Dir = dir
					javatmp.Apply(cmd)
					out, runErr := cmd.CombinedOutput()
					cancel()
					exit := 0
					if runErr != nil {
						if e, ok := runErr.(*exec.ExitError); ok {
							exit = e.ExitCode()
						} else {
							t.Fatal(runErr)
						}
					}
					var printed []string
					for line := range strings.SplitSeq(string(out), "\n") {
						if strings.HasPrefix(line, "<<") {
							printed = append(printed, line)
						}
					}
					if exit != tc.exit || !strings.Contains(string(out), "2 states generated, 2 distinct states found") || !reflect.DeepEqual(printed, tc.printed) {
						t.Fatalf("tool=%v exit=%d output=%q; want=%q\n%s", tool, exit, printed, tc.printed, out)
					}
				}
			}
		})
	}
}
