package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The expected codes and text follow Naturals.java, Integers.java and MP.java
// from the Java TLC 1.7.4 source pinned for this compatibility suite.
func TestJavaTLCArithmeticErrors(t *testing.T) {
	cases := []struct {
		name, expr string
		code       int
		message    string
	}{
		{"addition", "2147483647 + 1", 2178, "Overflow when computing 2147483647+1"},
		{"subtraction", "(0 - 2147483647 - 1) - 1", 2178, "Overflow when computing -2147483648-1"},
		{"multiplication", "50000 * 50000", 2178, "Overflow when computing 50000*50000"},
		{"negation", "-(0 - 2147483647 - 1)", 2178, "Overflow when computing --2147483648"},
		{"division overflow", "(0 - 2147483647 - 1) \\div (0 - 1)", 2178, "Overflow when computing -2147483648 \\div -1"},
		{"power overflow", "2 ^ 31", 2178, "Overflow when computing 2^31"},
		{"negative exponent", "2 ^ (0 - 1)", 2169, "The second argument of ^ should be a natural number, but instead it is:\n-1"},
		{"zero power", "0 ^ 0", 2180, "0^0 is undefined."},
		{"zero modulo", "1 % 0", 2169, "The second argument of % should be a positive number, but instead it is:\n0"},
		{"negative modulo", "1 % (0 - 2)", 2169, "The second argument of % should be a positive number, but instead it is:\n-2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == x' = "+tc.expr, "INIT Init\nNEXT Next\n")
			path := filepath.Join(dir, "M.tla")
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(strings.Replace(string(source), "EXTENDS Naturals,", "EXTENDS Integers,", 1)), 0o600); err != nil {
				t.Fatal(err)
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", filepath.Join(dir, "M.tla")}, &out, &errout)
			want := fmt.Sprintf("@!@!@STARTMSG %d:1 @!@!@\n%s\n@!@!@ENDMSG %d @!@!@", tc.code, tc.message, tc.code)
			if code != 255 || errout.Len() != 0 || !strings.Contains(out.String(), want) {
				t.Fatalf("exit %d, stderr %s, output:\n%s\nwant %s", code, errout.String(), out.String(), want)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				cmd := exec.Command("java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "M.tla")
				cmd.Dir = dir
				java, err := cmd.CombinedOutput()
				if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code || !strings.Contains(string(java), want) {
					t.Fatalf("Java: %v\n%s\nwant %s", err, java, want)
				}
				gotMessages, javaMessages := readToolMessages(out.String()), readToolMessages(string(java))
				var got, expected []toolMessage
				for _, m := range gotMessages {
					if m.code == tc.code || m.code == 2103 {
						got = append(got, m)
					}
				}
				for _, m := range javaMessages {
					if m.code == tc.code || m.code == 2103 {
						expected = append(expected, m)
					}
				}
				if fmt.Sprint(got) != fmt.Sprint(expected) {
					t.Fatalf("error messages differ\nGo: %+v\nJava: %+v", got, expected)
				}
			}
		})
	}
}

func TestJavaTLCArithmeticErrorContexts(t *testing.T) {
	for _, tc := range []struct{ name, body, cfg string }{
		{"initial", "Init == x = 0 /\\ x = 0 ^ 0\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n"},
		{"initial invariant", "Init == x = 0\nNext == UNCHANGED x\nInv == 0 ^ 0 = x", "INIT Init\nNEXT Next\nINVARIANT Inv\n"},
		{"invariant", "Init == x = 0\nNext == x' = 1\nInv == IF x = 0 THEN TRUE ELSE 0 ^ 0 = x", "INIT Init\nNEXT Next\nINVARIANT Inv\n"},
		{"assumption", "ASSUME 0 ^ 0 = 0\nInit == x = 0\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			jar := os.Getenv("TLC_JAR")
			if jar == "" {
				t.Skip("TLC_JAR is not set")
			}
			dir := t.TempDir()
			writeDOTFixture(t, dir, "VARIABLE x\n"+tc.body, tc.cfg)
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", filepath.Join(dir, "M.tla")}, &out, &errout)
			cmd := exec.Command("java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "M.tla")
			cmd.Dir = dir
			java, err := cmd.CombinedOutput()
			if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code {
				t.Fatalf("Go exit %d:\n%s\nJava: %v\n%s", code, out.String(), err, java)
			}
			selectErrors := func(output string) []toolMessage {
				var selected []toolMessage
				for _, m := range readToolMessages(output) {
					if m.code == 2102 || m.code == 2103 || m.code == 2105 || m.code == 2111 || m.code == 2180 {
						selected = append(selected, m)
					}
				}
				return selected
			}
			got, want := selectErrors(out.String()), selectErrors(string(java))
			if fmt.Sprint(got) != fmt.Sprint(want) {
				t.Fatalf("Go: %+v\nJava: %+v\nGo output: %s\nJava output: %s", got, want, out.String(), java)
			}
		})
	}
}

func TestJavaTLCNaturalDivisionBoundaries(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == UNCHANGED x\nInv == /\\ (0 - 1) \\div 2 = 0\n       /\\ (0 - 2147483647 - 1) \\div (0 - 1) = (0 - 2147483647 - 1)", "INIT Init\nNEXT Next\nINVARIANT Inv\n")
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", filepath.Join(dir, "M.tla")}, &out, &errout)
	if code != 0 || errout.Len() != 0 {
		t.Fatalf("exit %d, stderr %s, output %s", code, errout.String(), out.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareJavaToolMessages(t, jar, dir, code, readToolMessages(out.String()))
	}
}
