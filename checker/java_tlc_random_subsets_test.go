package checker

import (
	"context"
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

// The boundary cases mirror RandomizationTest in Java TLC 1.7.4.
func TestJavaTLCRandomSetOfSubsetsBoundaries(t *testing.T) {
	const source = `---- MODULE M ----
EXTENDS Naturals, TLC, Randomization
VARIABLE x
Init == x = 0
Next == UNCHANGED x
ASSUME PrintT(RandomSetOfSubsets(1,0,{}))
ASSUME PrintT(RandomSetOfSubsets(0,0,{}))
ASSUME PrintT(RandomSetOfSubsets(23,0,1..42))
ASSUME PrintT(RandomSetOfSubsets(32,5,1..5))
ASSUME PrintT(RandomSetOfSubsets(4,2,1..5))
ASSUME PrintT(RandomSubset(3,1..20))
====
`
	want := []string{"{{}}", "{}", "{{}}", "{{1, 2, 3, 4, 5}}", "{{}, {3}, {1, 3}, {1, 4, 5}}", "{3, 5, 14}"}
	var printed []string
	got, err := CheckWithOptions(context.Background(), source, javaRandomSubsetConfig, Options{Seed: 1, OnPrint: func(line string) { printed = append(printed, line) }})
	if err != nil || got.Status != Success || !reflect.DeepEqual(printed, want) {
		t.Fatalf("got %+v, %v, output %q; want %q", got, err, printed, want)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareJavaRandomSubsetSequence(t, jar, source, javaRandomSubsetConfig, 1, want)
	}
}

func TestJavaTLCRandomSetOfSubsetsErrors(t *testing.T) {
	for _, tc := range []struct {
		name, args, message string
	}{
		{"first-negative", "-42, 2, 1..5", "The first argument of RandomSetOfSubsets should be a nonnegative integer, but instead it is:\n-42"},
		{"first-string", `"52", 2, 1..5`, "The first argument of RandomSetOfSubsets should be a nonnegative integer, but instead it is:\n\"52\""},
		{"second-negative", "1, -1, 1..5", "The second argument of RandomSetOfSubsets should be a nonnegative integer, but instead it is:\n-1"},
		{"empty-set", "42, 42, {}", "The first argument of RandomSetOfSubsets should be a nonnegative integer that is smaller than the subset's size of 2^0, but instead it is:\n42"},
		{"size-too-large", "23, 10, 1..5", "The second argument of RandomSetOfSubsets should be a nonnegative integer in range 0..Cardinality(S), but instead it is:\n10"},
		{"infinite-set", "42, 42, Nat", "The third argument of RandomSetOfSubsets should be a finite set, but instead it is:\nNat"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := fmt.Sprintf("---- MODULE M ----\nEXTENDS Integers, TLC, Randomization\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\nASSUME PrintT(RandomSetOfSubsets(%s))\n====\n", tc.args)
			got, err := CheckWithOptions(context.Background(), source, javaRandomSubsetConfig, Options{Seed: 1})
			if err != nil || got.Status != AssumptionViolation || got.Violation == nil || !strings.Contains(got.Violation.Message, tc.message) {
				t.Fatalf("got %+v, %v; want %q", got, err, tc.message)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaRandomSetError(t, jar, source, tc.message)
			}
		})
	}
}

func compareJavaRandomSetError(t *testing.T, jar, source, message string) {
	t.Helper()
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, content := range map[string]string{"M.tla": source, "M.cfg": javaRandomSubsetConfig} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-workers", "1", "-seed", "1", "-fp", "0", "-deadlock", "M.tla")
	cmd.Dir = dir
	javatmp.Apply(cmd)
	out, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if ctx.Err() != nil || !ok || exit.ExitCode() != 10 || !strings.Contains(string(out), message) {
		t.Fatalf("Java TLC: %v, context: %v; want %q\n%s", err, ctx.Err(), message, out)
	}
}
