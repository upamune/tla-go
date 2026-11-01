package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// Java TLC 1.7.4 constructs a tableau even when it ignores a constant FALSE
// particle. Its promises and node order influence the counterexample trace.
func TestJavaTLCConstantTableauTraces(t *testing.T) {
	for _, tc := range []struct {
		name, init, next, property, fairness string
		dump                                 bool
	}{
		{"eventually true", `x = 0`, `x' = (x+1)%3`, `<>TRUE`, "", false},
		{"not eventually false", `x = 0`, `x' = (x+1)%3`, `~<>FALSE`, "", false},
		{"nested always true", `x = 0`, `x' = (x+1)%3`, `[][]TRUE`, "", false},
		{"nested recurrence cycle", `x = 0`, `x' = (x+1)%3`, `[]<>[]TRUE`, "", false},
		{"nested recurrence dot", `x = 0`, `x' = (x+1)%3`, `[]<>[]TRUE`, "", true},
		{"nested stability cycle", `x = 0`, `x' = (x+1)%3`, `<>[]<>TRUE`, "", false},
		{"two initials", `x \in {0, 1}`, `x' = (x+1)%3`, `[]<>[]TRUE`, "", false},
		{"self loop", `x = 0`, `x' = x`, `[]<>[]TRUE`, "", false},
		{"chain", `x = 0`, `x' = IF x < 2 THEN x+1 ELSE x`, `[]<>[]TRUE`, "", false},
		{"branch", `x = 0`, `x' \in {0, 1, 2}`, `[]<>[]TRUE`, "", false},
		{"weak fairness", `x = 0`, `x' = (x+1)%3`, `[]<>[]TRUE`, `WF_x(Next)`, false},
		{"strong fairness", `x = 0`, `x' = (x+1)%3`, `[]<>[]TRUE`, `SF_x(Next)`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == " + tc.init + "\nNext == " + tc.next + "\nP == " + tc.property + "\n"
			cfg := "INIT Init\nNEXT Next\nPROPERTY P\nCHECK_DEADLOCK FALSE\n"
			if tc.fairness != "" {
				body += "Spec == Init /\\ [][Next]_x /\\ " + tc.fairness + "\n"
				cfg = "SPECIFICATION Spec\nPROPERTY P\nCHECK_DEADLOCK FALSE\n"
			}
			writeDOTFixture(t, dir, body, cfg)
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final"}
			if tc.dump {
				args = append(args, "-dump", "dot", filepath.Join(dir, "out.dot"))
			}
			args = append(args, "M.tla")
			var out, stderr bytes.Buffer
			goArgs := append(append([]string{}, args[:len(args)-1]...), filepath.Join(dir, "M.tla"))
			code := run(context.Background(), goArgs, &out, &stderr)
			if code != 13 || stderr.Len() != 0 {
				t.Fatalf("Go exit %d: %s %s", code, &out, &stderr)
			}
			if tc.name == "nested recurrence cycle" {
				for _, fragment := range []string{"1: <Initial predicate>\nx = 0", "x = 1", "x = 2", "1: Back to state:"} {
					if !strings.Contains(out.String(), fragment) {
						t.Fatalf("Go counterexample missing %q: %s", fragment, &out)
					}
				}
			}
			compareJavaTraceStates(t, dir, args, readToolMessages(out.String()), code)
		})
	}
}
