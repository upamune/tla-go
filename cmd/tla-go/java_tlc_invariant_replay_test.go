package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestJavaTLCInvariantFailureReplay(t *testing.T) {
	for _, tc := range []struct {
		name, invariant, counts string
		width, code             int
		stack                   bool
	}{
		{"one successor", `IF x = 0 THEN TRUE ELSE 0 = "a"`, "2/2/0", 1, 76, false},
		{"repeated error", `IF x = 0 THEN TRUE ELSE 0 = "a"`, "2/3/0", 2, 75, true},
		{"other successors pass", `IF x = 1 THEN 0 = "a" ELSE TRUE`, "2/4/2", 3, 76, false},
		{"division fails again", `1 \div (1 - (x % 2)) >= 0`, "2/4/1", 3, 255, true},
		{"comparison fails again", `IF x % 2 = 1 THEN "a" > 0 ELSE TRUE`, "2/4/1", 3, 255, true},
		{"assert fails again", `Assert(x % 2 = 0, "oops")`, "2/4/1", 3, 14, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := fmt.Sprintf("VARIABLE x\nInit == x = 0\nNext == x' \\in 1..%d\nInv == %s", tc.width, tc.invariant)
			writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nINVARIANT Inv\n")
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", filepath.Join(dir, "M.tla")}, &out, &errout)
			checkMultipleConfigResult(t, code, out.String(), errout.String(), tc.code, tc.counts)
			if got := hasToolMessage(readToolMessages(out.String()), 2103); got != tc.stack {
				t.Fatalf("stack message = %t, want %t: %s", got, tc.stack, out.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runMultipleConfigJava(t, jar, dir, "M", "M", tc.code, tc.counts)
				if got := hasToolMessage(readToolMessages(java), 2103); got != tc.stack {
					t.Fatalf("Java stack message = %t, want %t: %s", got, tc.stack, java)
				}
				compareReplayStacks(t, java, out.String())
			}
		})
	}
}

func compareReplayStacks(t *testing.T, java, got string) {
	t.Helper()
	var expected, actual string
	for _, message := range readToolMessages(java) {
		if message.code == 2103 {
			expected = message.body
		}
	}
	for _, message := range readToolMessages(got) {
		if message.code == 2103 {
			actual = message.body
		}
	}
	if actual != expected {
		t.Fatalf("evaluation stacks differ\nJava: %s\nGo: %s", expected, actual)
	}
}

func TestJavaTLCActionPropertyFailureReplay(t *testing.T) {
	for _, tc := range []struct {
		name, predicate string
		code            int
	}{
		{"equality", `IF x = 0 THEN TRUE ELSE 0 = "a"`, 75},
		{"assert", `Assert(x = 0, "oops")`, 14},
		{"division", `1 \div (1 - x) >= 0`, 255},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == x = 0\nNext == x' \\in 1..2\nP == [][" + tc.predicate + "]_x"
			writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nPROPERTY P\n")
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", filepath.Join(dir, "M.tla")}, &out, &errout)
			checkMultipleConfigResult(t, code, out.String(), errout.String(), tc.code, "4/3/1")
			if !hasToolMessage(readToolMessages(out.String()), 2103) {
				t.Fatalf("missing evaluation stack: %s", out.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runMultipleConfigJava(t, jar, dir, "M", "M", tc.code, "4/3/1")
				compareReplayStacks(t, java, out.String())
				compareRawToolTrace(t, java, out.String())
			}
		})
	}
}

func TestJavaTLCSafetyFailureReplayDOT(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("set TLC_JAR to compare DOT output")
	}
	for _, tc := range []struct{ name, definition, config string }{
		{"invariant", `Inv == IF x = 0 THEN TRUE ELSE 0 = "a"`, "INVARIANT Inv"},
		{"action property", `P == [][IF x = 0 THEN TRUE ELSE 0 = "a"]_x`, "PROPERTY P"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == x' \\in 1..3\n"+tc.definition, "INIT Init\nNEXT Next\n"+tc.config+"\n")
			var out, errout bytes.Buffer
			dot := filepath.Join(dir, "go.dot")
			code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", "-dump", "dot,actionlabels", dot, filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != 75 {
				t.Fatalf("exit %d: %s %s", code, out.String(), errout.String())
			}
			got, err := os.ReadFile(dot)
			if err != nil {
				t.Fatal(err)
			}
			compareJavaDOT(t, jar, dir, got, code)
		})
	}
}
