package checker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/upamune/tla-go/evaluator"
)

// TLC 1.7.4 fingerprints the product before checking invariants or constraints.
func TestJavaTLCMaxSetSuccessor(t *testing.T) {
	cases := []struct {
		name, next                 string
		bound, generated, distinct int
		terse, failure             bool
	}{
		{"regular", "x' = (1..4) \\X (1..4)", 10, 2, 1, false, true},
		{"domain-no-source", "x' = DOMAIN [i \\in 1..4, j \\in 1..4 |-> 0]", 10, 3, 2, false, false},
		{"domain-terse", "x' = DOMAIN [i \\in 1..4, j \\in 1..4 |-> 0]", 10, 3, 2, true, false},
		{"domain-at-limit", "x' = DOMAIN [i \\in 1..4, j \\in 1..4 |-> 0]", 16, 3, 2, false, false},
		{"domain-empty", "x' = DOMAIN [i \\in 1..4, j \\in 1..0 |-> 0]", 10, 3, 2, false, false},
		{"terse", "x' = (1..4) \\X (1..4)", 10, 2, 1, true, true},
		{"below-limit", "x' = (1..4) \\X (1..4)", 16, 3, 2, false, false},
		{"default", "x' = (1..4) \\X (1..4)", 0, 3, 2, false, false},
		{"minimum-capacity", "x' = (1..3) \\X (1..3)", 5, 3, 2, false, false},
		{"minimum-capacity-exceeded", "x' = (1..3) \\X (1..4)", 5, 2, 1, false, true},
		{"large-product", "x' = (1..1000000000) \\X (1..1000000000)", 10, 2, 1, false, true},
		{"later-state", "IF x = 0 THEN x' = 1 ELSE x' = (1..4) \\X (1..4)", 10, 3, 2, false, true},
		{"subset-control", "x' = SUBSET (1..5)", 10, 3, 2, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := fmt.Sprintf("---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == %s\n====\n", tc.next)
			cfg := "INIT Init NEXT Next CHECK_DEADLOCK FALSE\n"
			got, err := CheckWithOptions(context.Background(), source, cfg, Options{MaxSetSize: tc.bound, PrintStyle: evaluator.PrintStyle{Terse: tc.terse}})
			if err != nil {
				t.Fatal(err)
			}
			if got.GeneratedStates != tc.generated || got.DistinctStates != tc.distinct {
				t.Fatalf("counts: %+v", got)
			}
			if tc.failure {
				f := got.Failure
				if got.Status != EvaluationFailure || f == nil || f.ToolCode != 1000 || f.ExitCode != 75 || f.SuppressStatistics || !strings.Contains(f.Message, fmt.Sprintf("java.lang.RuntimeException\n: util.WrongInvocationException: Attempted to construct a set with too many elements (>%d).", tc.bound)) || !strings.Contains(f.FingerprintDetail, "Fingerprint Stack Trace:") || !strings.Contains(f.FingerprintDetail, fmt.Sprintf("Reason:\nAttempted to construct a set with too many elements (>%d).", tc.bound)) {
					t.Fatalf("failure: %+v", f)
				}

			} else if got.Status != Success {
				t.Fatalf("result: %+v", got)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareMaxSetSuccessor(t, jar, source, cfg, tc.bound, tc.terse, tc.failure, tc.generated, tc.distinct)
			}
		})
	}
}

func TestJavaTLCMaxSetInitialActionConstraint(t *testing.T) {
	for _, tc := range []struct {
		name    string
		bound   int
		failure bool
	}{
		{"exceeded", 10, true},
		{"at-limit", 12, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = (1..3) \\X (1..4)\nNext == UNCHANGED x\nA == TRUE\n====\n"
			cfg := "INIT Init NEXT Next ACTION_CONSTRAINT A CHECK_DEADLOCK FALSE\n"
			got, err := CheckWithOptions(context.Background(), source, cfg, Options{MaxSetSize: tc.bound})
			if err != nil {
				t.Fatal(err)
			}
			if tc.failure {
				if got.Status != EvaluationFailure || got.Failure == nil || got.Failure.ToolCode != 1000 || got.Failure.ExitCode != 255 || !strings.Contains(got.Failure.Message, "Attempted to construct a set with too many elements (>10).") {
					t.Fatalf("result: %+v failure: %+v", got, got.Failure)
				}
			} else if got.Status != Success {
				t.Fatalf("result: %+v", got)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				dir := t.TempDir()
				for name, contents := range map[string]string{"M.tla": source, "M.cfg": cfg} {
					if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				cmd := exec.Command("java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-jar", jar, "-tool", "-workers", "1", "-maxSetSize", fmt.Sprint(tc.bound), "M.tla")
				cmd.Dir = dir
				out, err := cmd.CombinedOutput()
				if tc.failure {
					exit, ok := err.(*exec.ExitError)
					if !ok || exit.ExitCode() != 255 || !strings.Contains(string(out), "Attempted to construct a set with too many elements (>10).") {
						t.Fatalf("Java: %v\n%s", err, out)
					}
				} else if err != nil {
					t.Fatalf("Java: %v\n%s", err, out)
				}
			}
		})
	}
}

func compareMaxSetSuccessor(t *testing.T, jar, source, cfg string, bound int, terse, failure bool, generated, distinct int) {
	t.Helper()
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, contents := range map[string]string{"M.tla": source, "M.cfg": cfg} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	args := []string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-tool", "-workers", "1", "-fp", "0"}
	if bound > 0 {
		args = append(args, "-maxSetSize", fmt.Sprint(bound))
	}
	if terse {
		args = append(args, "-terse")
	}
	args = append(args, "M.tla")
	cmd := exec.CommandContext(ctx, "java", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("Java timeout: %v\n%s", ctx.Err(), out)
	}
	if failure {
		exit, ok := err.(*exec.ExitError)
		if !ok || exit.ExitCode() != 75 || !strings.Contains(string(out), "STARTMSG 1000:1") || !strings.Contains(string(out), "STARTMSG 2147:1") || !strings.Contains(string(out), fmt.Sprintf("Attempted to construct a set with too many elements (>%d).", bound)) {
			t.Fatalf("Java failure: %v\n%s", err, out)
		}
	} else if err != nil {
		t.Fatalf("Java: %v\n%s", err, out)
	}
	stats := fmt.Sprintf("%d states generated, %d distinct states found", generated, distinct)
	if !strings.Contains(string(out), stats) {
		t.Fatalf("missing %s:\n%s", stats, out)
	}
}

// These controls document deliberately unsupported paths and VIEW behavior.
func TestJavaTLCMaxSetSuccessorControls(t *testing.T) {
	cases := []struct {
		name, variables, init, next, defs, config string
		javaFailure                               bool
	}{
		{"nested", "x", "x = 0", "x' = <<(1..4) \\X (1..4)>>", "", "", true},
		{"function-set", "x", "x = 0", "x' = [1..4 -> 1..2]", "", "", true},
		{"record-set", "x", "x = 0", "x' = [a: 1..4, b: 1..4]", "", "", true},
		{"multiple-variables", "x, y", "/\\ x = 0 /\\ y = 0", "/\\ x' = (1..4) \\X (1..4) /\\ y' = y", "", "", true},
		{"constraint-accept", "x", "x = 0", "x' = (1..4) \\X (1..4)", "C == TRUE", "CONSTRAINT C", true},
		{"action-constraint", "x", "x = 0", "x' = (1..4) \\X (1..4)", "A == TRUE", "ACTION_CONSTRAINT A", true},
		{"view-constant", "x", "x = 0", "x' = (1..4) \\X (1..4)", "V == 0", "VIEW V", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := fmt.Sprintf("---- MODULE M ----\nEXTENDS Naturals\nVARIABLES %s\nInit == %s\nNext == %s\n%s\n====\n", tc.variables, tc.init, tc.next, tc.defs)
			cfg := "INIT Init NEXT Next CHECK_DEADLOCK FALSE\n" + tc.config + "\n"
			got, err := CheckWithOptions(context.Background(), source, cfg, Options{MaxSetSize: 10})
			if err != nil || got.Status != Success {
				t.Fatalf("unsupported control: %+v, %v", got, err)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareMaxSetSuccessor(t, jar, source, cfg, 10, false, tc.javaFailure, 2, 1)
			}
		})
	}
}

// A rejecting constraint still follows fingerprinting in Java TLC.
func TestJavaTLCMaxSetRejectedSuccessor(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR not set")
	}
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = (1..4) \\X (1..4)\nC == x = 0\n====\n"
	compareMaxSetSuccessor(t, jar, source, "INIT Init NEXT Next CHECK_DEADLOCK FALSE CONSTRAINT C\n", 10, false, true, 2, 1)
}

// A state constraint does not force the lazy function domain to be enumerated.
func TestJavaTLCMaxSetDomainConstraint(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = DOMAIN [i \\in 1..4, j \\in 1..4 |-> 0]\nC == TRUE\n====\n"
	cfg := "INIT Init NEXT Next CHECK_DEADLOCK FALSE CONSTRAINT C\n"
	got, err := CheckWithOptions(context.Background(), source, cfg, Options{MaxSetSize: 10})
	if err != nil || got.Status != Success || got.GeneratedStates != 3 || got.DistinctStates != 2 {
		t.Fatalf("result: %+v, %v", got, err)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareMaxSetSuccessor(t, jar, source, cfg, 10, false, false, 3, 2)
	}
}

// Java enumerates a filtered domain and reaches its evaluator bound.
// This is a Java-only control for a deferred-set path not yet supported in Go.
func TestJavaTLCMaxSetEnumeratedDomain(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = {z \\in DOMAIN [i \\in 1..4, j \\in 1..4 |-> 0]: TRUE}\n====\n"
	cfg := "INIT Init NEXT Next CHECK_DEADLOCK FALSE\n"
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		dir := t.TempDir()
		for name, contents := range map[string]string{"M.tla": source, "M.cfg": cfg} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		cmd := exec.Command("java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "-maxSetSize", "10", "M.tla")
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		exit, ok := err.(*exec.ExitError)
		if !ok || exit.ExitCode() != 75 || !strings.Contains(string(out), "too many elements (>10)") || !strings.Contains(string(out), "1 states generated, 1 distinct states found") {
			t.Fatalf("Java: %v\n%s", err, out)
		}
	}
}
