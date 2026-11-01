package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The Java reference is TLC v1.7.4; see docs/max-set-size-java-1.7.4.md.
func TestJavaTLCInitialProductBound(t *testing.T) {
	for _, tc := range []struct {
		name, body, cfg string
		code            int
		contains        string
	}{
		{"above", "Init == x = (1..3) \\X (1..4)\nNext == UNCHANGED x", "", 255, "Attempted to construct a set with too many elements (>10)."},
		{"at", "Init == x = (1..2) \\X (1..5)\nNext == UNCHANGED x", "", 0, ""},
		{"empty", "Init == x = (1..0) \\X (1..100)\nNext == UNCHANGED x", "", 0, ""},
		{"powerset", "Init == x = SUBSET (1..4)\nNext == UNCHANGED x", "", 0, ""},
		{"constraint excludes", "Init == x = (1..3) \\X (1..4)\nNext == UNCHANGED x\nC == Cardinality(x) < 0", "CONSTRAINT C\n", 0, ""},
		{"evaluation error", "Init == x = (1..3) \\X (1..4) /\\ 1[1] = 0\nNext == UNCHANGED x", "", 75, "non-function"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeMaxSetSizeFixture(t, dir, "VARIABLE x\n"+tc.body, "INIT Init\nNEXT Next\n"+tc.cfg)
			args := []string{"-tool", "-deadlock", "-workers", "1", "-maxSetSize", "10"}
			var out, errout bytes.Buffer
			code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
			if code != tc.code || errout.Len() != 0 || !strings.Contains(strings.ToLower(out.String()), strings.ToLower(tc.contains)) {
				t.Fatalf("exit %d want %d, output:\n%s\nstderr: %s", code, tc.code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runJavaToolMessages(t, jar, dir, append(args, "M.tla"), tc.code)
				got := errorBehaviorMessages(readToolMessages(out.String()))
				if !reflect.DeepEqual(got, java) {
					t.Fatalf("messages differ\nJava: %q\nGo:   %q", java, got)
				}
			}
		})
	}
}

func TestJavaTLCInitialProductBoundEffects(t *testing.T) {
	dir := t.TempDir()
	body := "VARIABLE x\nInit == PrintT(\"initial marker\") /\\ PrintT(\"second marker\") /\\ x = (1..3) \\X (1..4)\nNext == UNCHANGED x"
	writeMaxSetSizeFixture(t, dir, body, "INIT Init\nNEXT Next\n")
	args := []string{"-tool", "-difftrace", "-deadlock", "-workers", "1", "-maxSetSize", "10"}
	var out, errout bytes.Buffer
	code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
	check := func(output string, exit int) {
		t.Helper()
		first, second, failure := strings.Index(output, "\"initial marker\""), strings.Index(output, "\"second marker\""), strings.Index(output, "Attempted to construct a set with too many elements")
		if exit != 255 || first < 0 || second <= first || failure <= second || strings.Count(output, "\"initial marker\"") != 1 || strings.Count(output, "\"second marker\"") != 1 {
			t.Fatalf("exit %d; effect order:\n%s", exit, output)
		}
		messages := readToolMessages(output)
		got := fingerprintToolMessages(messages)
		if len(got) != 1 || got[0].code != 1000 {
			t.Fatalf("fingerprint/trace messages: %+v", got)
		}
	}
	if errout.Len() != 0 {
		t.Fatal(errout.String())
	}
	check(out.String(), code)
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		output, javaCode := runJavaTLCArgs(t, jar, dir, append(args, "M.tla"))
		check(output, javaCode)
		if got, want := fingerprintToolMessages(readToolMessages(out.String())), fingerprintToolMessages(readToolMessages(output)); !reflect.DeepEqual(got, want) {
			t.Fatalf("Java: %+v\nGo: %+v", want, got)
		}
	}
}

// In terse mode Java's state printing does not expand the product; the
// fingerprint error and replayed effects differ from the normal 255 path.
func TestJavaTLCInitialProductBoundTerseReference(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	dir := t.TempDir()
	writeMaxSetSizeFixture(t, dir, "VARIABLE x\nInit == PrintT(\"marker\") /\\ x = (1..3) \\X (1..4)\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n")
	output, code := runJavaTLCArgs(t, jar, dir, []string{"-tool", "-terse", "-deadlock", "-workers", "1", "-maxSetSize", "10", "M.tla"})
	messages := fingerprintToolMessages(readToolMessages(output))
	if code != 75 || strings.Count(output, "\"marker\"") != 2 || len(messages) != 2 || messages[0].code != 2102 || messages[1].code != 2147 {
		t.Fatalf("exit %d, messages %+v; output:\n%s", code, messages, output)
	}
}

// Java's attempt to print the initial state can replace the constraint error.
// This is a reference observation, not a claim of Go compatibility.
func TestJavaTLCInitialProductConstraintErrorReference(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	dir := t.TempDir()
	writeMaxSetSizeFixture(t, dir, "VARIABLE x\nInit == x = (1..3) \\X (1..4)\nNext == UNCHANGED x\nC == x[1] = 0", "INIT Init\nNEXT Next\nCONSTRAINT C\n")
	got := runJavaToolMessages(t, jar, dir, []string{"-tool", "-deadlock", "-workers", "1", "-maxSetSize", "10", "M.tla"}, 255)
	if len(got) != 1 || !strings.Contains(got[0], "1000:1:") || !strings.Contains(got[0], "util.WrongInvocationException") {
		t.Fatalf("Java messages: %q", got)
	}
}
