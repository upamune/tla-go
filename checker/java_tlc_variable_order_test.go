package checker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Compare SANY's variable order with the order attached to a violation.
func TestJavaTLCVariableOrder(t *testing.T) {
	for _, tc := range []struct {
		name, extends string
		variables     []string
	}{
		{"three", "Naturals", []string{"x", "y", "z"}},
		{"reordered", "Naturals", []string{"z", "x", "y"}},
		{"six", "Naturals, TLC", []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta"}},
		{"hash-collisions", "Integers", []string{"Aa", "BB", "foo", "bar"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var conjuncts []string
			for _, name := range tc.variables {
				conjuncts = append(conjuncts, name+" = 0")
			}
			source := fmt.Sprintf("---- MODULE M ----\nEXTENDS %s\nVARIABLES %s\nInit == /\\ %s\nNext == UNCHANGED <<%s>>\nInv == %s # 0\n====\n", tc.extends, strings.Join(tc.variables, ", "), strings.Join(conjuncts, " /\\ "), strings.Join(tc.variables, ", "), tc.variables[0])
			cfg := "INIT Init NEXT Next INVARIANT Inv\n"
			got, err := Check(source, cfg)
			if err != nil || got.Status != InvariantViolation || got.Violation == nil || len(got.Violation.Trace) != 1 {
				t.Fatalf("got %+v, %v", got, err)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				order := javaInitialStateOrder(t, jar, source, cfg, 12)
				if !reflect.DeepEqual(got.Violation.Trace[0].StateOrder, order) {
					t.Fatalf("got order %v; Java TLC returned %v", got.Violation.Trace[0].StateOrder, order)
				}
			}
		})
	}
}

func TestJavaTLCInitialEvaluationFailureOrder(t *testing.T) {
	source := "---- MODULE M ----\nVARIABLES x, y, z\nInit == /\\ x = 0 /\\ y = 1 /\\ z = 2\nNext == UNCHANGED <<x, y, z>>\nInv == <<1>>[2] = x\n====\n"
	cfg := "INIT Init NEXT Next INVARIANT Inv\n"
	got, err := Check(source, cfg)
	if err != nil || got.Status != EvaluationFailure || got.Failure == nil || got.Failure.InitialState == nil {
		t.Fatalf("got %+v, %v", got, err)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		order := javaInitialStateOrder(t, jar, source, cfg, 75)
		if !reflect.DeepEqual(got.Failure.InitialStateOrder, order) {
			t.Fatalf("got order %v; Java TLC returned %v", got.Failure.InitialStateOrder, order)
		}
	}
}

func javaInitialStateOrder(t *testing.T, jar, source, cfg string, expectedExit int) []string {
	t.Helper()
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, content := range map[string]string{"M.tla": source, "M.cfg": cfg} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-workers", "1", "-fp", "0", "M.tla")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if ctx.Err() != nil || !ok || exit.ExitCode() != expectedExit {
		t.Fatalf("Java TLC: %v, context: %v\n%s", err, ctx.Err(), out)
	}
	matches := regexp.MustCompile(`(?m)^/\\ ([A-Za-z_][A-Za-z_0-9]*) = `).FindAllStringSubmatch(string(out), -1)
	var order []string
	for _, match := range matches {
		order = append(order, match[1])
	}
	return order
}
