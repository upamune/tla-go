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
