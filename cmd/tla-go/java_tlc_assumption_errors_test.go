package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These models are unchanged from the Java TLC v1.7.4 test suite.
func TestJavaTLCAssumptionEvaluationFailures(t *testing.T) {
	for _, tc := range []struct{ name, diagnostic string }{
		{"etest12", "Overflow when computing the number of elements in:\n[0..2 -> 1..2000]"},
		{"etest13", "Overflow when computing the number of elements in:\n[0..2 -> 1..2000]"},
		{"etest14", "CHOOSE x \\in S: P, but no element of S satisfied P.\nline 5, col 7 to line 5, col 28 of module etest14"},
		{"etest15", "Attempted to compare integer 2 with non-integer:\n<<3>>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, ext := range []string{".tla", ".cfg"} {
				name := tc.name + ext
				data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", name))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, tc.name+".tla")}, &out, &errout)
			if code != 10 || errout.Len() != 0 || !strings.Contains(out.String(), "Error: Evaluating assumption") || !strings.Contains(out.String(), tc.diagnostic) {
				t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runJavaTLCForExit(t, jar, dir, tc.name, 10)
				if !strings.Contains(java, tc.diagnostic) {
					t.Fatalf("Java TLC:\n%s", java)
				}
			}
		})
	}
}

func TestJavaTLCNestedCardinalityOverflow(t *testing.T) {
	dir := t.TempDir()
	source := "---- MODULE M ----\nEXTENDS Naturals, FiniteSets\nASSUME Cardinality([0..1 -> [0..2 -> 1..2000]]) = 0\n====\n"
	for name, contents := range map[string]string{"M.tla": source, "M.cfg": ""} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want := "Overflow when computing the number of elements in:\n[0..2 -> 1..2000]"
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
	if code != 10 || errout.Len() != 0 || !strings.Contains(out.String(), want) {
		t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java := runJavaTLCForExit(t, jar, dir, "M", 10)
		if !strings.Contains(java, want) {
			t.Fatalf("Java TLC:\n%s", java)
		}
	}
}

func TestJavaTLCSetMembershipComparisonOrder(t *testing.T) {
	for _, tc := range []struct{ name, declarations, expression, diagnostic string }{
		{"literal", "", `2 \in {<<1>>, <<2>>, <<3>>, <<4>>}`, "Attempted to check equality of the function <<1>> with the value:\n2"},
		{"named constant", `S == {<<1>>, <<2>>, <<3>>, <<4>>}`, `2 \in S`, "Attempted to compare integer 2 with non-integer:\n<<3>>"},
		{"nested constant", `S == [a |-> {<<1>>, <<2>>, <<3>>, <<4>>}]`, `2 \in S.a`, "Attempted to compare integer 2 with non-integer:\n<<3>>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := "---- MODULE M ----\n" + tc.declarations + "\nASSUME " + tc.expression + "\n====\n"
			for name, contents := range map[string]string{"M.tla": source, "M.cfg": ""} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != 10 || errout.Len() != 0 || !strings.Contains(out.String(), tc.diagnostic) {
				t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runJavaTLCForExit(t, jar, dir, "M", 10)
				if !strings.Contains(java, tc.diagnostic) {
					t.Fatalf("Java TLC:\n%s", java)
				}
			}
		})
	}
}

func TestJavaTLCStateMembershipNormalization(t *testing.T) {
	for _, tc := range []struct{ name, initial, member, extraConfig string }{
		{"set", `{<<1>>, <<2>>, <<3>>, <<4>>}`, "x", ""},
		{"nested record", `[item |-> {<<1>>, <<2>>, <<3>>, <<4>>}]`, "x.item", ""},
		{"view", `{<<1>>, <<2>>, <<3>>, <<4>>}`, "x", "VIEW V\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := "---- MODULE M ----\nVARIABLE x\nInit == x = " + tc.initial + "\nNext == UNCHANGED x\nV == 0\nInv == 2 \\notin " + tc.member + "\n====\n"
			for name, contents := range map[string]string{"M.tla": source, "M.cfg": "INIT Init\nNEXT Next\nINVARIANT Inv\n" + tc.extraConfig} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			want := "Attempted to compare integer 2 with non-integer:\n<<3>>"
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != 75 || !strings.Contains(out.String(), want) {
				t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runJavaTLCForExit(t, jar, dir, "M", 75)
				if !strings.Contains(java, want) {
					t.Fatalf("Java TLC:\n%s", java)
				}
			}
		})
	}
}

func TestJavaTLCConstraintBeforeStateNormalization(t *testing.T) {
	for _, extra := range []string{"", "VIEW V\n"} {
		t.Run(strings.TrimSpace(extra), func(t *testing.T) {
			dir := t.TempDir()
			source := "---- MODULE M ----\nVARIABLE x\nInit == x = {<<1>>, <<2>>, <<3>>, <<4>>}\nNext == UNCHANGED x\nV == 0\nC == 2 \\notin x\n====\n"
			for name, contents := range map[string]string{"M.tla": source, "M.cfg": "INIT Init\nNEXT Next\nCONSTRAINT C\n" + extra} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			want := "Attempted to check equality of the function <<1>> with the value:\n2"
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != 75 || !strings.Contains(out.String(), want) {
				t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runJavaTLCForExit(t, jar, dir, "M", 75)
				if !strings.Contains(java, want) {
					t.Fatalf("Java TLC:\n%s", java)
				}
			}
		})
	}
}
