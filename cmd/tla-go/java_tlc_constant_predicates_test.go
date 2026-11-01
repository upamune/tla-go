package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/upamune/tla-go/internal/javatmp"
)

func TestJavaTLCConstantPredicates(t *testing.T) {
	for _, kind := range []string{"INVARIANT", "CONSTRAINT", "ACTION_CONSTRAINT", "PROPERTY"} {
		for _, truth := range []string{"FALSE", "TRUE"} {
			for _, simulate := range []bool{false, true} {
				name := fmt.Sprintf("%s/%s/simulate=%t", kind, truth, simulate)
				t.Run(name, func(t *testing.T) {
					dir := t.TempDir()
					body := "VARIABLE x\nASSUME PrintT(\"assume\")\nInit == /\\ PrintT(\"init\") /\\ x=0\nNext == x'=x\nPred == /\\ PrintT(\"pred\") /\\ " + truth
					writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\n"+kind+" Pred\n")
					args := []string{"-tool", "-workers", "1", "-seed", "42"}
					if simulate {
						args = append(args, "-simulate", "num=1", "-depth", "1")
					}
					var out, errout bytes.Buffer
					code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
					wantCode := 0
					wantEvents := []string{`"pred"`, `"assume"`, `"init"`}
					if truth == "FALSE" {
						wantCode = 151
						wantEvents = []string{`"pred"`}
					}
					pattern := regexp.MustCompile(`(?m)^"(?:pred|assume|init)"$`)
					got := pattern.FindAllString(out.String(), -1)
					if code != wantCode || errout.Len() != 0 || !reflect.DeepEqual(got, wantEvents) {
						t.Fatalf("Go exit %d events %q stderr %s stdout %s", code, got, errout.String(), out.String())
					}
					for _, env := range []string{"TLC_JAR", "TLC_MODERN_JAR"} {
						if jar := os.Getenv(env); jar != "" {
							cmd := exec.Command("java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-jar", jar}, append(args, "M.tla")...)...)
							cmd.Dir = dir
							output, _ := javatmp.Apply(cmd).CombinedOutput()
							if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code || !reflect.DeepEqual(pattern.FindAllString(string(output), -1), got) {
								t.Fatalf("%s exit %v output %s", env, cmd.ProcessState, output)
							}
							if truth == "FALSE" {
								message := "The " + strings.ToLower(strings.ReplaceAll(kind, "_", " ")) + " of Pred is equal to FALSE"
								if !strings.Contains(string(output), "@!@!@STARTMSG 2230:1") || !strings.Contains(out.String(), "@!@!@STARTMSG 2230:1") || !strings.Contains(string(output), message) || !strings.Contains(out.String(), message) {
									t.Fatalf("missing constant configuration diagnostic: Go %s Java %s", out.String(), output)
								}
							}
						}
					}
				})
			}
		}
	}
}

func TestJavaTLCConstantPredicateOrder(t *testing.T) {
	for _, tc := range []struct{ property, constraint string }{{"TRUE", "TRUE"}, {"FALSE", "TRUE"}, {"FALSE", "FALSE"}} {
		t.Run(tc.property+"/"+tc.constraint, func(t *testing.T) {
			dir := t.TempDir()
			body := `VARIABLE x
ASSUME PrintT("assume")
Init == /\ PrintT("init") /\ x=0
Next == x'=x
D == /\ PrintT("action constraint") /\ TRUE
C == /\ PrintT("constraint") /\ ` + tc.constraint + `
B == /\ PrintT("property") /\ ` + tc.property + `
A == /\ PrintT("invariant") /\ TRUE
Unused == /\ PrintT("unused") /\ TRUE`
			cfg := "INIT Init\nNEXT Next\nACTION_CONSTRAINT D\nCONSTRAINT C\nPROPERTY B\nINVARIANT A\n"
			writeDOTFixture(t, dir, body, cfg)
			args := []string{"-tool", "-workers", "1", "-seed", "42"}
			var out, errout bytes.Buffer
			code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
			pattern := regexp.MustCompile(`(?m)^"(?:invariant|property|constraint|action constraint|unused|assume|init)"$`)
			want := []string{`"action constraint"`, `"constraint"`, `"property"`, `"invariant"`, `"unused"`, `"assume"`, `"init"`}
			wantCode := 0
			if tc.property == "FALSE" {
				want, wantCode = want[:5], 151
			}
			got := pattern.FindAllString(out.String(), -1)
			if code != wantCode || errout.Len() != 0 || !reflect.DeepEqual(got, want) || tc.property == "FALSE" && !strings.Contains(out.String(), "The property of B is equal to FALSE") {
				t.Fatalf("Go exit %d events %q stderr %s stdout %s", code, got, errout.String(), out.String())
			}
			for _, env := range []string{"TLC_JAR", "TLC_MODERN_JAR"} {
				if jar := os.Getenv(env); jar != "" {
					cmd := exec.Command("java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-jar", jar}, append(args, "M.tla")...)...)
					cmd.Dir = dir
					output, _ := javatmp.Apply(cmd).CombinedOutput()
					if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code || !reflect.DeepEqual(pattern.FindAllString(string(output), -1), got) || tc.property == "FALSE" && !strings.Contains(string(output), "The property of B is equal to FALSE") {
						t.Fatalf("%s exit %v output %s", env, cmd.ProcessState, output)
					}
				}
			}
		})
	}
}

func TestJavaTLCModernConstantPostcondition(t *testing.T) {
	dir := t.TempDir()
	body := `VARIABLE x
Init == /\ PrintT("init") /\ x=0
Next == x'=x+1
Inv == x=0
Post == /\ PrintT("post") /\ FALSE`
	writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nINVARIANT Inv\nPOSTCONDITION Post\n")
	args := []string{"-tool", "-workers", "1", "-seed", "42"}
	var out, errout bytes.Buffer
	code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
	pattern := regexp.MustCompile(`(?m)^"post"$`)
	got := pattern.FindAllString(out.String(), -1)
	if code != 12 || errout.Len() != 0 || !reflect.DeepEqual(got, []string{`"post"`}) || strings.Index(out.String(), `"post"`) > strings.Index(out.String(), `"init"`) {
		t.Fatalf("Go exit %d events %q stderr %s stdout %s", code, got, errout.String(), out.String())
	}
	if jar := os.Getenv("TLC_MODERN_JAR"); jar != "" {
		cmd := exec.Command("java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-jar", jar}, append(args, "M.tla")...)...)
		cmd.Dir = dir
		output, _ := javatmp.Apply(cmd).CombinedOutput()
		if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code || !reflect.DeepEqual(pattern.FindAllString(string(output), -1), got) || strings.Index(string(output), `"post"`) > strings.Index(string(output), `"init"`) {
			t.Fatalf("Java exit %v output %s", cmd.ProcessState, output)
		}
	}
}

func TestJavaTLCConstantAssignmentPrecomputation(t *testing.T) {
	dir := t.TempDir()
	body := `CONSTANT K
VARIABLE x
Init == x=0
Next == x'=x
Unused == PrintT("unused")`
	writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nCONSTANT K = FALSE\nINVARIANT K\n")
	args := []string{"-tool", "-workers", "1", "-seed", "42"}
	var out, errout bytes.Buffer
	code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
	pattern := regexp.MustCompile(`(?m)^"unused"$`)
	got := pattern.FindAllString(out.String(), -1)
	message := "The invariant of K is equal to FALSE"
	if code != 151 || errout.Len() != 0 || !reflect.DeepEqual(got, []string{`"unused"`}) || !strings.Contains(out.String(), message) {
		t.Fatalf("Go exit %d events %q stderr %s stdout %s", code, got, errout.String(), out.String())
	}
	for _, env := range []string{"TLC_JAR", "TLC_MODERN_JAR"} {
		if jar := os.Getenv(env); jar != "" {
			cmd := exec.Command("java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-jar", jar}, append(args, "M.tla")...)...)
			cmd.Dir = dir
			output, _ := javatmp.Apply(cmd).CombinedOutput()
			if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code || !reflect.DeepEqual(pattern.FindAllString(string(output), -1), got) || !strings.Contains(string(output), message) {
				t.Fatalf("%s exit %v output %s", env, cmd.ProcessState, output)
			}
		}
	}
}
