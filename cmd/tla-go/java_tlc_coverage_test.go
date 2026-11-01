package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The option parser and integer overflow behavior are from TLC.java at
// TLA+ Tools commit 5a47802b5c391f59ecdd44117981f4ff8c0656ba (1.7.4).
// Set TLC_JAR to that version to compare these errors against Java directly.
func TestJavaTLCCoverageOptionErrors(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"M.tla", "-coverage"}, "Error: coverage report interval required."},
		{[]string{"-coverage", "bad", "M.tla"}, "Error: An integer for coverage report interval required. But encountered bad"},
		{[]string{"-coverage", "2147483648", "M.tla"}, "Error: An integer for coverage report interval required. But encountered 2147483648"},
		{[]string{"-coverage", "-1", "M.tla"}, "Error: expect a nonnegative integer for -coverage option."},
		{[]string{"-coverage", "35792", "M.tla"}, "Error: expect a nonnegative integer for -coverage option."},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			dir := t.TempDir()
			out, code, stderr := runGoInDir(t, dir, tc.args)
			if code != 1 || stderr != "" || out != printedOptionError(tc.want) {
				t.Fatalf("exit %d; stdout %q; stderr %q", code, out, stderr)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, tc.args)
				if javaCode != code || !strings.Contains(java, "Error: "+tc.want+"\n") {
					t.Fatalf("Java exit %d:\n%s", javaCode, java)
				}
			}
		})
	}
}

func coverageReport(t *testing.T, output string) string {
	t.Helper()
	start := strings.Index(output, "<Init ")
	end := strings.Index(output, "End of statistics.")
	if start < 0 || end < start {
		t.Fatalf("missing coverage report:\n%s", output)
	}
	return output[start:end+len("End of statistics.")] + "\n"
}

// The fixture and its golden report come from Java TLC 1.7.4 at the commit
// recorded in the fixture README. With TLC_JAR set, both the original model
// and a changed constraint are compared directly to the Java oracle.
func TestJavaTLCUpstreamCoverageStatistics(t *testing.T) {
	for _, limit := range []string{"20", "7"} {
		t.Run(limit, func(t *testing.T) {
			golden, err := os.ReadFile(filepath.Join("testdata", "java-tlc", "upstream-coverage", "CoverageStatistics.coverage"))
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			for _, ext := range []string{".tla", ".cfg"} {
				data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", "upstream-coverage", "CoverageStatistics"+ext))
				if err != nil {
					t.Fatal(err)
				}
				if ext == ".tla" {
					data = []byte(strings.Replace(string(data), "Constraint == x < 20", "Constraint == x < "+limit, 1))
				}
				if err := os.WriteFile(filepath.Join(dir, "CoverageStatistics"+ext), data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"-workers", "1", "-coverage", "0", "CoverageStatistics.tla"}
			out, code, stderr := runGoInDir(t, dir, args)
			if code != 0 || stderr != "" {
				t.Fatalf("exit %d, stderr %s, output %s", code, stderr, out)
			}
			got := coverageReport(t, out)
			if limit == "7" && !strings.Contains(got, "<A line 15, col 1 to line 15, col 1 of module CoverageStatistics>: 3:6\n") {
				t.Fatalf("changed constraint did not change action counts:\n%s", got)
			}
			if limit == "20" {
				if got != string(golden) {
					t.Fatalf("coverage mismatch\ngot:\n%s\nwant:\n%s", got, golden)
				}
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java, code := runJavaTLCArgs(t, jar, dir, args)
				if code != 0 {
					t.Fatalf("Java exit %d:\n%s", code, java)
				}
				if want := coverageReport(t, java); got != want {
					t.Fatalf("coverage differs from Java\ngot:\n%s\nwant:\n%s", got, want)
				}
			}
		})
	}
}

func TestJavaTLCCoverageCostVariants(t *testing.T) {
	for _, tc := range []struct{ name, old, replacement string }{
		{"guard", `c \in Nat`, `x < 2`},
		{"deterministic-init", `x \in 1..3`, `x = 1`},
		{"equality", `c \in Nat`, `x = 1`},
		{"equality-two", `c \in Nat`, `x = 2`},
		{"equality-never", `c \in Nat`, `x = 0`},
		{"unreached", `c \in Nat`, `x < 0`},
		{"less-three", `c \in Nat`, `x < 3`},
		{"always-less", `c \in Nat`, `x < 4`},
		{"always-equal", `c \in Nat`, `y = 0`},
		{"second-equality", `y \in Nat`, `x = 2`},
		{"second-never", `y \in Nat`, `x < 0`},
		{"action-members", `a = x + 1`, `a \in 0..x`},
		{"action-empty", `a = x + 1`, `a \in 1..0`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			golden, err := os.ReadFile(filepath.Join("testdata", "java-tlc", "upstream-coverage", "CoverageStatistics-"+tc.name+".coverage"))
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			for _, ext := range []string{".tla", ".cfg"} {
				data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", "upstream-coverage", "CoverageStatistics"+ext))
				if err != nil {
					t.Fatal(err)
				}
				if ext == ".tla" {
					data = []byte(strings.Replace(string(data), tc.old, tc.replacement, 1))
				}
				if err := os.WriteFile(filepath.Join(dir, "CoverageStatistics"+ext), data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"-workers", "1", "-coverage", "0", "CoverageStatistics.tla"}
			out, code, stderr := runGoInDir(t, dir, args)
			if code != 0 || stderr != "" {
				t.Fatalf("exit %d, stderr %s, output %s", code, stderr, out)
			}
			got := coverageReport(t, out)
			if got != string(golden) {
				t.Fatalf("coverage mismatch\ngot:\n%s\nwant:\n%s", got, golden)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java, code := runJavaTLCArgs(t, jar, dir, args)
				if code != 0 {
					t.Fatalf("Java exit %d:\n%s", code, java)
				}
				if want := coverageReport(t, java); got != want {
					t.Fatalf("coverage differs from Java\ngot:\n%s\nwant:\n%s", got, want)
				}
			}
		})
	}
}

func TestJavaTLCCoverageInitialMemberships(t *testing.T) {
	for _, tc := range []struct{ name, x, y string }{
		{"members-3x3", "1..3", "1..3"},
		{"members-2x4", "1..2", "1..4"},
		{"members-dependent", "1..4", "1..x"},
		{"members-dependent-empty", "0..3", "1..x"},
		{"members-empty-first", "1..0", "1..3"},
		{"members-empty-last", "1..3", "1..0"},
		{"members-singleton", "1..1", "1..1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			base := filepath.Join("testdata", "java-tlc", "upstream-coverage")
			golden, err := os.ReadFile(filepath.Join(base, "CoverageStatistics-"+tc.name+".coverage"))
			if err != nil {
				t.Fatal(err)
			}
			for _, ext := range []string{".tla", ".cfg"} {
				data, err := os.ReadFile(filepath.Join(base, "CoverageStatistics"+ext))
				if err != nil {
					t.Fatal(err)
				}
				if ext == ".tla" {
					data = []byte(strings.Replace(string(data), `x \in 1..3`, `x \in `+tc.x, 1))
					data = []byte(strings.Replace(string(data), `y = 0`, `y \in `+tc.y, 1))
				}
				if err := os.WriteFile(filepath.Join(dir, "CoverageStatistics"+ext), data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"-workers", "1", "-coverage", "0", "CoverageStatistics.tla"}
			out, code, stderr := runGoInDir(t, dir, args)
			if code != 0 || stderr != "" {
				t.Fatalf("exit %d, stderr %s, output %s", code, stderr, out)
			}
			got := coverageReport(t, out)
			if got != string(golden) {
				t.Fatalf("coverage mismatch\ngot:\n%s\nwant:\n%s", got, golden)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				if javaCode != 0 {
					t.Fatalf("Java exit %d:\n%s", javaCode, java)
				}
				if want := coverageReport(t, java); got != want {
					t.Fatalf("coverage differs from Java\ngot:\n%s\nwant:\n%s", got, want)
				}
			}
		})
	}
}

func TestJavaTLCCoverageThreeInitialMemberships(t *testing.T) {
	for _, tc := range []struct{ name, domain string }{
		{"three-independent", "1..2"},
		{"three-dependent", "1..y"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			golden, err := os.ReadFile(filepath.Join("testdata", "java-tlc", "upstream-coverage", tc.name+".coverage"))
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			model := `---- MODULE M ----
EXTENDS Naturals
VARIABLES x,y,z
Init == /\ x \in 1..2
        /\ y \in 1..3
        /\ z \in DOMAIN
A == UNCHANGED <<x,y,z>>
B == UNCHANGED <<x,y,z>>
Next == A \/ B
====
`
			if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(strings.Replace(model, "DOMAIN", tc.domain, 1)), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			args := []string{"-workers", "1", "-coverage", "0", "M.tla"}
			out, code, stderr := runGoInDir(t, dir, args)
			if code != 0 || stderr != "" {
				t.Fatalf("exit %d, stderr %s, output %s", code, stderr, out)
			}
			got := coverageReport(t, out)
			if got != string(golden) {
				t.Fatalf("coverage mismatch\ngot:\n%s\nwant:\n%s", got, golden)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				if javaCode != 0 {
					t.Fatalf("Java exit %d:\n%s", javaCode, java)
				}
				if want := coverageReport(t, java); got != want {
					t.Fatalf("coverage differs from Java\ngot:\n%s\nwant:\n%s", got, want)
				}
			}
		})
	}
}

func TestCoverageUnsupportedModes(t *testing.T) {
	for _, args := range [][]string{{"-coverage", "1", "M.tla"}, {"-coverage", "0", "-simulate", "M.tla"}, {"-coverage", "0", "-dfid", "3", "M.tla"}, {"-coverage", "0", "-continue", "M.tla"}} {
		if _, err := parseOptions(args); err == nil {
			t.Errorf("accepted unsupported options %v", args)
		}
	}
}

func TestCoverageRejectsUnsupportedModel(t *testing.T) {
	dir := t.TempDir()
	golden, err := os.ReadFile(filepath.Join("testdata", "java-tlc", "upstream-coverage", "C.coverage"))
	if err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{".tla", ".cfg"} {
		data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", "upstream-coverage", "C"+ext))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "C"+ext), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	_, code, stderr := runGoInDir(t, dir, []string{"-coverage", "0", "C.tla"})
	if code == 0 || !strings.Contains(stderr, "-coverage does not yet support") {
		t.Fatalf("exit %d, stderr %s", code, stderr)
	}
	// Keep a complete oracle for the quantified action and invariants while
	// their cost nodes are not yet implemented in the Go evaluator.
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java, javaCode := runJavaTLCArgs(t, jar, dir, []string{"-workers", "1", "-coverage", "0", "C.tla"})
		if javaCode != 0 {
			t.Fatalf("Java exit %d:\n%s", javaCode, java)
		}
		if got := coverageReport(t, java); got != string(golden) {
			t.Fatalf("Java C coverage differs from oracle\ngot:\n%s\nwant:\n%s", got, golden)
		}
	}
}

func TestCoverageRejectsUnimplementedExpressionCosts(t *testing.T) {
	dir := t.TempDir()
	for _, ext := range []string{".tla", ".cfg"} {
		data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", "upstream-coverage", "CoverageStatistics"+ext))
		if err != nil {
			t.Fatal(err)
		}
		if ext == ".tla" {
			data = []byte(strings.Replace(string(data), "c \\in Nat", "c + 1 \\in Nat", 1))
		}
		if err := os.WriteFile(filepath.Join(dir, "CoverageStatistics"+ext), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	_, code, stderr := runGoInDir(t, dir, []string{"-coverage", "0", "CoverageStatistics.tla"})
	if code == 0 || !strings.Contains(stderr, "-coverage does not yet support") {
		t.Fatalf("exit %d, stderr %s", code, stderr)
	}
}

// Compare the complete coverage message sequence, including codes and bodies.
func TestJavaTLCToolCoverage(t *testing.T) {
	dir := t.TempDir()
	for _, ext := range []string{".tla", ".cfg"} {
		name := "CoverageStatistics" + ext
		data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", "upstream-coverage", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	args := []string{"-tool", "-workers", "1", "-coverage", "0", "CoverageStatistics.tla"}
	out, code, stderr := runGoInDir(t, dir, args)
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %s, output %s", code, stderr, out)
	}
	got := coverageMessages(readToolMessages(out))
	if len(got) < 3 || got[0].code != 2201 || got[len(got)-1].code != 2202 {
		t.Fatalf("invalid coverage messages: %v", got)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java, code := runJavaTLCArgs(t, jar, dir, args)
		if code != 0 {
			t.Fatalf("Java exit %d: %s", code, java)
		}
		want := coverageMessages(readToolMessages(java))
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("coverage messages differ\ngot: %#v\nwant: %#v", got, want)
		}
	}
}

func coverageMessages(messages []toolMessage) []toolMessage {
	var result []toolMessage
	for _, m := range messages {
		switch m.code {
		case 2201:
			m.body = "The coverage statistics at <time>"
			result = append(result, m)
		case 2202, 2221, 2772, 2773:
			result = append(result, m)
		}
	}
	return result
}

func TestJavaTLCCoverageInitialCandidateGuards(t *testing.T) {
	for _, tc := range []struct{ name, init, counts string }{
		{"rejected", `/\ x = 0 /\ y = 0 /\ x < 0`, "0:1"},
		{"accepted", `/\ x = 0 /\ y = 0 /\ x < 1`, "1:1"},
		{"early-rejected", `/\ x = 0 /\ x < 0 /\ y = 0`, "0:0"},
		{"early-accepted", `/\ x = 0 /\ x < 1 /\ y = 0`, "1:1"},
		{"ranges-rejected", `/\ x \in 1..2 /\ y \in 1..x /\ y < 0`, "0:3"},
		{"ranges-mixed", `/\ x \in 1..2 /\ y \in 1..x /\ y < 2`, "2:3"},
		{"ranges-early-guard", `/\ x \in 1..2 /\ x < 2 /\ y \in 1..x`, "1:1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			model := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLES x,y\nInit == " + strings.ReplaceAll(tc.init, ` /\ `, "\n        /\\ ") + "\nA == /\\ x' = x\n     /\\ y' = y\nB == /\\ x' = x\n     /\\ y' = y\nNext == A \\/ B\n====\n"
			if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(model), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			args := []string{"-workers", "1", "-coverage", "0", "M.tla"}
			out, code, stderr := runGoInDir(t, dir, args)
			if code != 0 || stderr != "" {
				t.Fatalf("exit %d, stderr %s, output %s", code, stderr, out)
			}
			got := coverageReport(t, out)
			if !strings.Contains(strings.Split(got, "\n")[0], ": "+tc.counts) {
				t.Errorf("unexpected initial counts:\n%s", got)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				if javaCode != 0 {
					t.Fatalf("Java exit %d:\n%s", javaCode, java)
				}
				if want := coverageReport(t, java); got != want {
					t.Errorf("coverage differs from Java\ngot:\n%s\nwant:\n%s", got, want)
				}
			}
		})
	}
}
