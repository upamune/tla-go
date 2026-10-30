package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// etest7 and test201a are unchanged models from Java TLC v1.7.4.
func TestJavaTLCConstantConfigurationErrors(t *testing.T) {
	for _, tc := range []struct{ name, diagnostic string }{
		{"etest7", "The configuration file substitutes constant C with non-constant Foo - specifically: In evaluation, the identifier x is either undefined or not an operator.\nline 11, col 8 to line 11, col 8 of module etest7"},
		{"test201a", "The constant parameter A is not assigned a value by the configuration file."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, ext := range []string{".tla", ".cfg"} {
				name := tc.name + ext
				data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", name))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, tc.name+".tla")}, &out, &errout)
			if code != 151 || errout.Len() != 0 || !strings.Contains(out.String(), "Error: "+tc.diagnostic) {
				t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runJavaTLCForExit(t, jar, dir, tc.name, 151)
				if !strings.Contains(java, tc.diagnostic) {
					t.Fatalf("Java TLC:\n%s", java)
				}
			}
		})
	}
}

func TestJavaTLCConstantSubstitutionEvaluation(t *testing.T) {
	for _, tc := range []struct {
		name, constant, definition, next, substitution, stats string
		prints                                                int
	}{
		{"unreachable state", "C", `Foo == IF FALSE THEN x ELSE 1`, "C", "C <- Foo", "3 states generated, 2 distinct states found, 0 states left on queue.", 0},
		{"stateful operator", "F(_)", `Foo(n) == x+n`, "F(0)", "F <- Foo", "2 states generated, 1 distinct states found, 0 states left on queue.", 0},
		{"effects", "C", `Foo == IF FALSE THEN x ELSE Print("ran", 1)`, "C", "C <- Foo", "3 states generated, 2 distinct states found, 0 states left on queue.", 3},
		{"direct effects", "C", `Foo == Print("ran", 1)`, "C", "C <- Foo", "3 states generated, 2 distinct states found, 0 states left on queue.", 3},
		{"nested effects", "C", "Bar == Print(\"ran\", 1)\nFoo == Bar", "C", "C <- Foo", "3 states generated, 2 distinct states found, 0 states left on queue.", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nCONSTANT " + tc.constant + "\nVARIABLE x\n" + tc.definition + "\nInit == x = 0\nNext == x' = " + tc.next + "\n====\n"
			for name, contents := range map[string]string{"M.tla": source, "M.cfg": "INIT Init\nNEXT Next\nCONSTANT " + tc.substitution + "\n"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != 0 || errout.Len() != 0 || !strings.Contains(out.String(), tc.stats) || strings.Count(out.String(), `"ran"  1`) != tc.prints {
				t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runJavaTLCForExit(t, jar, dir, "M", 0)
				if !strings.Contains(java, tc.stats) || strings.Count(java, `"ran"  1`) != tc.prints {
					t.Fatalf("Java TLC:\n%s", java)
				}
			}
		})
	}
}

func TestJavaTLCConstantSubstitutionRuntimeFailures(t *testing.T) {
	for _, tc := range []struct{ name, expression, diagnostic string }{
		{"division", `1 \div 0`, "The configuration file substitutes constant C with non-constant Foo"},
		{"empty choose", `CHOOSE y \in {} : TRUE`, "The configuration file substitutes constant C with non-constant Foo - specifically: Attempted to compute the value of an expression of form\nCHOOSE x \\in S: P, but no element of S satisfied P."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nCONSTANT C\nVARIABLE x\nFoo == " + tc.expression + "\nInit == x = 0\nNext == x' = C\n====\n"
			for name, contents := range map[string]string{"M.tla": source, "M.cfg": "INIT Init\nNEXT Next\nCONSTANT C <- Foo\n"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != 151 || errout.Len() != 0 || !strings.Contains(out.String(), "Error: "+tc.diagnostic) {
				t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runJavaTLCForExit(t, jar, dir, "M", 151)
				if !strings.Contains(java, tc.diagnostic) {
					t.Fatalf("Java TLC:\n%s", java)
				}
			}
		})
	}
}

func TestJavaTLCConstantSubstitutionStartupEffects(t *testing.T) {
	for _, tc := range []struct {
		name, definitions string
		want              []string
	}{
		{"direct", `Foo == IF FALSE THEN x ELSE Print("target", 1)`, []string{`"target"  1`, `"init"  TRUE`, `"target"  1`, `"target"  1`}},
		{"nested", "Bar == Print(\"target\", 1)\nFoo == Bar", []string{`"target"  1`, `"target"  1`, `"init"  TRUE`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nCONSTANT C\nVARIABLE x\n" + tc.definitions + "\nInit == /\\ Print(\"init\", TRUE) /\\ x = 0\nNext == x' = C\n====\n"
			for name, contents := range map[string]string{"M.tla": source, "M.cfg": "INIT Init\nNEXT Next\nCONSTANT C <- Foo\n"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != 0 || errout.Len() != 0 || !containsLinesInOrder(out.String(), tc.want) {
				t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runJavaTLCForExit(t, jar, dir, "M", 0)
				if !containsLinesInOrder(java, tc.want) {
					t.Fatalf("Java TLC:\n%s", java)
				}
			}
		})
	}
}

func containsLinesInOrder(output string, expected []string) bool {
	for _, line := range expected {
		index := strings.Index(output, line+"\n")
		if index < 0 {
			return false
		}
		output = output[index+len(line)+1:]
	}
	return true
}

func TestJavaTLCMissingConstantPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name, declarations, diagnostic string
		code                           int
	}{
		{"unused", "CONSTANT C\nA == 1", "The constant parameter C is not assigned a value by the configuration file.", 151},
		{"operator", "CONSTANT F(_)\nA == 1", "The constant parameter F is not assigned a value by the configuration file.", 151},
		{"before missing init", "VARIABLE x\nCONSTANT C", "The constant parameter C is not assigned a value by the configuration file.", 151},
		{"semantic before configuration", "CONSTANT C\nBad == Unknown", "undefined symbol Unknown", 150},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, contents := range map[string]string{"M.tla": "---- MODULE M ----\n" + tc.declarations + "\n====\n", "M.cfg": ""} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != tc.code || !strings.Contains(out.String()+errout.String(), tc.diagnostic) {
				t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				runJavaTLCForExit(t, jar, dir, "M", tc.code)
			}
		})
	}
}
