package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestJavaTLCUpstreamGCoverage(t *testing.T) {
	dir := t.TempDir()
	fixture, err := filepath.Abs(filepath.Join("testdata", "java-tlc", "upstream-coverage"))
	if err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{".tla", ".cfg"} {
		data, err := os.ReadFile(filepath.Join(fixture, "G"+ext))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "G"+ext), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	args := []string{"-deadlock", "-fp", "0", "-workers", "1", "-checkpoint", "0", "-coverage", "9999", "G.tla"}
	out, code, stderr := runGoInDir(t, dir, args)
	if code != 0 || stderr != "" {
		t.Fatalf("upstream G: exit %d stderr %s output %s", code, stderr, out)
	}
	golden, err := os.ReadFile(filepath.Join(fixture, "G.coverage"))
	if err != nil {
		t.Fatal(err)
	}
	if report := coverageReport(t, out); report != string(golden) {
		t.Fatalf("coverage report\ngot:\n%s\nwant:\n%s", report, golden)
	}
	if output, code, stderr := runGoInDir(t, dir, []string{"-deadlock", "-fp", "0", "-workers", "1", "G.tla"}); code != 0 || stderr != "" || !strings.Contains(output, "2 states generated, 1 distinct states found") {
		t.Fatalf("default mode: exit %d stderr %s output %s", code, stderr, output)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java, code := runJavaTLCArgs(t, jar, dir, args)
		if code != 0 || coverageReport(t, java) != string(golden) || !strings.Contains(java, "2 states generated, 1 distinct states found") {
			t.Fatalf("Java exit %d output %s", code, java)
		}
	}
}

// GCoverageTest's single Next action combines UNCHANGED conjuncts with an
// alias and an ENABLED invariant.
// AbstractCoverageTest uses -coverage 9999; ModelCheckerTestCase uses
// -deadlock -fp 0 -workers 1 -checkpoint 0.
func TestJavaTLCCoverageDirectUnchanged(t *testing.T) {
	for _, tc := range []struct {
		name, module, aux, cfg string
	}{
		{"scalar", "VARIABLE x\nInit == x = TRUE\nNext == UNCHANGED x", "", ""},
		{"max-integer", "VARIABLE x\nInit == x = 2147483647\nNext == UNCHANGED x", "", ""},
		{"range", "EXTENDS Naturals\nVARIABLE x\nInit == x \\in 0..2\nNext == UNCHANGED x", "", ""},
		{"set-empty", "VARIABLE x\nInit == x \\in {}\nNext == UNCHANGED x", "", ""},
		{"set-singleton", "VARIABLE x\nInit == x \\in {1}\nNext == UNCHANGED x", "", ""},
		{"set-numbers", "VARIABLE x\nInit == x \\in {1,2,3}\nNext == UNCHANGED x", "", ""},
		{"set-duplicates", "VARIABLE x\nInit == x \\in {2,1,2,3}\nNext == UNCHANGED x", "", ""},
		{"set-booleans", "VARIABLE x\nInit == x \\in {TRUE,FALSE,TRUE}\nNext == UNCHANGED x", "", ""},
		{"set-spec", "VARIABLE x\nInit == x \\in {1,2,3}\nNext == UNCHANGED x\nSpec == Init /\\ [][Next]_x", "", "SPECIFICATION Spec\n"},
		{"set-enabled", "VARIABLE x\nInit == x \\in {1,2,3}\nNext == /\\ UNCHANGED x\n        /\\ UNCHANGED x\nInv == ENABLED Next", "", "INIT Init\nNEXT Next\nINVARIANT Inv\n"},
		{"set-invariant", "EXTENDS Naturals\nVARIABLE x\nInit == x \\in {1,2,3}\nNext == UNCHANGED x\nInv == x < 4", "", "INIT Init\nNEXT Next\nINVARIANT Inv\n"},
		{"empty", "EXTENDS Naturals\nVARIABLE x\nInit == x \\in 1..0\nNext == UNCHANGED x", "", ""},
		{"tuple", "VARIABLES x,y\nInit == /\\ x=TRUE\n        /\\ y=FALSE\nNext == UNCHANGED <<y,x>>", "", ""},
		{"three-values", "VARIABLES x,y,z\nInit == /\\ x=1\n        /\\ y=0\n        /\\ z=TRUE\nNext == UNCHANGED <<z,y,x>>", "", ""},
		{"extended", "EXTENDS Aux\n", "VARIABLE x\nInit == x = TRUE\nNext == UNCHANGED x", ""},
		{"split-modules", "EXTENDS Aux\nInit == x = TRUE", "VARIABLE x\nNext == UNCHANGED x", ""},
		{"reordered", "VARIABLE x\nNext == UNCHANGED x\nInit == x = TRUE", "", ""},
		{"spec", "VARIABLE x\nInit == x = TRUE\nNext == UNCHANGED x\nSpec == Init /\\ [][Next]_x", "", "SPECIFICATION Spec\n"},
		{"invariant", "EXTENDS Naturals\nVARIABLE x\nInit == x \\in 0..2\nNext == UNCHANGED x\nInv == x < 3", "", "INIT Init\nNEXT Next\nINVARIANT Inv\n"},
		{"enabled-scalar", "VARIABLE x\nInit == x = TRUE\nNext == UNCHANGED x\nInv == ENABLED Next", "", "INIT Init\nNEXT Next\nINVARIANT Inv\n"},
		{"enabled-conjunction", "VARIABLE x\nInit == x = TRUE\nvars == <<x>>\nNext == /\\ UNCHANGED vars\n        /\\ UNCHANGED <<x>>\n        /\\ UNCHANGED x\nInv == ENABLED Next", "", "INIT Init\nNEXT Next\nINVARIANT Inv\n"},
		{"enabled-range", "EXTENDS Naturals\nVARIABLE x\nInit == x \\in 0..2\nNext == /\\ UNCHANGED x\n        /\\ UNCHANGED <<x>>\nInv == ENABLED Next", "", "INIT Init\nNEXT Next\nINVARIANT Inv\n"},
		{"enabled-empty", "EXTENDS Naturals\nVARIABLE x\nInit == x \\in 1..0\nNext == UNCHANGED x\nInv == ENABLED Next", "", "INIT Init\nNEXT Next\nINVARIANT Inv\n"},
		{"enabled-empty-conjunction", "EXTENDS Naturals\nVARIABLE x\nInit == x \\in 1..0\nNext == /\\ UNCHANGED x\n        /\\ UNCHANGED <<x>>\nInv == ENABLED Next", "", "INIT Init\nNEXT Next\nINVARIANT Inv\n"},
		{"enabled-tuple", "VARIABLES x,y\nInit == /\\ x = TRUE\n        /\\ y = FALSE\nvars == <<y,x>>\nNext == /\\ UNCHANGED vars\n        /\\ UNCHANGED <<x,y>>\nInv == ENABLED Next", "", "INIT Init\nNEXT Next\nINVARIANT Inv\n"},
		{"enabled-two-invariants", "VARIABLE x\nInit == x = TRUE\nNext == UNCHANGED x\nInv1 == ENABLED Next\nInv2 == ENABLED Next", "", "INIT Init\nNEXT Next\nINVARIANTS Inv1 Inv2\n"},
		{"enabled-spec", "VARIABLE x\nInit == x = TRUE\nNext == UNCHANGED x\nInv == ENABLED Next\nSpec == Init /\\ [][Next]_x", "", "SPECIFICATION Spec\nINVARIANT Inv\n"},
		{"conjunction-no-invariant", "VARIABLE x\nInit == x = TRUE\nNext == /\\ UNCHANGED x\n        /\\ UNCHANGED <<x>>", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeCoverageModule(t, dir, "M", tc.module)
			if tc.aux != "" {
				writeCoverageModule(t, dir, "Aux", tc.aux)
			}
			cfg := tc.cfg
			if cfg == "" {
				cfg = "INIT Init\nNEXT Next\n"
			}
			if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte(cfg), 0o600); err != nil {
				t.Fatal(err)
			}
			args := []string{"-deadlock", "-fp", "0", "-workers", "1", "-checkpoint", "0", "-coverage", "9999", "M.tla"}
			for _, tool := range []bool{false, true} {
				runArgs := append([]string{}, args...)
				if tool {
					runArgs = append([]string{"-tool"}, runArgs...)
				}
				out, code, stderr := runGoInDir(t, dir, runArgs)
				if code != 0 || stderr != "" {
					t.Fatalf("Go exit %d stderr %s\n%s", code, stderr, out)
				}
				if tc.name == "scalar" && !tool {
					const want = "<Init line 3, col 1 to line 3, col 4 of module M>: 1:1\n  line 3, col 9 to line 3, col 16 of module M: 1\n<Next line 4, col 1 to line 4, col 4 of module M>: 0:1\n  line 4, col 9 to line 4, col 19 of module M: 1\nEnd of statistics.\n"
					if got := coverageReport(t, out); got != want {
						t.Fatalf("coverage\n%s\nwant\n%s", got, want)
					}
				}
				if tc.name == "set-numbers" && !tool {
					const want = "<Init line 3, col 1 to line 3, col 4 of module M>: 3:3\n  line 3, col 9 to line 3, col 21 of module M: 3\n  |line 3, col 15 to line 3, col 21 of module M: 1:4\n<Next line 4, col 1 to line 4, col 4 of module M>: 0:3\n  line 4, col 9 to line 4, col 19 of module M: 3\nEnd of statistics.\n"
					if got := coverageReport(t, out); got != want {
						t.Fatalf("coverage\n%s\nwant\n%s", got, want)
					}
				}
				if jar := os.Getenv("TLC_JAR"); jar != "" {
					java, javaCode := runJavaTLCArgs(t, jar, dir, runArgs)
					if javaCode != code {
						t.Fatalf("Go exit %d Java exit %d\n%s", code, javaCode, java)
					}
					if tool {
						got, want := coverageMessages(readToolMessages(out)), coverageMessages(readToolMessages(java))
						if !reflect.DeepEqual(got, want) {
							t.Fatalf("tool coverage\nGo: %+v\nJava: %+v", got, want)
						}
						if strings.HasPrefix(tc.name, "set-") {
							for _, code := range []int{2199, 2194} {
								if got, want := rawToolMessage(out, code), rawToolMessage(java, code); got != want {
									t.Fatalf("tool message %d\nGo: %s\nJava: %s", code, got, want)
								}
							}
						}
					} else if got, want := coverageReport(t, out), coverageReport(t, java); got != want {
						t.Fatalf("coverage\nGo:\n%s\nJava:\n%s", got, want)
					}
				}
			}
			if out, code, stderr := runGoInDir(t, dir, []string{"-workers", "1", "-deadlock", "M.tla"}); code != 0 || stderr != "" || strings.Contains(out, "The coverage statistics at") {
				t.Fatalf("default mode: exit %d stderr %s output %s", code, stderr, out)
			}
		})
	}
}

func TestJavaTLCCoverageUnchangedAfterAssignments(t *testing.T) {
	for _, tc := range []struct{ name, module string }{
		{"one-variable", "VARIABLE x\nInit == x=0\nA == /\\ x'=0\n     /\\ UNCHANGED x\nB == UNCHANGED x\nNext == A \\/ B"},
		{"two-variables", "VARIABLES x,y\nInit == /\\ x=0\n        /\\ y=0\nA == /\\ x'=0\n     /\\ y'=0\n     /\\ UNCHANGED <<x,y>>\nB == UNCHANGED <<x,y>>\nNext == A \\/ B"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeCoverageModule(t, dir, "M", tc.module)
			if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			args := []string{"-tool", "-deadlock", "-fp", "0", "-workers", "1", "-coverage", "9999", "M.tla"}
			out, code, stderr := runGoInDir(t, dir, args)
			if code != 0 || stderr != "" {
				t.Fatalf("Go exit %d stderr %s\n%s", code, stderr, out)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				if got, want := coverageMessages(readToolMessages(out)), coverageMessages(readToolMessages(java)); javaCode != code || !reflect.DeepEqual(got, want) {
					t.Fatalf("tool coverage Go exit %d Java exit %d\nGo: %+v\nJava: %+v", code, javaCode, got, want)
				}
			}
		})
	}
}

// A deterministic assignment is charged only if its continuation generates a
// successor, whereas an evaluated arithmetic RHS has its own cost node.
func TestJavaTLCCoverageAssignmentContinuation(t *testing.T) {
	for _, tc := range []struct{ name, rhs, assignment, child string }{
		{"one-success", "0", "line 5, col 9 to line 5, col 14 of module M: 1", ""},
		{"all-rejected", "3", "line 5, col 9 to line 5, col 14 of module M: 0", ""},
		{"all-success", "x", "line 5, col 9 to line 5, col 14 of module M: 3", ""},
		{"all-success-expression", "x + 0", "line 5, col 9 to line 5, col 18 of module M: 3", ""},
		{"one-success-expression", "x + x", "line 5, col 9 to line 5, col 18 of module M: 1", "|line 5, col 14 to line 5, col 18 of module M: 3"},
		{"all-rejected-expression", "x + 1", "line 5, col 9 to line 5, col 18 of module M: 0", "|line 5, col 14 to line 5, col 18 of module M: 3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			module := "EXTENDS Naturals\nVARIABLE x\nInit == x \\in 0..2\nA == /\\ x' = " + tc.rhs + "\n     /\\ UNCHANGED x\nB == UNCHANGED x\nNext == A \\/ B"
			writeCoverageModule(t, dir, "M", module)
			if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			for _, tool := range []bool{false, true} {
				args := []string{"-deadlock", "-fp", "0", "-workers", "1", "-checkpoint", "0", "-coverage", "9999", "M.tla"}
				if tool {
					args = append([]string{"-tool"}, args...)
				}
				out, code, stderr := runGoInDir(t, dir, args)
				if code != 0 || stderr != "" || !strings.Contains(out, tc.assignment) || !strings.Contains(out, tc.child) {
					t.Fatalf("Go exit %d stderr %s\n%s", code, stderr, out)
				}
				if jar := os.Getenv("TLC_JAR"); jar != "" {
					java, javaCode := runJavaTLCArgs(t, jar, dir, args)
					if javaCode != code {
						t.Fatalf("Java exit %d Go exit %d\n%s", javaCode, code, java)
					}
					if tool {
						if got, want := coverageMessages(readToolMessages(out)), coverageMessages(readToolMessages(java)); !reflect.DeepEqual(got, want) {
							t.Fatalf("tool coverage\nGo: %+v\nJava: %+v", got, want)
						}
					} else if got, want := coverageReport(t, out), coverageReport(t, java); got != want {
						t.Fatalf("coverage\nGo:\n%s\nJava:\n%s", got, want)
					}
				}
			}
		})
	}
}

func TestJavaTLCCoverageAssignmentContinuationMultipleVariables(t *testing.T) {
	for _, tc := range []struct{ name, first, second string }{
		{"success", "x", "y"},
		{"first-rejected", "x + 1", "y"},
		{"second-rejected", "x", "y + 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			module := "EXTENDS Naturals\nVARIABLES x, y\nInit == /\\ x = 0\n        /\\ y = 0\nA == /\\ x' = " + tc.first + "\n     /\\ y' = " + tc.second + "\n     /\\ UNCHANGED <<x,y>>\nB == UNCHANGED <<x,y>>\nNext == A \\/ B"
			writeCoverageModule(t, dir, "M", module)
			if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			args := []string{"-tool", "-deadlock", "-fp", "0", "-workers", "1", "-checkpoint", "0", "-coverage", "9999", "M.tla"}
			out, code, stderr := runGoInDir(t, dir, args)
			if code != 0 || stderr != "" {
				t.Fatalf("Go exit %d stderr %s\n%s", code, stderr, out)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				if got, want := coverageMessages(readToolMessages(out)), coverageMessages(readToolMessages(java)); javaCode != code || !reflect.DeepEqual(got, want) {
					t.Fatalf("tool coverage Go exit %d Java exit %d\nGo: %+v\nJava: %+v", code, javaCode, got, want)
				}
			}
		})
	}
}

func TestJavaTLCCoverageAssignmentContinuationAliases(t *testing.T) {
	for _, tc := range []struct{ name, definitions, target, rhs, constants string }{
		{"constant-operator", "D == 1\n", "x'", "D", ""},
		{"state-operator", "D == x + 1\n", "x'", "D", ""},
		{"variable-operator", "D == x\n", "x'", "D", ""},
		{"nested-operator", "E == x + 1\nD == E\n", "x'", "D", ""},
		{"primed-operator", "P == x'\nD == 1\n", "P", "D", ""},
		{"configured-constant", "CONSTANT N\n", "x'", "N", "CONSTANT N = 1\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			module := "EXTENDS Naturals\nVARIABLE x\nInit == x \\in 0..2\n" + tc.definitions + "A == /\\ " + tc.target + " = " + tc.rhs + "\n     /\\ UNCHANGED x\nB == UNCHANGED x\nNext == A \\/ B"
			writeCoverageModule(t, dir, "M", module)
			if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte(tc.constants+"INIT Init\nNEXT Next\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			args := []string{"-tool", "-deadlock", "-fp", "0", "-workers", "1", "-checkpoint", "0", "-coverage", "9999", "M.tla"}
			out, code, stderr := runGoInDir(t, dir, args)
			if code != 0 || stderr != "" {
				t.Fatalf("Go exit %d stderr %s\n%s", code, stderr, out)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				if got, want := coverageMessages(readToolMessages(out)), coverageMessages(readToolMessages(java)); javaCode != code || !reflect.DeepEqual(got, want) {
					t.Fatalf("tool coverage Go exit %d Java exit %d\nGo: %+v\nJava: %+v", code, javaCode, got, want)
				}
			}
		})
	}
}

func TestJavaTLCCoverageAssignmentContinuationBoolean(t *testing.T) {
	dir := t.TempDir()
	module := "VARIABLE x\nInit == x = TRUE\nA == /\\ x' = FALSE\n     /\\ UNCHANGED x\nB == x' = FALSE\nNext == A \\/ B"
	writeCoverageModule(t, dir, "M", module)
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"-tool", "-deadlock", "-fp", "0", "-workers", "1", "-checkpoint", "0", "-coverage", "9999", "M.tla"}
	out, code, stderr := runGoInDir(t, dir, args)
	if code != 0 || stderr != "" || !strings.Contains(out, "|line 4, col 14 to line 4, col 18 of module M: 2") {
		t.Fatalf("Go exit %d stderr %s\n%s", code, stderr, out)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java, javaCode := runJavaTLCArgs(t, jar, dir, args)
		if got, want := coverageMessages(readToolMessages(out)), coverageMessages(readToolMessages(java)); javaCode != code || !reflect.DeepEqual(got, want) {
			t.Fatalf("tool coverage Go exit %d Java exit %d\nGo: %+v\nJava: %+v", code, javaCode, got, want)
		}
	}
}

// TLC counts initial states before the constraint filters them, whereas the
// current collector counts only accepted initial states.
func TestCoverageRejectsDirectUnchangedWithConstraint(t *testing.T) {
	dir := t.TempDir()
	writeCoverageModule(t, dir, "M", "EXTENDS Naturals\nVARIABLE x\nInit == x \\in 0..2\nNext == UNCHANGED x\nLimit == x < 2")
	if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\nCONSTRAINT Limit\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, code, stderr := runGoInDir(t, dir, []string{"-coverage", "9999", "M.tla"})
	if code == 0 || !strings.Contains(stderr, "-coverage does not yet support") {
		t.Fatalf("exit %d stderr %s", code, stderr)
	}
}

func TestCoverageRejectsUnmodeledInitialSetCosts(t *testing.T) {
	for _, tc := range []struct{ name, body, cfg string }{
		{"guard", "EXTENDS Naturals\nVARIABLE x\nInit == /\\ x \\in {1,2,3}\n        /\\ x < 3\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n"},
		{"multiple variables", "VARIABLES x,y\nInit == /\\ x \\in {1,2}\n        /\\ y = 0\nNext == UNCHANGED <<x,y>>", "INIT Init\nNEXT Next\n"},
		{"constraint", "EXTENDS Naturals\nVARIABLE x\nInit == x \\in {1,2,3}\nA == x' = 1\nB == x' = 2\nNext == A \\/ B\nLimit == x < 3", "INIT Init\nNEXT Next\nCONSTRAINT Limit\n"},
		{"mixed kinds", "VARIABLE x\nInit == x \\in {1,TRUE}\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n"},
		{"nonliteral", "EXTENDS Naturals\nVARIABLE x\nInit == x \\in {1,1+1}\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n"},
		{"set assignment", "VARIABLE x\nInit == x = {1,2}\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n"},
		{"set initial and false unchanged", `VARIABLE x
Init == x \in {0,1,2}
A == /\ x' = 0
     /\ UNCHANGED x
B == UNCHANGED x
Next == A \/ B`, "INIT Init\nNEXT Next\n"},
		{"set initial and membership then unchanged", `EXTENDS Naturals
VARIABLE x
Init == x \in {0,1,2}
A == /\ x' \in 0..2
     /\ UNCHANGED <<x>>
B == UNCHANGED x
Next == A \/ B`, "INIT Init\nNEXT Next\n"},
		{"set alias in unchanged", `EXTENDS Naturals
VARIABLE x
S == {1,2,3}
Init == x \in 0..2
A == /\ x' = 0
     /\ UNCHANGED x
     /\ UNCHANGED S
B == UNCHANGED x
Next == A \/ B`, "INIT Init\nNEXT Next\n"},
		{"indirect set alias in unchanged", `EXTENDS Naturals
VARIABLE x
S == {1,2,3}
T == S
Init == x \in 0..2
A == /\ x' = 0
     /\ UNCHANGED x
     /\ UNCHANGED T
B == UNCHANGED x
Next == A \/ B`, "INIT Init\nNEXT Next\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeCoverageModule(t, dir, "M", tc.body)
			if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte(tc.cfg), 0o600); err != nil {
				t.Fatal(err)
			}
			out, code, stderr := runGoInDir(t, dir, []string{"-coverage", "9999", "M.tla"})
			if code == 0 || !strings.Contains(out+stderr, "-coverage does not yet support") {
				t.Fatalf("expected rejection: exit %d stderr %s output %s", code, stderr, out)
			}
			if strings.Contains(tc.name, "set alias in unchanged") || strings.Contains(tc.name, "false unchanged") {
				out, code, stderr = runGoInDir(t, dir, []string{"M.tla"})
				if code != 0 || !strings.Contains(out, "7 states generated, 3 distinct states found") {
					t.Fatalf("default mode: exit %d stderr %s output %s", code, stderr, out)
				}
			}
		})
	}
}

func TestJavaTLCCoverageInitialSetDisjunction(t *testing.T) {
	for _, initial := range []string{"{}", "{1}", "{1,2,2,3}"} {
		t.Run(initial, func(t *testing.T) {
			dir := t.TempDir()
			writeCoverageModule(t, dir, "M", "VARIABLE x\nInit == x \\in "+initial+"\nA == x' = 1\nB == x' = 2\nNext == A \\/ B")
			writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\n")
			compareParameterizedCoverage(t, dir)
		})
	}
}

func TestJavaTLCCoverageInitialSetBulletDisjunction(t *testing.T) {
	for _, cfg := range []string{"INIT Init\nNEXT Next\n", "SPECIFICATION Spec\n"} {
		t.Run(strings.Fields(cfg)[0], func(t *testing.T) {
			dir := t.TempDir()
			writeCoverageModule(t, dir, "M", "VARIABLE x\nInit == /\\ x \\in {1,2,3}\nA == x' = 1\nB == x' = 2\nNext == A \\/ B\nSpec == Init /\\ [][Next]_x")
			writeParameterizedConfig(t, dir, cfg)
			compareParameterizedCoverage(t, dir)
		})
	}
}

func TestCoverageRejectsInitialSetFalseContinuation(t *testing.T) {
	for _, tc := range []struct{ name, initial, cfg string }{
		{"direct", `Init == x \in {0,1,2}`, "INIT Init\nNEXT Next\n"},
		{"bullet", `Init == /\ x \in {0,1,2}`, "INIT Init\nNEXT Next\n"},
		{"parenthesized", `Init == (x \in {0,1,2})`, "INIT Init\nNEXT Next\n"},
		{"bullet parentheses", `Init == /\ (x \in {0,1,2})`, "INIT Init\nNEXT Next\n"},
		{"nested bullet", `Init == /\ (/\ x \in {0,1,2})`, "INIT Init\nNEXT Next\n"},
		{"spec bullet", `Init == /\ x \in {0,1,2}`, "SPECIFICATION Spec\n"},
		{"initial alias", "P == x \\in {0,1,2}\nInit == P", "INIT Init\nNEXT Next\n"},
		{"nested initial alias", "P == x \\in {0,1,2}\nQ == P\nInit == Q", "INIT Init\nNEXT Next\n"},
		{"set alias", "S == {0,1,2}\nInit == x \\in S", "INIT Init\nNEXT Next\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\n" + tc.initial + "\nA == /\\ x' = 0\n     /\\ UNCHANGED x\nB == UNCHANGED x\nNext == A \\/ B\nSpec == Init /\\ [][Next]_x"
			writeCoverageModule(t, dir, "M", body)
			writeParameterizedConfig(t, dir, tc.cfg)
			args := []string{"-deadlock", "-fp", "0", "-workers", "1", "-coverage", "9999", "M.tla"}
			out, code, stderr := runGoInDir(t, dir, args)
			if code == 0 || !strings.Contains(out+stderr, "-coverage does not yet support") {
				t.Fatalf("expected rejection: exit %d stderr %s output %s", code, stderr, out)
			}
			out, code, stderr = runGoInDir(t, dir, []string{"-deadlock", "M.tla"})
			if code != 0 || !strings.Contains(out, "7 states generated, 3 distinct states found") {
				t.Fatalf("default mode: exit %d stderr %s output %s", code, stderr, out)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				if javaCode != 0 || !strings.Contains(java, "7 states generated, 3 distinct states found") {
					t.Fatalf("Java: exit %d output %s", javaCode, java)
				}
				report := coverageReport(t, java)
				start := strings.Index(report, "<A line ")
				if start < 0 {
					t.Fatalf("Java action missing from coverage: %s", report)
				}
				lines := strings.Split(report[start:], "\n")
				if len(lines) < 2 || !strings.HasSuffix(lines[1], ": 1") {
					t.Fatalf("Java assignment count is not 1: %s", report[start:])
				}
			}
		})
	}
}

func TestJavaTLCCoverageInitialSetInvariantViolation(t *testing.T) {
	for _, tool := range []bool{false, true} {
		t.Run(boolName(tool), func(t *testing.T) {
			dir := t.TempDir()
			writeCoverageModule(t, dir, "M", "EXTENDS Naturals\nVARIABLE x\nInit == x \\in {1,2,3}\nNext == UNCHANGED x\nInv == x < 3")
			writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\nINVARIANT Inv\n")
			args := []string{"-deadlock", "-fp", "0", "-workers", "1", "-checkpoint", "0", "-coverage", "9999", "M.tla"}
			if tool {
				args = append([]string{"-tool"}, args...)
			}
			out, code, stderr := runGoInDir(t, dir, args)
			if code != 12 || stderr != "" {
				t.Fatalf("Go exit %d stderr %s\n%s", code, stderr, out)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				if javaCode != code {
					t.Fatalf("Java exit %d\n%s", javaCode, java)
				}
				if tool {
					got, want := coverageMessages(readToolMessages(out)), coverageMessages(readToolMessages(java))
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("tool coverage\nGo: %+v\nJava: %+v", got, want)
					}
				} else if strings.Contains(out, "The coverage statistics at") || strings.Contains(java, "The coverage statistics at") {
					t.Fatalf("unexpected coverage report\nGo:\n%s\nJava:\n%s", out, java)
				}
			}
		})
	}
}

func TestJavaTLCCoverageInitialSetMaxSize(t *testing.T) {
	dir := t.TempDir()
	writeCoverageModule(t, dir, "M", "VARIABLE x\nInit == x \\in {1,2,3}\nNext == UNCHANGED x")
	writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\n")
	args := []string{"-tool", "-deadlock", "-fp", "0", "-workers", "1", "-coverage", "9999", "-maxSetSize", "2", "M.tla"}
	out, code, stderr := runGoInDir(t, dir, args)
	if code != 0 || stderr != "" {
		t.Fatalf("Go exit %d stderr %s\n%s", code, stderr, out)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java, javaCode := runJavaTLCArgs(t, jar, dir, args)
		got, want := coverageMessages(readToolMessages(out)), coverageMessages(readToolMessages(java))
		if javaCode != code || !reflect.DeepEqual(got, want) || rawToolMessage(out, 2199) != rawToolMessage(java, 2199) {
			t.Fatalf("Java exit %d\nGo: %+v\nJava: %+v", javaCode, got, want)
		}
	}
}

func TestCoverageRejectsUnsafeDirectInitial(t *testing.T) {
	for _, tc := range []struct {
		name, body, javaCoverage string
		incomplete               bool
	}{
		{"incomplete", "VARIABLES x,y\nInit == x=0\nNext == UNCHANGED <<x,y>>", "", true},
		{"guard", "EXTENDS Naturals\nVARIABLE x\nInit == /\\ x \\in 0..2\n        /\\ x < 2\nNext == UNCHANGED x", "line 4, col 12 to line 4, col 21 of module M: 2\n  |line 4, col 18 to line 4, col 21 of module M: 1", false},
		{"guard-all", "EXTENDS Naturals\nVARIABLE x\nInit == /\\ x \\in 0..2\n        /\\ x < 4\nNext == UNCHANGED x", "line 4, col 12 to line 4, col 21 of module M: 3\n  |line 4, col 18 to line 4, col 21 of module M: 1", false},
		{"mixed", "EXTENDS Naturals\nVARIABLES x,y\nInit == y=0 /\\ x \\in 0..2\nNext == UNCHANGED <<x,y>>", "line 4, col 16 to line 4, col 25 of module M: 3\n  |line 4, col 22 to line 4, col 25 of module M: 1", false},
		{"multiple-ranges", "EXTENDS Naturals\nVARIABLES x,y,z\nInit == x \\in 1..1 /\\ y \\in 0..2 /\\ z \\in 0..1\nNext == UNCHANGED <<z,y,x>>", "line 4, col 9 to line 4, col 32 of module M: 1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeCoverageModule(t, dir, "M", tc.body)
			if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			args := []string{"-tool", "-deadlock", "-fp", "0", "-workers", "1", "-coverage", "9999", "M.tla"}
			out, code, stderr := runGoInDir(t, dir, args)
			if code == 0 || !strings.Contains(out+stderr, "-coverage does not yet support") {
				t.Fatalf("expected rejection: exit %d stderr %s output %s", code, stderr, out)
			}
			if tc.incomplete {
				out, exit, _ := runGoInDir(t, dir, []string{"-tool", "-deadlock", "-workers", "1", "M.tla"})
				messages := readToolMessages(out)
				if exit == 0 || !hasToolMessage(messages, 1000) || !hasToolMessage(messages, 2102) {
					t.Fatalf("expected existing Go error difference: exit %d output %s", exit, out)
				}
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java, exit := runJavaTLCArgs(t, jar, dir, args)
				if tc.incomplete {
					messages := readToolMessages(java)
					if exit == 0 || !hasToolMessage(messages, 2102) || hasToolMessage(messages, 1000) {
						t.Fatalf("expected Java's 2102-only illegal state: exit %d output %s", exit, java)
					}
				} else if exit != 0 || !strings.Contains(strings.ReplaceAll(java, "@!@!@ENDMSG 2221 @!@!@\n@!@!@STARTMSG 2221:0 @!@!@\n", ""), tc.javaCoverage) {
					t.Fatalf("unexpected Java coverage: exit %d output %s", exit, java)
				}
			}
		})
	}
}

func TestCoverageRejectsDirectInitialOutsideIntegerSubset(t *testing.T) {
	for _, tc := range []struct {
		name, initial string
		javaOverflow  bool
	}{
		{"value-too-large", "x=2147483648", true},
		{"range-start-too-large", "x \\in 2147483648..2147483649", true},
		{"range-end-too-large", "x \\in 0..2147483648", true},
		{"negative", "x = -1", false},
		{"interval-too-large", "x \\in 0..2147483647", false},
		{"interval-maximum", "x \\in 2147483647..2147483647", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeCoverageModule(t, dir, "M", "EXTENDS Integers\nVARIABLE x\nInit == "+tc.initial+"\nNext == UNCHANGED x")
			if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			args := []string{"-tool", "-workers", "1", "-coverage", "9999", "M.tla"}
			out, exit, stderr := runGoInDir(t, dir, args)
			if exit == 0 || !strings.Contains(out+stderr, "-coverage does not yet support") {
				t.Fatalf("expected rejection: exit %d stderr %s output %s", exit, stderr, out)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" && tc.javaOverflow {
				java, exit := runJavaTLCArgs(t, jar, dir, args)
				if exit == 0 || !hasToolMessage(readToolMessages(java), 2265) {
					t.Fatalf("expected Java 2265: exit %d output %s", exit, java)
				}
			}
		})
	}
}

func TestJavaTLCCoverageInitialLiteralSetAssignments(t *testing.T) {
	for _, tc := range []struct{ name, variables, initial, next, config string }{
		{"empty", "x", "x = {}", "UNCHANGED x", ""},
		{"numeric", "x", "x = {1,2,2}", "UNCHANGED x", ""},
		{"boolean", "x", "x = {TRUE,FALSE,TRUE}", "UNCHANGED x", ""},
		{"multiple", "x,y,z", "/\\ x = {}\n        /\\ y = {1,2}\n        /\\ z = 1", "UNCHANGED <<x,y,z>>", ""},
		{"inline", "x,y", "x = {1} /\\ y = {}", "UNCHANGED <<x,y>>", ""},
		{"invariant", "x", "x = {}", "UNCHANGED x\nInv == TRUE", "INIT Init\nNEXT Next\nINVARIANT Inv\n"},
		{"spec", "x", "x = {}", "UNCHANGED x\nSpec == Init /\\ [][Next]_x", "SPECIFICATION Spec\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeCoverageModule(t, dir, "M", "VARIABLES "+tc.variables+"\nInit == "+tc.initial+"\nNext == "+tc.next)
			cfg := tc.config
			if cfg == "" {
				cfg = "INIT Init\nNEXT Next\n"
			}
			writeParameterizedConfig(t, dir, cfg)
			compareParameterizedCoverage(t, dir)
		})
	}
}

func TestCoverageRejectsUnmodeledInitialSetAssignments(t *testing.T) {
	for _, initial := range []string{
		"x = {1, TRUE}",
		"x = {1 + 1}",
		"x = {} /\\ x = {}",
	} {
		t.Run(initial, func(t *testing.T) {
			dir := t.TempDir()
			writeCoverageModule(t, dir, "M", "EXTENDS Naturals\nVARIABLE x\nInit == "+initial+"\nNext == UNCHANGED x")
			writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\n")
			out, code, stderr := runGoInDir(t, dir, coverageAArgs("M.tla", true))
			if code == 0 || !strings.Contains(out+stderr, "-coverage does not yet support") {
				t.Fatalf("expected rejection: exit %d stderr %s output %s", code, stderr, out)
			}
		})
	}
}

func TestJavaTLCCoverageInitialLiteralSetAndNamedActions(t *testing.T) {
	dir := t.TempDir()
	writeCoverageModule(t, dir, "M", "VARIABLES x,y\nInit == /\\ x = {}\n        /\\ y = 0\nA == /\\ x' = x\n     /\\ y' = y\nB == /\\ x' = x\n     /\\ y' = y\nNext == A \\/ B")
	writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\n")
	compareParameterizedCoverage(t, dir)
}

func TestJavaTLCCoverageInitialLiteralSetAssignmentMaxSize(t *testing.T) {
	dir := t.TempDir()
	writeCoverageModule(t, dir, "M", "VARIABLE x\nInit == x = {1,2,3}\nNext == UNCHANGED x")
	writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\n")
	args := []string{"-tool", "-deadlock", "-fp", "0", "-workers", "1", "-coverage", "9999", "-maxSetSize", "2", "M.tla"}
	out, code, stderr := runGoInDir(t, dir, args)
	if code != 0 || stderr != "" {
		t.Fatalf("Go exit %d stderr %s\n%s", code, stderr, out)
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java, javaCode := runJavaTLCArgs(t, jar, dir, args)
		got, want := coverageMessages(readToolMessages(out)), coverageMessages(readToolMessages(java))
		if javaCode != code || !reflect.DeepEqual(got, want) {
			t.Fatalf("Java exit %d\nGo: %+v\nJava: %+v", javaCode, got, want)
		}
	}
}

func TestCoverageRejectsInstantiatedDirectUnchanged(t *testing.T) {
	for _, tc := range []struct{ name, body, cfg string }{
		{"next", "VARIABLE x\nInit == x=0\nI == INSTANCE Aux WITH x <- x", "INIT Init\nNEXT I!Act\n"},
		{"initial", "VARIABLE x\nNext == UNCHANGED x\nI == INSTANCE Aux WITH x <- x", "INIT I!Initial\nNEXT Next\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeCoverageModule(t, dir, "Aux", "VARIABLE x\nInitial == x=0\nAct == UNCHANGED x")
			writeCoverageModule(t, dir, "M", tc.body)
			if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte(tc.cfg), 0o600); err != nil {
				t.Fatal(err)
			}
			_, code, stderr := runGoInDir(t, dir, []string{"-coverage", "9999", "M.tla"})
			if code == 0 || !strings.Contains(stderr, "-coverage does not yet support") {
				t.Fatalf("exit %d stderr %s", code, stderr)
			}
		})
	}
}

func writeCoverageModule(t *testing.T, dir, name, body string) {
	t.Helper()
	source := "---- MODULE " + name + " ----\n" + body + "\n====\n"
	if err := os.WriteFile(filepath.Join(dir, name+".tla"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCoverageRejectsUnmodeledDirectNext(t *testing.T) {
	for _, action := range []string{
		"UNCHANGED <<x,x>>", "UNCHANGED <<x>>", "UNCHANGED y", "UNCHANGED <<x, 1>>",
		"UNCHANGED <<x, y>> /\\ x=0", "x'=x /\\ UNCHANGED y", "Alias",
	} {
		t.Run(action, func(t *testing.T) {
			dir := t.TempDir()
			writeCoverageModule(t, dir, "M", "VARIABLES x,y\nInit == /\\ x=0\n        /\\ y=0\nAlias == UNCHANGED <<x,y>>\nNext == "+action)
			if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			_, code, stderr := runGoInDir(t, dir, []string{"-coverage", "9999", "M.tla"})
			if code == 0 || !strings.Contains(stderr, "-coverage does not yet support") {
				t.Fatalf("exit %d stderr %s", code, stderr)
			}
		})
	}
}

func TestCoverageRejectsUnmodeledEnabledAction(t *testing.T) {
	for _, tc := range []struct{ name, next, extra, invariant string }{
		{"partial-conjuncts", "UNCHANGED x /\\ UNCHANGED y", "", "ENABLED Next"},
		{"different-action", "UNCHANGED <<x,y>>", "Other == UNCHANGED <<x,y>>", "ENABLED Other"},
		{"nested-enabled", "UNCHANGED <<x,y>>", "", "(ENABLED Next) /\\ x = 0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeCoverageModule(t, dir, "M", "VARIABLES x,y\nInit == /\\ x=0\n        /\\ y=0\nNext == "+tc.next+"\n"+tc.extra+"\nInv == "+tc.invariant)
			if err := os.WriteFile(filepath.Join(dir, "M.cfg"), []byte("INIT Init\nNEXT Next\nINVARIANT Inv\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			out, code, stderr := runGoInDir(t, dir, []string{"-coverage", "9999", "M.tla"})
			if code == 0 || !strings.Contains(out+stderr, "-coverage does not yet support") {
				t.Fatalf("exit %d stderr %s output %s", code, stderr, out)
			}
		})
	}
}
