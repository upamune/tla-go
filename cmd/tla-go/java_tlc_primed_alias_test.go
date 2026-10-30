package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Coverage/C is an upstream model that assigns x through a == x'.
func TestJavaTLCPrimedOperatorAssignment(t *testing.T) {
	for _, tc := range []struct{ name, definitions, assignment string }{
		{"operator", "a == x'", "a = x + 1"},
		{"nested operators", "a == x'\nb == a", "b = x + 1"},
		{"membership", "a == x'", "a \\in {x + 1}"},
		{"primed state alias", "a == x", "a' = x + 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\n" + tc.definitions + "\nInit == x = 0\nNext == " + tc.assignment + "\nConstraint == x < 3"
			writeDOTFixture(t, dir, body, "INIT Init NEXT Next CONSTRAINT Constraint CHECK_DEADLOCK FALSE")
			checkSymmetryRecurrence(t, dir, "M", 0, "4 states generated, 3 distinct states found, 0 states left on queue")
		})
	}
}

func TestJavaTLCUpstreamCoveragePrimedAliases(t *testing.T) {
	for _, tc := range []struct{ name, counts string }{
		{"C", "253 states generated, 20 distinct states found, 0 states left on queue"},
		{"CoverageStatistics", "98 states generated, 19 distinct states found, 0 states left on queue"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, extension := range []string{".tla", ".cfg"} {
				name := tc.name + extension
				data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", "upstream-coverage", name))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			checkSymmetryRecurrence(t, dir, tc.name, 0, tc.counts)
		})
	}
}
