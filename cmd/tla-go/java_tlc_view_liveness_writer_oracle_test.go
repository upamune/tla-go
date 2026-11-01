package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// This is a Java-only oracle, not a claim that tla-go reproduces the result.
// DotLivenessStateWriter performs extra VIEW evaluations in a separate file.
func TestJavaTLCViewLivenessWriterOracle(t *testing.T) {
	jar := maxSetOracleJar(t)
	if jar == "" {
		t.Skip("set TLC_JAR to the pinned Java TLC 1.7.4 jar")
	}
	for _, tc := range []struct {
		name   string
		dot    bool
		exit   int
		stats  string
		prefix []string
	}{
		{"no dump", false, 13, "10 states generated, 3 distinct states found, 0 states left on queue.", []string{`<<"constraint", 0, 0>>`, `<<"view", 0, 1>>`, `<<"next", 0, 1>>`}},
		{"DOT", true, 0, "4 states generated, 1 distinct states found, 0 states left on queue.", []string{`<<"constraint", 0, 0>>`, `<<"view", 0, 1>>`, `<<"view", 0, 2>>`, `<<"view", 0, 3>>`, `<<"view", 0, 4>>`, `<<"view", 0, 5>>`, `<<"next", 0, 5>>`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := `VARIABLE x
Init == x = 0 /\ TLCSet(0, 0)
Next == IF PrintT(<<"next", x, TLCGet(0)>>) THEN x' \in 0..2 ELSE FALSE
C == IF PrintT(<<"constraint", x, TLCGet(0)>>) THEN TLCGet(0) < 8 ELSE FALSE
V == IF TLCSet(0, TLCGet(0) + 1) THEN IF PrintT(<<"view", x, TLCGet(0)>>) THEN x ELSE x ELSE x
P == []<>(x = 0)`
			writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nVIEW V\nCONSTRAINT C\nPROPERTY P\nCHECK_DEADLOCK FALSE\n")
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final"}
			if tc.dot {
				args = append(args, "-dump", "dot", "out.dot")
			}
			out, code := runJavaTLCArgs(t, jar, dir, append(args, "M.tla"))
			var prints []string
			for line := range strings.SplitSeq(out, "\n") {
				if strings.HasPrefix(line, `<<"view",`) || strings.HasPrefix(line, `<<"constraint",`) || strings.HasPrefix(line, `<<"next",`) {
					prints = append(prints, line)
				}
			}
			if code != tc.exit || !strings.Contains(out, tc.stats) || len(prints) < len(tc.prefix) || !reflect.DeepEqual(prints[:len(tc.prefix)], tc.prefix) {
				t.Fatalf("Java exit %d, prints %q, output:\n%s", code, prints, out)
			}
			graph, err := os.ReadFile(filepath.Join(dir, "out_liveness.dot"))
			if tc.dot && (err != nil || !strings.Contains(string(graph), "digraph DiskGraph")) {
				t.Fatalf("liveness DOT %q: %v", graph, err)
			}
			if !tc.dot && !os.IsNotExist(err) {
				t.Fatalf("unexpected liveness graph: %v", err)
			}
		})
	}
}
