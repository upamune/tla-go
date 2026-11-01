package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/upamune/tla-go/internal/javatmp"
)

func TestJavaTLCModernSimulationExit(t *testing.T) {
	falseEvents := []string{`<<"inv", 0>>`}
	wantStates := map[string]string{"assumption": "1", "initial": "1", "next": "3", "invariant": "2", "deadlock": "1", "false": "13", "branch": "4"}
	for range 12 {
		falseEvents = append(falseEvents, `<<"next", 0>>`, `<<"inv", 0>>`)
	}
	for _, tc := range []struct {
		name, assumption, init, next, invariant string
		want                                    []string
	}{
		{"assumption", `ASSUME TLCSet("exit", TRUE)`, `x=0`, `x'=x+1`, `PrintT(<<"inv", x>>) /\ x<2`, []string{`<<"inv", 0>>`}},
		{"initial", ``, `TLCSet("exit", TRUE) /\ x=0`, `x'=x+1`, `PrintT(<<"inv", x>>) /\ x<2`, []string{`<<"inv", 0>>`}},
		{"next", ``, `x=0`, `PrintT(<<"next", x>>) /\ TLCSet("exit", x=1) /\ x'=x+1`, `PrintT(<<"inv", x>>) /\ x<2`, []string{`<<"inv", 0>>`, `<<"next", 0>>`, `<<"inv", 1>>`, `<<"next", 1>>`, `<<"inv", 2>>`}},
		{"invariant", ``, `x=0`, `x'=x+1`, `PrintT(<<"inv", x>>) /\ TLCSet("exit", x=1) /\ x<2`, []string{`<<"inv", 0>>`, `<<"inv", 1>>`}},
		{"deadlock", ``, `x=0`, `TLCSet("exit", TRUE) /\ FALSE /\ x'=1`, `PrintT(<<"inv", x>>) /\ x<2`, []string{`<<"inv", 0>>`}},
		{"false", ``, `x=0`, `PrintT(<<"next", x>>) /\ TLCSet("exit", FALSE) /\ x'=0`, `PrintT(<<"inv", x>>) /\ x=0`, falseEvents},
		{"branch", ``, `x=0`, `PrintT(<<"next", x>>) /\ TLCSet("exit", TRUE) /\ x' \in {1,2,3}`, `PrintT(<<"inv", x>>) /\ x<3`, []string{`<<"inv", 0>>`, `<<"next", 0>>`, `<<"inv", 1>>`, `<<"inv", 2>>`, `<<"inv", 3>>`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\n" + tc.assumption + "\nInit == " + tc.init + "\nNext == " + tc.next + "\nInv == " + tc.invariant
			writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nINVARIANT Inv\n")
			args := []string{"-tool", "-workers", "1", "-seed", "42", "-simulate", "num=3", "-depth", "4"}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			var out, errout bytes.Buffer
			code := run(ctx, append(args, filepath.Join(dir, "M.tla")), &out, &errout)
			pattern := regexp.MustCompile(`(?m)^<<"(?:inv|next)", [0-9]+>>$`)
			got := pattern.FindAllString(out.String(), -1)
			if code != 0 || errout.Len() != 0 || !reflect.DeepEqual(got, tc.want) || simulationExitStateCount(out.String()) != wantStates[tc.name] {
				t.Fatalf("Go exit %d, events %q, stderr %s, stdout %s", code, got, errout.String(), out.String())
			}
			if jar := os.Getenv("TLC_MODERN_JAR"); jar != "" {
				cmd := exec.Command("java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-jar", jar}, append(args, "M.tla")...)...)
				cmd.Dir = dir
				output, _ := javatmp.Apply(cmd).CombinedOutput()
				if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code || !reflect.DeepEqual(pattern.FindAllString(string(output), -1), got) || simulationExitStateCount(string(output)) != wantStates[tc.name] {
					t.Fatalf("Java exit %v, output %s", cmd.ProcessState, output)
				}
			}
		})
	}
}

func simulationExitStateCount(output string) string {
	pattern := regexp.MustCompile(`(?m)^Progress\([^\n]*?: ([0-9]+) states generated`)
	matches := pattern.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return ""
	}
	return matches[len(matches)-1][1]
}

func TestJavaTLCModernSimulationExitEvaluationError(t *testing.T) {
	dir := t.TempDir()
	body := `VARIABLE x
Init == x=0
Next == /\ TLCSet("exit", TRUE) /\ PrintT("after") /\ x'=1 \div 0`
	writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\n")
	args := []string{"-tool", "-workers", "1", "-seed", "42", "-simulate", "num=3", "-depth", "4"}
	var out, errout bytes.Buffer
	code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
	if code != 255 || !strings.Contains(out.String(), `"after"`) {
		t.Fatalf("Go exit %d, stderr %s, stdout %s", code, errout.String(), out.String())
	}
	if jar := os.Getenv("TLC_MODERN_JAR"); jar != "" {
		cmd := exec.Command("java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-jar", jar}, append(args, "M.tla")...)...)
		cmd.Dir = dir
		output, _ := javatmp.Apply(cmd).CombinedOutput()
		if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code || !strings.Contains(string(output), `"after"`) {
			t.Fatalf("Java exit %v, output %s", cmd.ProcessState, output)
		}
	}
}

func TestJavaTLCModernSimulationExitConstraint(t *testing.T) {
	dir := t.TempDir()
	body := `VARIABLE x
Init == x=0
Next == /\ PrintT(<<"next", x>>) /\ x'=x+1
C == /\ PrintT(<<"constraint", x>>) /\ TLCSet("exit", x=1)`
	writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nCONSTRAINT C\n")
	args := []string{"-tool", "-workers", "1", "-seed", "42", "-simulate", "num=3", "-depth", "4"}
	var out, errout bytes.Buffer
	code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
	pattern := regexp.MustCompile(`(?m)^<<"(?:constraint|next)", [0-9]+>>$`)
	got := pattern.FindAllString(out.String(), -1)
	want := []string{`<<"constraint", 0>>`, `<<"next", 0>>`, `<<"constraint", 1>>`}
	if code != 0 || errout.Len() != 0 || !reflect.DeepEqual(got, want) {
		t.Fatalf("Go exit %d, events %q, stderr %s, stdout %s", code, got, errout.String(), out.String())
	}
	if jar := os.Getenv("TLC_MODERN_JAR"); jar != "" {
		cmd := exec.Command("java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-jar", jar}, append(args, "M.tla")...)...)
		cmd.Dir = dir
		output, _ := javatmp.Apply(cmd).CombinedOutput()
		if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code || !reflect.DeepEqual(pattern.FindAllString(string(output), -1), got) {
			t.Fatalf("Java exit %v, output %s", cmd.ProcessState, output)
		}
	}
}

func TestJavaTLCModernSimulationExitLiveness(t *testing.T) {
	dir := t.TempDir()
	body := `VARIABLE x
Init == x=0
Next == /\ PrintT(<<"next", x>>) /\ TLCSet("exit", x=1) /\ x'=x+1
P == <> (x=9)`
	writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nPROPERTY P\n")
	args := []string{"-tool", "-workers", "1", "-seed", "42", "-simulate", "num=3", "-depth", "4"}
	var out, errout bytes.Buffer
	code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
	pattern := regexp.MustCompile(`(?m)^<<"next", [0-9]+>>$`)
	got := pattern.FindAllString(out.String(), -1)
	want := []string{`<<"next", 0>>`, `<<"next", 1>>`}
	if code != 0 || errout.Len() != 0 || !reflect.DeepEqual(got, want) {
		t.Fatalf("Go exit %d, events %q, stderr %s, stdout %s", code, got, errout.String(), out.String())
	}
	if jar := os.Getenv("TLC_MODERN_JAR"); jar != "" {
		cmd := exec.Command("java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-jar", jar}, append(args, "M.tla")...)...)
		cmd.Dir = dir
		output, _ := javatmp.Apply(cmd).CombinedOutput()
		if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code || !reflect.DeepEqual(pattern.FindAllString(string(output), -1), got) {
			t.Fatalf("Java exit %v, output %s", cmd.ProcessState, output)
		}
	}
}

func TestJavaTLCSimulationExitVersionDifference(t *testing.T) {
	dir := t.TempDir()
	body := `VARIABLE x
Init == x=0
Next == /\ TLCSet("exit", TRUE) /\ x'=1`
	writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\n")
	args := []string{"-tool", "-workers", "1", "-seed", "42", "-simulate", "num=3", "-depth", "4"}
	var out, errout bytes.Buffer
	code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
	if code != 0 || errout.Len() != 0 {
		t.Fatalf("Go exit %d, stderr %s, stdout %s", code, errout.String(), out.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		cmd := exec.Command("java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-jar", jar}, append(args, "M.tla")...)...)
		cmd.Dir = dir
		output, _ := javatmp.Apply(cmd).CombinedOutput()
		if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 255 || !strings.Contains(string(output), "TLCGlobals.mainChecker") {
			t.Fatalf("Java 1.7.4 exit %v, output %s", cmd.ProcessState, output)
		}
	}
}
