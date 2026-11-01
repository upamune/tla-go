package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/upamune/tla-go/internal/javatmp"
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

// Java TLC reports zero initial states for models without variables, as in the
// upstream BagsTest, CodePlexBug21Test, and Github391Test models.
func TestJavaTLCVariableFreeInitialStates(t *testing.T) {
	for _, tc := range []struct{ name, body, cfg string }{
		{"empty configuration", "ASSUME {1, 2} = {2, 1}", ""},
		{"specification without initial predicate", "Spec == [][TRUE]_<<>>", "SPECIFICATION Spec\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, tc.body, tc.cfg)
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", "-deadlock", filepath.Join(dir, "M.tla")}, &out, &errout)
			messages := readToolMessages(out.String())
			want := "@!@!@STARTMSG 2189:0 @!@!@\nComputing initial states...\n@!@!@ENDMSG 2189 @!@!@\n" +
				"@!@!@STARTMSG 2190:0 @!@!@\nFinished computing initial states: 0 distinct states generated at "
			if code != 0 || errout.Len() != 0 || !strings.Contains(out.String(), want) || !hasToolMessage(messages, 2193) {
				t.Fatalf("exit %d; stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaToolMessages(t, jar, dir, code, messages)
			}
		})
	}
}

// Java TLC's DoInitFunctor reports progress before adding the third, fifth,
// ninth, ... initial state.
func TestJavaTLCInitialProgress(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x \\in 1..11\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n")
	args := []string{"-tool", "-workers", "1", "-fp", "0"}
	var out, errout bytes.Buffer
	code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
	got := progressMessages(out.String())
	if want := "Computed 2 initial states...|Computed 4 initial states...|Computed 8 initial states..."; code != 0 || got != want {
		t.Fatalf("exit %d, progress %q; stdout:\n%s\nstderr:\n%s", code, got, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		cmd := exec.Command("java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-jar", jar}, append(args, "M.tla")...)...)
		cmd.Dir = dir
		output, _ := javatmp.Apply(cmd).Output()
		if java := progressMessages(string(output)); java != got {
			t.Fatalf("progress differs\nJava: %q\nGo:   %q", java, got)
		}
	}
}

func progressMessages(output string) string {
	var bodies []string
	for _, message := range readToolMessages(output) {
		if message.code == 2269 {
			bodies = append(bodies, message.body)
		}
	}
	return strings.Join(bodies, "|")
}

// Java TLC replaces constant definitions by their values, so a configured
// INIT or NEXT that reads no variable is rejected with message 2233.
func TestJavaTLCConstantInitialPredicate(t *testing.T) {
	for _, tc := range []struct{ name, body, message string }{
		{"initial predicate", "VARIABLE x\nInit == TRUE\nNext == UNCHANGED x", "The initial predicate Init cannot be a constant."},
		{"next state action", "VARIABLE x\nInit == x = 0\nNext == 1 = 1", "The next state action Next cannot be a constant."},
		{"no variables", "Init == TRUE\nNext == TRUE", "The initial predicate Init cannot be a constant."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, tc.body, "INIT Init\nNEXT Next\n")
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final", filepath.Join(dir, "M.tla")}, &out, &errout)
			messages := readToolMessages(out.String())
			if code != 151 || errout.Len() != 0 || !slices.Contains(messages, toolMessage{code: 2233, level: 1, body: tc.message}) {
				t.Fatalf("exit %d; stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaToolMessages(t, jar, dir, code, messages)
			}
		})
	}
}
