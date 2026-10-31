package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestJavaTLCNonBooleanInvariant(t *testing.T) {
	for _, tc := range []struct {
		name, expression, message string
	}{
		{"and left", `42 /\ TRUE`, "Attempted to evaluate an expression of form P /\\ Q when P was\nan integer."},
		{"and right", `TRUE /\ 42`, "Attempted to evaluate an expression of form P /\\ Q when Q was\nan integer."},
		{"or left", `42 \/ FALSE`, "Attempted to evaluate an expression of form P \\/ Q when P was\nan integer."},
		{"or right", `FALSE \/ 42`, "Attempted to evaluate an expression of form P \\/ Q when Q was\nan integer."},
		{"implies left", `42 => TRUE`, "Attempted to evaluate an expression of form P => Q when P was\nan integer."},
		{"implies right", `TRUE => 42`, "Attempted to evaluate an expression of form P => Q when Q was\nan integer."},
		{"equivalence left", `42 <=> TRUE`, "Attempted to evaluate an expression of form P <=> Q when P or Q was not a boolean."},
		{"equivalence right", `TRUE <=> 42`, "Attempted to evaluate an expression of form P <=> Q when P or Q was not a boolean."},
		{"if condition", `IF 42 THEN TRUE ELSE FALSE`, "A non-boolean expression (an integer) was used as the condition of an IF."},
		{"case condition", `CASE 42 -> TRUE [] OTHER -> FALSE`, "A non-boolean expression (an integer) was used as a condition of a CASE. 42"},
		{"case expression", `CASE (1+1) -> TRUE [] OTHER -> FALSE`, "A non-boolean expression (an integer) was used as a condition of a CASE. line 6, col 14 to line 6, col 16 of module M"},
		{"case variable", `CASE x -> TRUE [] OTHER -> FALSE`, "A non-boolean expression (an integer) was used as a condition of a CASE. line 6, col 13 to line 6, col 13 of module M"},
		{"forall", `\A i \in {1}: i`, "TLC expected a boolean value, but did not find one. line 6, col 8 to line 6, col 22 of module M"},
		{"exists", `\E i \in {1}: 42`, "TLC expected a boolean value, but did not find one. line 6, col 8 to line 6, col 23 of module M"},
		{"choose", `CHOOSE i \in {1}: i`, "TLC expected a boolean value, but did not find one. line 6, col 8 to line 6, col 26 of module M"},
		{"negation", `~42`, "Attempted to apply the operator ~ to a non-boolean\n(an integer)"},
		{"string", `TRUE /\ "s"`, "Attempted to evaluate an expression of form P /\\ Q when Q was\na string."},
		{"set", `TRUE /\ {1,2}`, "Attempted to evaluate an expression of form P /\\ Q when Q was\na set of the form {e1, ... ,eN}."},
		{"tuple", `TRUE /\ <<1,2>>`, "Attempted to evaluate an expression of form P /\\ Q when Q was\na tuple."},
		{"record", `TRUE /\ [a |-> 1]`, "Attempted to evaluate an expression of form P /\\ Q when Q was\na record."},
		{"function", `TRUE /\ [i \in 1..3 |-> i]`, "Attempted to evaluate an expression of form P /\\ Q when Q was\na function  of the form (d1 :> e1 @@ ... @@ dN :> eN)."},
		{"infinite function", `TRUE /\ [i \in Nat |-> i]`, "Attempted to evaluate an expression of form P /\\ Q when Q was\na function of the form  [x \\in S |-> expr]."},
		{"string domain function", `TRUE /\ [i \in {"a"} |-> 1]`, "Attempted to evaluate an expression of form P /\\ Q when Q was\na function  of the form (d1 :> e1 @@ ... @@ dN :> eN)."},
		{"updated record", `TRUE /\ [[a |-> 1] EXCEPT !.a = 2]`, "Attempted to evaluate an expression of form P /\\ Q when Q was\na record."},
		{"updated tuple", `TRUE /\ [<<1>> EXCEPT ![1] = 2]`, "Attempted to evaluate an expression of form P /\\ Q when Q was\na tuple."},
		{"updated function", `TRUE /\ [[i \in {"a"} |-> 1] EXCEPT !["a"] = 2]`, "Attempted to evaluate an expression of form P /\\ Q when Q was\na function  of the form (d1 :> e1 @@ ... @@ dN :> eN)."},
		{"interval", `TRUE /\ 1..3`, "Attempted to evaluate an expression of form P /\\ Q when Q was\na set of the form i..j."},
		{"booleans", `TRUE /\ BOOLEAN`, "Attempted to evaluate an expression of form P /\\ Q when Q was\na set of the form {e1, ... ,eN}."},
		{"function set", `TRUE /\ [1..2 -> 1..3]`, "Attempted to evaluate an expression of form P /\\ Q when Q was\na set of the form [S -> T]."},
		{"record set", `TRUE /\ [a: {1,2}]`, "Attempted to evaluate an expression of form P /\\ Q when Q was\na set of the form [d1 : S1, ... , dN : SN]."},
		{"subset", `TRUE /\ SUBSET {1,2}`, "Attempted to evaluate an expression of form P /\\ Q when Q was\na set of the form SUBSET S."},
		{"union", `TRUE /\ UNION {{1},{2}}`, "Attempted to evaluate an expression of form P /\\ Q when Q was\na set of the form {e1, ... ,eN}."},
		{"symbolic union", `TRUE /\ UNION {1..2,3..4}`, "Attempted to evaluate an expression of form P /\\ Q when Q was\na set of the form UNION  S."},
		{"filtered set", `TRUE /\ {i \in 1..3 : i>1}`, "Attempted to evaluate an expression of form P /\\ Q when Q was\na set of the form {e1, ... ,eN}."},
		{"direct", `42`, "TLC expected a boolean value, but did not find one. 42"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == x=0\nNext == UNCHANGED x\nInv == " + tc.expression
			writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nINVARIANT Inv\n")
			args := []string{"-tool", "-workers", "1", "-fp", "0"}
			var out, errout bytes.Buffer
			code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
			for _, fragment := range []string{tc.message, "@!@!@STARTMSG 2102:1 @!@!@", "While working on the initial state:", "x = 0"} {
				if code != 75 || errout.Len() != 0 || !strings.Contains(out.String(), fragment) {
					t.Fatalf("Go exit %d missing %q: stdout %s stderr %s", code, fragment, out.String(), errout.String())
				}
			}
			if tc.name != "direct" && !strings.HasPrefix(tc.name, "case ") {
				location := fmt.Sprintf("line 6, col 8 to line 6, col %d of module M", 7+len(tc.expression))
				if !strings.Contains(out.String(), location) {
					t.Fatalf("Go missing %q: %s", location, out.String())
				}
			}
			compareNonBooleanJava(t, dir, args, code, tc.message)
		})
	}
}

func TestJavaTLCNonBooleanSuccessor(t *testing.T) {
	for _, tc := range []struct{ name, expression, message string }{
		{"operator", `IF x=0 THEN TRUE ELSE TRUE /\ 42`, "Attempted to evaluate an expression of form P /\\ Q when Q was\nan integer.\nline 6, col 30 to line 6, col 39 of module M"},
		{"direct", `IF x=0 THEN TRUE ELSE 42`, "TLC expected a boolean value, but did not find one. line 6, col 8 to line 6, col 31 of module M"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == x=0\nNext == x'=x+1\nInv == " + tc.expression
			writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nINVARIANT Inv\n")
			args := []string{"-tool", "-workers", "1", "-fp", "0"}
			var out, errout bytes.Buffer
			code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
			for _, fragment := range []string{tc.message, "@!@!@STARTMSG 2111:1 @!@!@", "Evaluating invariant Inv failed.", "x = 0", "x = 1"} {
				if code != 76 || errout.Len() != 0 || !strings.Contains(out.String(), fragment) {
					t.Fatalf("Go exit %d missing %q: stdout %s stderr %s", code, fragment, out.String(), errout.String())
				}
			}
			compareNonBooleanJava(t, dir, args, code, tc.message)
		})
	}
}

func TestJavaTLCNonBooleanJunctionRanges(t *testing.T) {
	for _, tc := range []struct{ expression, location, position string }{
		{`42 /\ TRUE /\ TRUE`, "line 6, col 8 to line 6, col 17 of module M", "P"},
		{`TRUE /\ 42 /\ TRUE`, "line 6, col 8 to line 6, col 17 of module M", "Q"},
		{`/\ TRUE /\ 42 /\ TRUE`, "line 6, col 11 to line 6, col 20 of module M", "Q"},
		{`/\ TRUE /\ TRUE /\ 42`, "line 6, col 11 to line 6, col 28 of module M", "Q"},
	} {
		t.Run(tc.expression, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == x=0\nNext == UNCHANGED x\nInv == " + tc.expression
			writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nINVARIANT Inv\n")
			args := []string{"-tool", "-workers", "1", "-fp", "0"}
			var out, errout bytes.Buffer
			code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
			message := "Attempted to evaluate an expression of form P /\\ Q when " + tc.position + " was\nan integer.\n" + tc.location
			if code != 75 || errout.Len() != 0 || !strings.Contains(out.String(), message) {
				t.Fatalf("Go exit %d: stdout %s stderr %s", code, out.String(), errout.String())
			}
			compareNonBooleanJava(t, dir, args, code, message)
		})
	}
}

func TestJavaTLCNonBooleanProperty(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x=0\nNext == UNCHANGED x\nP == 42", "INIT Init\nNEXT Next\nPROPERTY P\n")
	args := []string{"-tool", "-workers", "1", "-fp", "0"}
	var out, errout bytes.Buffer
	code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
	message := "TLC expected a boolean value, but did not find one. 42"
	if code != 75 || errout.Len() != 0 || !strings.Contains(out.String(), message) || !strings.Contains(out.String(), "@!@!@STARTMSG 2102:1 @!@!@") {
		t.Fatalf("Go exit %d: stdout %s stderr %s", code, out.String(), errout.String())
	}
	compareNonBooleanJava(t, dir, args, code, message)
}

func TestJavaTLCNonBooleanSimulation(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x=0\nNext == UNCHANGED x\nInv == TRUE /\\ 42", "INIT Init\nNEXT Next\nINVARIANT Inv\n")
	args := []string{"-tool", "-workers", "1", "-fp", "0", "-seed", "42", "-simulate", "num=1", "-depth", "1"}
	var out, errout bytes.Buffer
	code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
	message := "Attempted to evaluate an expression of form P /\\ Q when Q was\nan integer.\nline 6, col 8 to line 6, col 17 of module M"
	if code != 255 || errout.Len() != 0 || !strings.Contains(out.String(), message) || !strings.Contains(out.String(), "@!@!@STARTMSG 2102:1 @!@!@") {
		t.Fatalf("Go exit %d: stdout %s stderr %s", code, out.String(), errout.String())
	}
	compareNonBooleanJava(t, dir, args, code, message)
}

func TestJavaTLCNonBooleanSimulationDirect(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x=0\nNext == UNCHANGED x\nInv == 42", "INIT Init\nNEXT Next\nINVARIANT Inv\n")
	args := []string{"-tool", "-workers", "1", "-fp", "0", "-seed", "42", "-simulate", "num=1", "-depth", "1"}
	var out, errout bytes.Buffer
	code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
	message := "@!@!@STARTMSG 2102:1 @!@!@\n@!@!@STARTMSG 2215:0 @!@!@\nTLC expected a boolean value, but did not find one. 42"
	if code != 255 || errout.Len() != 0 || !strings.Contains(out.String(), message) {
		t.Fatalf("Go exit %d: stdout %s stderr %s", code, out.String(), errout.String())
	}
	compareNonBooleanJava(t, dir, args, code, message)
}

func TestJavaTLCNonBooleanGeneration(t *testing.T) {
	for _, tc := range []struct{ name, init, next, message string }{
		{"initial literal", `/\ x=0 /\ 42`, `x'=x+1`, "In computing initial states, TLC expected a boolean expression,\nbut instead found 42.\n42"},
		{"initial expression", `/\ x=0 /\ (1+1)`, `x'=x+1`, "In computing initial states, TLC expected a boolean expression,\nbut instead found 2.\nline 4, col 20 to line 4, col 22 of module M"},
		{"next literal", `x=0`, `/\ x'=x+1 /\ 42`, "In computing next states, TLC expected a boolean expression,\nbut instead found 42.\n42"},
		{"next expression", `x=0`, `/\ x'=x+1 /\ (1+1)`, "In computing next states, TLC expected a boolean expression,\nbut instead found 2.\nline 5, col 23 to line 5, col 25 of module M"},
	} {
		for _, simulate := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/simulate=%t", tc.name, simulate), func(t *testing.T) {
				dir := t.TempDir()
				writeDOTFixture(t, dir, "VARIABLE x\nInit == "+tc.init+"\nNext == "+tc.next, "INIT Init\nNEXT Next\n")
				args := []string{"-tool", "-workers", "1", "-fp", "0", "-seed", "42"}
				if simulate {
					args = append(args, "-simulate", "num=1", "-depth", "1")
				}
				var out, errout bytes.Buffer
				code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
				wantCode := 75
				if simulate {
					wantCode = 255
				}
				if code != wantCode || errout.Len() != 0 || !strings.Contains(out.String(), tc.message) || !strings.Contains(out.String(), "@!@!@STARTMSG 2247:") {
					t.Fatalf("Go exit %d: stdout %s stderr %s", code, out.String(), errout.String())
				}
				fragments := []string{tc.message, "@!@!@STARTMSG 2247:"}
				if !simulate {
					wrapper := "@!@!@STARTMSG 1000:1 @!@!@\n@!@!@STARTMSG 2247:0 @!@!@"
					if strings.HasPrefix(tc.name, "next") {
						wrapper = ": @!@!@STARTMSG 2247:0 @!@!@"
					}
					if !strings.Contains(out.String(), wrapper) {
						t.Fatalf("Go missing %q: %s", wrapper, out.String())
					}
					fragments = append(fragments, wrapper)
				}
				compareNonBooleanJava(t, dir, args, code, fragments...)
			})
		}
	}
}

func compareNonBooleanJava(t *testing.T, dir string, args []string, code int, fragments ...string) {
	t.Helper()
	for _, env := range []string{"TLC_JAR", "TLC_MODERN_JAR"} {
		jar := os.Getenv(env)
		if jar == "" {
			continue
		}
		javaArgs := append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-jar", jar}, args...)
		javaArgs = append(javaArgs, "-metadir", t.TempDir(), "M.tla")
		cmd := exec.Command("java", javaArgs...)
		cmd.Dir = dir
		output, _ := cmd.CombinedOutput()
		if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code {
			t.Fatalf("%s exit %v output %s", env, cmd.ProcessState, output)
		}
		for _, fragment := range fragments {
			if !strings.Contains(string(output), fragment) {
				t.Fatalf("%s missing %q: %s", env, fragment, output)
			}
		}
	}
}
