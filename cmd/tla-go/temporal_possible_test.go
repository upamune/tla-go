package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTemporalDiagnosticsAfterCounterexample(t *testing.T) {
	for _, tc := range []struct {
		name, property, extra, config string
		code                          int
		possible, postPrint           bool
	}{
		{"unwitnessed", "x=4", "Never == x=5", "_POSSIBLE Never", 13, true, false},
		{"witnessed", "x=4", "Seen == x=1", "_POSSIBLE Seen", 13, false, false},
		{"false postcondition", "x=4", "Post == PrintT(12345) /\\ (TLCGet(\"distinct\") = 0)", "POSTCONDITION Post", 13, false, true},
		{"error postcondition", "x=4", "Post == PrintT(12345) /\\ (1 \\div 0 = 1)", "POSTCONDITION Post", 13, false, true},
		{"success", "x>=0", "Seen == x=1", "_POSSIBLE Seen", 0, false, false},
	} {
		for _, tool := range []bool{false, true} {
			name := tc.name
			if tool {
				name += "/tool"
			} else {
				name += "/human"
			}
			t.Run(name, func(t *testing.T) {
				dir := t.TempDir()
				body := "VARIABLE x\nInit == x=0\nNext == x'=(x+1)%3\nP == []<>(" + tc.property + ")\n" + tc.extra
				writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nPROPERTY P\n"+tc.config)
				args := []string{"-workers", "1", "-fp", "0", "-lncheck", "final"}
				if tool {
					args = append(args, "-tool")
				}
				var out, errout bytes.Buffer
				code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
				if code != tc.code || errout.Len() != 0 {
					t.Fatalf("exit %d stderr %s output %s", code, &errout, &out)
				}
				checkTemporalDiagnosticOrder(t, out.String(), tool, tc.code, tc.possible, tc.postPrint)
				jar := os.Getenv("TLC_MODERN_JAR")
				if jar == "" {
					return
				}
				cmd := exec.Command("java", append([]string{"-jar", jar}, append(args, "M.tla")...)...)
				cmd.Dir = dir
				java, err := cmd.CombinedOutput()
				if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != tc.code {
					t.Fatalf("Java: %v\n%s", err, java)
				}
				checkTemporalDiagnosticOrder(t, string(java), tool, tc.code, tc.possible, tc.postPrint)
			})
		}
	}
}

func checkTemporalDiagnosticOrder(t *testing.T, out string, tool bool, code int, possible, postPrint bool) {
	t.Helper()
	end := strings.Index(out, "Finished checking temporal properties")
	if end < 0 {
		t.Fatalf("missing temporal end: %s", out)
	}
	if code == 13 {
		headline := strings.Index(out, "Temporal property P was violated")
		if headline < 0 && tool {
			headline = strings.Index(out, "Temporal properties were violated")
		}
		trace := strings.Index(out, "counter-example")
		if headline < 0 || trace < headline || trace > end || strings.Count(out, "The following behavior constitutes a counter-example") != 1 {
			t.Fatalf("bad counterexample order: %s", out)
		}
		if possible {
			counts := strings.Index(out, "[Never |-> 0]")
			diagnostic := strings.Index(out, "The _POSSIBLE predicate Never")
			if counts < trace || diagnostic < counts || diagnostic > end || strings.Count(out, "The _POSSIBLE predicate Never") != 1 {
				t.Fatalf("bad possible order: %s", out)
			}
		} else if strings.Contains(out, "was never witnessed") {
			t.Fatalf("unexpected possible diagnostic: %s", out)
		}
		if postPrint {
			post := strings.LastIndex(out, "\n12345\n")
			if post < trace || post > end {
				t.Fatalf("bad postcondition order: %s", out)
			}
		}
	} else if strings.Contains(out, "counter-example") || strings.Contains(out, "was never witnessed") {
		t.Fatalf("unexpected diagnostic: %s", out)
	}
}
