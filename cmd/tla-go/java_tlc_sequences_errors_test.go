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

	"github.com/upamune/tla-go/internal/javatmp"
)

// Sequences.java and MP.java in Java TLC 1.7.4 define these errors.
func TestJavaTLCSequenceErrors(t *testing.T) {
	cases := []struct {
		name, expr string
		code       int
		message    string
	}{
		{"head empty", "Head(<<>>)", 2184, "Attempted to apply Head to the empty sequence."},
		{"head empty string", `Head("")`, 2184, "Attempted to apply Head to the empty sequence."},
		{"tail empty", "Tail(<<>>)", 2184, "Attempted to apply Tail to the empty sequence."},
		{"tail empty string", `Tail("")`, 2184, "Attempted to apply Tail to the empty sequence."},
		{"head invalid", "Head(42)", 2283, "The argument of Head should be a sequence, but instead it is:\n42"},
		{"tail invalid", "Tail(TRUE)", 2283, "The argument of Tail should be a sequence, but instead it is:\nTRUE"},
		{"len invalid", "Len(42)", 2283, "The argument of Len should be a sequence, but instead it is:\n42"},
		{"append invalid", "Append(42, 1)", 2182, "Evaluating an expression of the form Append(v, s) when s is not a sequence:\n42"},
		{"append string invalid", `Append("a", 1)`, 2182, "Evaluating an expression of the form t \\o s when s is not a string:\n1"},
		{"concat first", "42 \\o <<1>>", 2182, "Evaluating an expression of the form s \\o t when s is not a sequence:\n42"},
		{"concat second", "<<1>> \\o 42", 2182, "Evaluating an expression of the form t \\o s when s is not a sequence:\n42"},
		{"subseq first", "SubSeq(42, TRUE, FALSE)", 2169, "The first argument of SubSeq should be a sequence, but instead it is:\n42"},
		{"subseq second", "SubSeq(<<1>>, TRUE, FALSE)", 2169, "The second argument of SubSeq should be a natural number, but instead it is:\nTRUE"},
		{"subseq third", "SubSeq(<<1>>, 1, FALSE)", 2169, "The third argument of SubSeq should be a natural number, but instead it is:\nFALSE"},
		{"subseq start", "SubSeq(<<1>>, 0, 2)", 2183, "The second argument of SubSeq must be in the domain of its first argument:\n<<1>>\n, but instead it is\n0"},
		{"subseq end", "SubSeq(<<1>>, 1, 2)", 2183, "The third argument of SubSeq must be in the domain of its first argument:\n<<1>>\n, but instead it is\n2"},
		{"select nonboolean", "SelectSeq(<<1>>, LAMBDA n : n)", 2169, "The second argument of SelectSeq should be a boolean-valued operator, but instead it is:\n<Operator LAMBDA>"},
		{"select first", "SelectSeq(42, LAMBDA n : TRUE)", 2169, "The first argument of SelectSeq should be a sequence, but instead it is:\n42"},
		{"argument evaluation before validation", "SubSeq(42, 1 \\div 0, 2)", 2179, "The second argument of \\div is 0."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeSequenceFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == x' = "+tc.expr, "INIT Init\nNEXT Next\n")
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", filepath.Join(dir, "M.tla")}, &out, &errout)
			want := fmt.Sprintf("@!@!@STARTMSG %d:1 @!@!@\n%s\n@!@!@ENDMSG %d @!@!@", tc.code, tc.message, tc.code)
			if code != 255 || errout.Len() != 0 || !strings.Contains(out.String(), want) {
				t.Fatalf("exit %d stderr %s output:\n%s\nwant %s", code, errout.String(), out.String(), want)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				cmd := exec.Command("java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "M.tla")
				cmd.Dir = dir
				java, err := javatmp.Apply(cmd).CombinedOutput()
				if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code {
					t.Fatalf("Java: %v\n%s", err, java)
				}
				selectMessages := func(s string) []toolMessage {
					var selected []toolMessage
					for _, m := range readToolMessages(s) {
						if m.code == tc.code || m.code == 2103 {
							selected = append(selected, m)
						}
					}
					return selected
				}
				got, expected := selectMessages(out.String()), selectMessages(string(java))
				if fmt.Sprint(got) != fmt.Sprint(expected) {
					t.Fatalf("Go: %+v\nJava: %+v\nGo output: %s\nJava output: %s", got, expected, out.String(), java)
				}
			}
		})
	}
}

func TestJavaTLCSequenceBoundaries(t *testing.T) {
	dir := t.TempDir()
	body := `VARIABLE x
Init == x = 0
Next == UNCHANGED x
Inv == /\ Head("abc") = "a"
       /\ Tail("abc") = "bc"
       /\ Append("a", "bc") = "abc"
       /\ Len("abc") = 3
       /\ SelectSeq("abc", LAMBDA c : c # "b") = <<"a", "c">>
       /\ (<<"a">> \o "bc") = <<"a", "b", "c">>
       /\ SubSeq(<<1, 2>>, 2, 2) = <<2>>
       /\ SubSeq(<<1>>, 3, 2) = <<>>
       /\ SubSeq("ab", 0, -1) = ""
       /\ SelectSeq(<<>>, LAMBDA n : 1) = <<>>
       /\ SelectSeq(<<1, 2>>, LAMBDA n : n = 2) = <<2>>
       /\ (LET cutoff == 1 IN SelectSeq(<<1, 2>>, LAMBDA n : n > cutoff)) = <<2>>
       /\ Head([i \in 1..2 |-> i]) = 1
       /\ SubSeq([i \in 1..2 |-> i], 1, 2) = <<1, 2>>`
	writeSequenceFixture(t, dir, body, "INIT Init\nNEXT Next\nINVARIANT Inv\n")
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", filepath.Join(dir, "M.tla")}, &out, &errout)
	if code != 0 || errout.Len() != 0 {
		t.Fatalf("exit %d stderr %s output %s", code, errout.String(), out.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		compareJavaToolMessages(t, jar, dir, code, readToolMessages(out.String()))
	}
}

func writeSequenceFixture(t *testing.T, dir, body, cfg string) {
	t.Helper()
	writeDOTFixture(t, dir, body, cfg)
	path := filepath.Join(dir, "M.tla")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(source), "EXTENDS Naturals,", "EXTENDS Integers, Sequences,", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestJavaTLCSequenceErrorContexts(t *testing.T) {
	for _, tc := range []struct{ name, body, cfg string }{
		{"initial", "Init == x = Head(<<>>)\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n"},
		{"initial invariant", "Init == x = 0\nNext == UNCHANGED x\nInv == Head(<<>>) = x", "INIT Init\nNEXT Next\nINVARIANT Inv\n"},
		{"invariant", "Init == x = 0\nNext == x' = 1\nInv == IF x = 0 THEN TRUE ELSE Head(<<>>) = x", "INIT Init\nNEXT Next\nINVARIANT Inv\n"},
		{"assumption", "ASSUME Head(<<>>) = 0\nInit == x = 0\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n"},
		{"nested definition", "Bad(s) == Head(s)\nInit == x = 0\nNext == x' = Bad(<<>>)", "INIT Init\nNEXT Next\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			jar := os.Getenv("TLC_JAR")
			if jar == "" {
				t.Skip("TLC_JAR is not set")
			}
			dir := t.TempDir()
			writeSequenceFixture(t, dir, "VARIABLE x\n"+tc.body, tc.cfg)
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", filepath.Join(dir, "M.tla")}, &out, &errout)
			cmd := exec.Command("java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "M.tla")
			cmd.Dir = dir
			java, err := javatmp.Apply(cmd).CombinedOutput()
			if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code {
				t.Fatalf("Go exit %d: %s\nJava: %v\n%s", code, out.String(), err, java)
			}
			selectErrors := func(s string) []toolMessage {
				var result []toolMessage
				for _, m := range readToolMessages(s) {
					if m.code == 2102 || m.code == 2103 || m.code == 2105 || m.code == 2111 || m.code == 2184 {
						result = append(result, m)
					}
				}
				return result
			}
			got, want := selectErrors(out.String()), selectErrors(string(java))
			if fmt.Sprint(got) != fmt.Sprint(want) {
				t.Fatalf("Go: %+v\nJava: %+v\nGo output: %s\nJava output: %s", got, want, out.String(), java)
			}
		})
	}
}
