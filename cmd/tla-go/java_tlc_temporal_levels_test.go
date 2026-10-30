package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJavaTLCTemporalOperatorLevels(t *testing.T) {
	for _, tc := range []struct {
		name, definitions string
		code              int
	}{
		{"always action", `P == [] (x' = x)`, 150},
		{"eventually action", `P == <> (x' = x)`, 150},
		{"action alias", "A == x' = x\nP == [] A", 150},
		{"box alias", "A == [x' = x]_x\nP == [] A", 150},
		{"angle alias", "A == <<x' = x>>_x\nP == <> A", 150},
		{"leads left action", `P == (x' = x) ~> (x = 0)`, 150},
		{"leads right action", `P == (x = 0) ~> (x' = x)`, 150},
		{"bounded temporal", `P == \A i \in {x'} : [] (x = 0)`, 150},
		{"constant prime", `P == [] (1' = 1)`, 150},
		{"bound prime", `P == \A i \in {1}: [] (i' = i)`, 150},
		{"always state", `P == [] (x = 0)`, 0},
		{"eventually state", `P == <> (x = 0)`, 0},
		{"box action", `P == [] [x' = x]_x`, 0},
		{"angle action", `P == <> <<x' = x>>_x`, 0},
		{"leads state", `P == (x = 0) ~> (x = 0)`, 0},
		{"bounded state", `P == \A i \in {x'} : i = x'`, 0},
		{"state bound temporal", `P == \A i \in {x} : [] (x = 0)`, 0},
		{"enabled action", "A == x' = x\nP == [] ENABLED A", 0},
		{"enabled action argument", "B(d) == ENABLED d\nP == [] B(x' = x)", 0},
		{"formal parameter", "B(d) == [] d\nP == B(x' = x)", 0},
		{"later definition shadowed by parameter", "B(A) == [] A\nA == x' = x\nP == B(x = 0)", 0},
		{"later definition shadowed by bound", "P == \\A A \\in {TRUE} : [] A\nA == x' = x", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\n" + tc.definitions + "\n====\n"
			for name, contents := range map[string]string{"M.tla": source, "M.cfg": "INIT Init\nNEXT Next\n"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != tc.code {
				t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				runJavaTLCForExit(t, jar, dir, "M", tc.code)
			}
		})
	}
}

// test215 is unchanged from the Java TLC v1.7.4 suite.
func TestJavaTLCTemporalLevelFixture(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"test215.tla", "test215.cfg"} {
		data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{filepath.Join(dir, "test215.tla")}, &out, &errout)
	want := "Action used where only temporal formula or state predicate allowed"
	if code != 150 || !strings.Contains(errout.String(), want) {
		t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java := runJavaTLCForExit(t, jar, dir, "test215", 150)
		if !strings.Contains(java, want) {
			t.Fatalf("Java TLC:\n%s", java)
		}
	}
}
