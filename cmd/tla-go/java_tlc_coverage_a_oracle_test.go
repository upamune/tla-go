package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The snapshots come from Java CLI runs using selected arguments from the
// pinned upstream tests. They do not run the JUnit setup or assertions.
func TestJavaTLCUpstreamACoverageBoundary(t *testing.T) {
	fixture := coverageAFixtures(t)
	dir := t.TempDir()
	for _, ext := range []string{".tla", ".cfg"} {
		data, err := os.ReadFile(filepath.Join(fixture, "A"+ext))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "A"+ext), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	args := coverageAArgs("A.tla", true)
	out, code, stderr := runGoInDir(t, dir, args)
	if code == 0 || !strings.Contains(out+stderr, "-coverage does not yet support") {
		t.Fatalf("A coverage should remain rejected: exit %d stderr %s output %s", code, stderr, out)
	}
	out, code, stderr = runGoInDir(t, dir, coverageAArgs("A.tla", false))
	if code != 0 || stderr != "" || !strings.Contains(out, "7 states generated, 3 distinct states found") || !strings.Contains(out, "depth of the complete state graph search is 2") {
		t.Fatalf("A default: exit %d stderr %s output %s", code, stderr, out)
	}
	checkJavaACoverage(t, fixture, dir, args, "A.coverage", "7 states generated, 3 distinct states found")
	if jar := pinnedFairnessEffectJar(t); jar != "" {
		java, exit := runJavaTLCArgs(t, jar, dir, append([]string{"-tool"}, args...))
		messages := readToolMessages(java)
		if exit != 0 || !hasToolMessage(messages, 2186) || !hasToolMessage(messages, 2194) || !hasToolMessage(messages, 2199) || hasToolMessage(messages, 1000) || hasToolMessage(messages, 2776) {
			t.Fatalf("Java CLI tool messages: exit %d output %s", exit, java)
		}
		if !strings.Contains(java, "7 states generated, 3 distinct states found, 0 states left on queue") || !strings.Contains(java, "depth of the complete state graph search is 2") {
			t.Fatalf("Java CLI statistics: %s", java)
		}
	}
}

// A's two calls have different parameters and use the same nested definition.
// The smaller models isolate supported roots and still-unsupported child costs.
func TestJavaTLCCoverageParameterizedActionsBoundary(t *testing.T) {
	fixture := coverageAFixtures(t)
	for _, tc := range []struct{ name, definitions string }{
		{"direct", "A(n) == x'=n\nB(n) == x'=n\nNext == A(2) \\/ B(3)"},
		{"range", "A(n) == x' \\in 0..n\nB(n) == x'=n\nNext == A(2) \\/ B(3)"},
		{"guard", "A(n) == x<n /\\ x'=n\nB(n) == x'=n\nNext == A(2) \\/ B(3)"},
		{"dup", "A(n) == x'=n\nNext == A(2) \\/ A(3)"},
		{"reverse", "A(n) == x'=n\nB(n) == x'=n\nNext == B(3) \\/ A(2)"},
		{"unused", "A(n) == UNCHANGED x\nB(n) == x'=n\nNext == A(2) \\/ B(3)"},
		{"head-spacing", "A( n  ) == x'=n\nB(n,\n  m) == x'=n\nNext == A(2) \\/ B(3,4)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x=0\n" + tc.definitions + "\n====\n"
			for name, contents := range map[string]string{"M.tla": source, "M.cfg": "INIT Init\nNEXT Next\n"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			args := coverageAArgs("M.tla", true)
			out, code, stderr := runGoInDir(t, dir, args)
			if tc.name == "guard" {
				if code == 0 || !strings.Contains(out+stderr, "-coverage does not yet support") {
					t.Fatalf("coverage should remain rejected: exit %d stderr %s output %s", code, stderr, out)
				}
			} else {
				golden, err := os.ReadFile(filepath.Join(fixture, "A-"+tc.name+".coverage"))
				if err != nil {
					t.Fatal(err)
				}
				if code != 0 || stderr != "" || coverageReport(t, out) != string(golden) {
					t.Fatalf("coverage: exit %d stderr %s output %s", code, stderr, out)
				}
				compareParameterizedToolCoverage(t, dir, args)
			}
			checkJavaACoverage(t, fixture, dir, args, "A-"+tc.name+".coverage", "")
		})
	}
}

func compareParameterizedToolCoverage(t *testing.T, dir string, args []string) {
	t.Helper()
	toolArgs := append([]string{"-tool"}, args...)
	out, exit, stderr := runGoInDir(t, dir, toolArgs)
	if exit != 0 || stderr != "" {
		t.Fatalf("Go tool exit %d stderr %s output %s", exit, stderr, out)
	}
	if jar := pinnedFairnessEffectJar(t); jar != "" {
		javaArgs := append([]string{}, toolArgs[:len(toolArgs)-1]...)
		javaArgs = append(javaArgs, "-metadir", filepath.Join(t.TempDir(), "states"), toolArgs[len(toolArgs)-1])
		java, javaExit := runJavaTLCArgs(t, jar, dir, javaArgs)
		got, want := coverageMessages(readToolMessages(out)), coverageMessages(readToolMessages(java))
		if javaExit != 0 || !reflect.DeepEqual(got, want) {
			t.Fatalf("Java tool exit %d\nGo coverage %+v\nJava coverage %+v", javaExit, got, want)
		}
	}
}

func coverageAFixtures(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("testdata", "java-tlc", "upstream-coverage"))
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func coverageAArgs(module string, coverage bool) []string {
	args := []string{"-deadlock", "-fp", "0", "-workers", "1", "-checkpoint", "0"}
	if coverage {
		args = append(args, "-coverage", "9999")
	}
	return append(args, module)
}

func checkJavaACoverage(t *testing.T, fixture, dir string, args []string, snapshot, statistics string) {
	t.Helper()
	if jar := pinnedFairnessEffectJar(t); jar != "" {
		golden, err := os.ReadFile(filepath.Join(fixture, snapshot))
		if err != nil {
			t.Fatal(err)
		}
		out, code := runJavaTLCArgs(t, jar, dir, args)
		if code != 0 || coverageReport(t, out) != string(golden) || !strings.Contains(out, statistics) {
			t.Fatalf("Java exit %d output %s", code, out)
		}
	}
}
