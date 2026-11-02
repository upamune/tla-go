package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Initial exceptions cause TLC 1.7.4 to rerun initialization with its
// CallStackTool. Coverage records both runs; state statistics do not double.
func TestJavaTLCCoverageInitialErrorReplay(t *testing.T) {
	for _, tc := range []struct {
		name, init, extra, cfg string
	}{
		{"invariant-first", "x = TRUE", "Inv == x < 2", "INVARIANT Inv\n"},
		{"invariant-second", `x \in 0..2`, `Inv == x = 0 \/ x < Nat`, "INVARIANT Inv\n"},
		{"constraint-second", `x \in 0..1`, `Limit == IF x = 0 THEN TRUE ELSE x < TRUE`, "CONSTRAINT Limit\n"},
		{"constraint-print", `x \in 0..1`, `Limit == IF PrintT(x) THEN IF x = 0 THEN TRUE ELSE x < TRUE ELSE FALSE`, "CONSTRAINT Limit\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "EXTENDS Naturals, TLC\nVARIABLE x\nInit == " + tc.init + "\nA == UNCHANGED x\nB == UNCHANGED x\nNext == A \\/ B\n" + tc.extra
			writeCoverageModule(t, dir, "M", body)
			cfg := "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n" + tc.cfg
			if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte(cfg), 0o600); err != nil {
				t.Fatal(err)
			}
			args := []string{"-tool", "-fp", "0", "-workers", "1", "-checkpoint", "0", "-coverage", "9999", "M.tla"}
			out, code, stderr := runGoInDir(t, dir, args)
			if code != 75 || stderr != "" {
				t.Fatalf("Go exit %d stderr %s\n%s", code, stderr, out)
			}
			if jar := pinnedFairnessEffectJar(t); jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				got, want := coverageMessages(readToolMessages(out)), coverageMessages(readToolMessages(java))
				if javaCode != code || !reflect.DeepEqual(got, want) || rawToolMessage(out, 2199) != rawToolMessage(java, 2199) || rawToolMessage(out, 2103) != rawToolMessage(java, 2103) || strings.Count(out, "@!@!@STARTMSG 2103:") != strings.Count(java, "@!@!@STARTMSG 2103:") || tc.name == "constraint-print" && !reflect.DeepEqual(initialErrorEventOrder(out), initialErrorEventOrder(java)) {
					t.Fatalf("Java exit %d Go exit %d\nJava: %#v\nGo: %#v\nJava output: %s\nGo output: %s", javaCode, code, want, got, java, out)
				}
			}
		})
	}
}

func initialErrorEventOrder(output string) []string {
	var events []string
	for line := range strings.SplitSeq(output, "\n") {
		if line == "0" || line == "1" || strings.HasPrefix(line, "@!@!@STARTMSG 1000:") || strings.HasPrefix(line, "@!@!@STARTMSG 2103:") {
			events = append(events, line)
		}
	}
	return events
}
