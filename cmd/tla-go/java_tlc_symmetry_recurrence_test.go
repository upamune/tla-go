package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These models are from Java TLC 1.7.4's test-model/symmetry directory.
func TestJavaTLCSymmetryReducedRecurrence(t *testing.T) {
	for _, tc := range []struct {
		name, counts string
		code         int
	}{
		{"April20aMC", "5 states generated, 3 distinct states found, 0 states left on queue", 0},
		{"April21MC", "4 states generated, 1 distinct states found, 0 states left on queue", 0},
		{"April25MC", "4 states generated, 1 distinct states found, 0 states left on queue", 0},
		{"LongMC", "8 states generated, 5 distinct states found, 0 states left on queue", 0},
		{"LongMCa", "7 states generated, 5 distinct states found, 0 states left on queue", 13},
		{"MC", "5 states generated, 2 distinct states found, 0 states left on queue", 0},
		{"MCa", "4 states generated, 2 distinct states found, 0 states left on queue", 13},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := copySymmetryRecurrenceFixtures(t)
			checkSymmetryRecurrence(t, dir, tc.name, tc.code, tc.counts)
		})
	}
}

func TestJavaTLCSymmetryParallelRecurrenceTransitions(t *testing.T) {
	for _, tc := range []struct{ name, constants, assignment, initial, fairness, counts string }{
		{"two values", "P,a,b", "P={a,b} a=a b=b", "bit = 0", "", "6 states generated, 2 distinct states found, 0 states left on queue"},
		{"multiple initial states", "P,a,b", "P={a,b} a=a b=b", "bit \\in {0,1}", "", "8 states generated, 2 distinct states found, 0 states left on queue"},
		{"three values weak fairness", "P,a,b,c", "P={a,b,c} a=a b=b c=c", "bit = 0", " /\\ WF_vars(Next)", "9 states generated, 2 distinct states found, 0 states left on queue"},
		{"three values strong fairness", "P,a,b,c", "P={a,b,c} a=a b=b c=c", "bit = 0", " /\\ SF_vars(Next)", "9 states generated, 2 distinct states found, 0 states left on queue"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "CONSTANTS " + tc.constants + "\nVARIABLES x,bit\nvars == <<x,bit>>\n" +
				"Init == x \\in P /\\ " + tc.initial + "\nNext == x' \\in P /\\ bit' = 1-bit\n" +
				"Spec == Init /\\ [][Next]_vars" + tc.fairness + "\nSym == Permutations(P)\nProp == <>[] [x'=x]_x"
			cfg := "CONSTANTS " + tc.assignment + " SPECIFICATION Spec SYMMETRY Sym PROPERTY Prop CHECK_DEADLOCK FALSE"
			writeDOTFixture(t, dir, body, cfg)
			checkSymmetryRecurrence(t, dir, "M", 13, tc.counts)
		})
	}
}

func copySymmetryRecurrenceFixtures(t *testing.T) string {
	t.Helper()
	source := filepath.Join("testdata", "java-tlc", "symmetry-recurrence")
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(source, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, entry.Name()), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func checkSymmetryRecurrence(t *testing.T, dir, name string, want int, counts string) {
	t.Helper()
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-workers", "1", "-fp", "0", "-lncheck", "final", filepath.Join(dir, name+".tla")}, &out, &errout)
	if code != want || errout.Len() != 0 || !strings.Contains(out.String(), counts) {
		t.Fatalf("exit %d, want %d; stdout:\n%s\nstderr:\n%s", code, want, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		output := runJavaTLCForExit(t, jar, dir, name, want)
		if !strings.Contains(output, counts) {
			t.Fatalf("Java TLC output does not contain %q:\n%s", counts, output)
		}
	}
}
