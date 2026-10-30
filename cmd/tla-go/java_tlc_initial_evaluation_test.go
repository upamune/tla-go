package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJavaTLCUpstreamInitialEvaluationFailures(t *testing.T) {
	for _, tc := range []struct {
		name, message, counts, nested, source string
	}{
		{"DoInitFunctorEvalException", "TLC expected a boolean value, but did not find one. line 15, col 15 to line 15, col 18 of module DoInitFunctorEvalException", "1 states generated, 0 distinct states found, 0 states left on queue", "@!@!@STARTMSG 2102:1 @!@!@\n@!@!@STARTMSG 2215:0 @!@!@", "Line 7, column 9 to line 7, column 19"},
		{"DoInitFunctorMinimalErrorStack", "The first argument of > should be an integer, but instead it is:\n<<1, 1>>", "1 states generated, 1 distinct states found, 1 states left on queue", "@!@!@STARTMSG 2266:0 @!@!@", "DoInitFunctorMinimalErrorStack"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, ext := range []string{".tla", ".cfg"} {
				name := tc.name + ext
				data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", "do-init-functor", name))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", filepath.Join(dir, tc.name+".tla")}, &out, &errout)
			if code != 75 || !strings.Contains(out.String(), tc.message) || !strings.Contains(out.String(), tc.counts) || !strings.Contains(out.String(), tc.nested) || !strings.Contains(out.String(), tc.source) {
				t.Fatalf("Go exit %d: %s %s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				java := runJavaTLCForExit(t, jar, dir, tc.name, 75)
				if !strings.Contains(java, tc.message) || !strings.Contains(java, tc.counts) {
					t.Fatalf("Java output: %s", java)
				}
			}
		})
	}
}

func TestJavaTLCInitialComparisonArgumentFailures(t *testing.T) {
	for _, module := range []string{"Naturals", "Integers"} {
		for _, op := range []string{"<", "<=", ">", ">="} {
			for _, position := range []string{"first", "second"} {
				t.Run(module+"/"+op+"/"+position, func(t *testing.T) {
					dir := t.TempDir()
					left, right := "x", "0"
					if position == "second" {
						left, right = "0", "x"
					}
					source := fmt.Sprintf("---- MODULE M ----\nEXTENDS %s\nVARIABLE x\nInit == x = <<1, 1>>\nNext == UNCHANGED x\nInv == %s %s %s\n====\n", module, left, op, right)
					for name, content := range map[string]string{"M.tla": source, "M.cfg": "INIT Init\nNEXT Next\nINVARIANT Inv\n"} {
						if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
							t.Fatal(err)
						}
					}
					displayedOp := op
					if module == "Naturals" && op == ">=" {
						displayedOp = ">"
					}
					message := fmt.Sprintf("The %s argument of %s should be an integer, but instead it is:\n<<1, 1>>", position, displayedOp)
					var out, errout bytes.Buffer
					code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", filepath.Join(dir, "M.tla")}, &out, &errout)
					if code != 75 || !strings.Contains(out.String(), message) || !strings.Contains(out.String(), "@!@!@STARTMSG 2266:0 @!@!@") {
						t.Fatalf("Go exit %d: %s %s", code, out.String(), errout.String())
					}
					if jar := os.Getenv("TLC_JAR"); jar != "" && !strings.Contains(runJavaTLCForExit(t, jar, dir, "M", 75), message) {
						t.Fatalf("Java output does not contain %q", message)
					}
				})
			}
		}
	}
}

func TestJavaTLCNonBooleanSuccessorConstraint(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == x' = x + 1\nCon == IF x = 0 THEN TRUE ELSE {}", "INIT Init\nNEXT Next\nCONSTRAINT Con\n")
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", filepath.Join(dir, "M.tla")}, &out, &errout)
	for _, fragment := range []string{
		"@!@!@STARTMSG 1000:1 @!@!@", "util.Assert$TLCRuntimeException: @!@!@STARTMSG 2215:0 @!@!@",
		"The error call stack is empty.", "2 states generated, 1 distinct states found, 0 states left on queue",
	} {
		if code != 75 || !strings.Contains(out.String(), fragment) {
			t.Fatalf("Go exit %d missing %q: %s %s", code, fragment, out.String(), errout.String())
		}
	}
	if strings.Contains(out.String(), "@!@!@STARTMSG 2121:") {
		t.Fatalf("Go unexpectedly emitted a trace: %s", out.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		java := runJavaTLCForExit(t, jar, dir, "M", 75)
		if !strings.Contains(java, "The error call stack is empty.") || !strings.Contains(java, "2 states generated, 1 distinct states found, 0 states left on queue") {
			t.Fatalf("Java output: %s", java)
		}
	}
}
