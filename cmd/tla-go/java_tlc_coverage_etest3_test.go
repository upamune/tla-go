package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// etest3 is the upstream suite fixture at tlaplus/tlaplus 5a47802b. Java
// replays Init while reconstructing the two-state deadlock trace, including
// Print and the coverage counters. Go must not claim a complete report yet.
func TestJavaTLCUpstreamETest3CoverageBoundary(t *testing.T) {
	dir := t.TempDir()
	for _, ext := range []string{".tla", ".cfg"} {
		data, err := os.ReadFile(filepath.Join(coverageAFixtures(t), "etest3"+ext))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "etest3"+ext), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	args := []string{"-tool", "-fp", "0", "-workers", "1", "-coverage", "1", "-checkpoint", "0", "etest3.tla"}
	out, code, stderr := runGoInDir(t, dir, args)
	if code == 0 || !strings.Contains(out+stderr, "-coverage does not yet support") {
		t.Fatalf("Go coverage should remain rejected: %d %s %s", code, stderr, out)
	}
	out, code, stderr = runGoInDir(t, dir, []string{"-tool", "-fp", "0", "-workers", "1", "-checkpoint", "0", "etest3.tla"})
	if code != 11 || stderr != "" || !hasToolMessage(readToolMessages(out), 2114) || !strings.Contains(out, "2 states generated, 2 distinct states found, 0 states left on queue") {
		t.Fatalf("Go deadlock: exit %d stderr %s\n%s", code, stderr, out)
	}
	if jar := pinnedFairnessEffectJar(t); jar != "" {
		java, exit := runJavaTLCArgs(t, jar, dir, args)
		want := []string{
			"<Init line 9, col 1 to line 9, col 4 of module etest3>: 2:2",
			"line 9, col 9 to line 12, col 16 of module etest3: 2",
			"<Next line 14, col 1 to line 14, col 4 of module etest3>: 1:2",
			"line 14, col 12 to line 14, col 17 of module etest3: 5",
			"|line 14, col 12 to line 14, col 12 of module etest3: 3",
			"|line 14, col 16 to line 14, col 17 of module etest3: 3",
			"line 15, col 12 to line 15, col 19 of module etest3: 2",
			"line 16, col 12 to line 16, col 27 of module etest3: 2",
			"line 17, col 12 to line 17, col 17 of module etest3: 2",
			"|line 17, col 17 to line 17, col 17 of module etest3: 4",
			"line 18, col 12 to line 18, col 18 of module etest3: 4",
			"2 states generated, 2 distinct states found, 0 states left on queue",
		}
		messages := readToolMessages(java)
		if exit != 11 || !hasToolMessage(messages, 2114) || len(coverageMessages(messages)) != 13 || strings.Count(java, `"This test should deadlock after finding two states"  TRUE`) != 2 {
			t.Fatalf("Java deadlock: exit %d\n%s", exit, java)
		}
		for _, part := range want {
			if !strings.Contains(java, part) {
				t.Fatalf("Java missing %q\n%s", part, java)
			}
		}
	}
}
