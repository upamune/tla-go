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
)

func TestJavaTLCSimulationAril(t *testing.T) {
	cases := []struct {
		seed, aril int64
		code       int
	}{
		{1, -2, 0},
		{1, 0, 0},
		{1, 1, 0},
		{1, 2, 12},
		{1, 3, 12},
		{1, 17, 12},
		{1, 100, 12},
		{1, 1000000, 12},
		{-42, -2, 12},
		{-42, 0, 12},
		{-42, 1, 12},
		{-42, 2, 12},
		{-42, 3, 0},
		{-42, 17, 0},
		{-42, 100, 0},
		{-42, 1000000, 12},
	}
	body := "VARIABLE x\nInit == x=0\nA == /\\ x=0 /\\ x'=1\nB == /\\ x=0 /\\ x'=2\nStay == /\\ x>0 /\\ UNCHANGED x\nNext == A \\/ B \\/ Stay\nInv == x#2"
	for _, tc := range cases {
		t.Run(fmt.Sprintf("seed-%d/aril-%d", tc.seed, tc.aril), func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nINVARIANT Inv\n")
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-seed", fmt.Sprint(tc.seed), "-aril", fmt.Sprint(tc.aril), "-simulate", "num=1", "-depth", "5"}
			var out, errout bytes.Buffer
			code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
			if code != tc.code || errout.Len() != 0 {
				t.Fatalf("Go exit %d, stderr %s, stdout %s", code, errout.String(), out.String())
			}
			if code != 0 {
				want := fmt.Sprintf("Simulation using seed %d and aril %d", tc.seed, max(tc.aril, 0))
				if !strings.Contains(out.String(), want) {
					t.Fatalf("missing %q: %s", want, out.String())
				}
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				javaArgs := append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-jar", jar}, append(args, "M.tla")...)
				cmd := exec.Command("java", javaArgs...)
				cmd.Dir = dir
				output, _ := cmd.CombinedOutput()
				if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code {
					t.Fatalf("Java exit %v, output %s", cmd.ProcessState, output)
				}
				if code != 0 {
					summary := fmt.Sprintf("Simulation using seed %d and aril %d", tc.seed, max(tc.aril, 0))
					if !strings.Contains(string(output), summary) {
						t.Fatalf("Java missing %q: %s", summary, output)
					}
				}
				got := simulationMessages(readToolMessages(out.String()))
				want := simulationMessages(readToolMessages(string(output)))
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("Go messages %q\nJava messages %q", got, want)
				}
			}
		})
	}
}

func TestJavaTLCModernSimulationAril(t *testing.T) {
	jar := os.Getenv("TLC_MODERN_JAR")
	if jar == "" {
		t.Skip("TLC_MODERN_JAR is not set")
	}
	for _, tc := range []struct{ seed, aril, code int }{{1, 0, 0}, {1, 2, 12}, {-42, 3, 0}, {-42, 2, 12}} {
		t.Run(fmt.Sprintf("%d/%d", tc.seed, tc.aril), func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == x=0\nA == /\\ x=0 /\\ x'=1\nB == /\\ x=0 /\\ x'=2\nStay == /\\ x>0 /\\ UNCHANGED x\nNext == A \\/ B \\/ Stay\nInv == x#2"
			writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nINVARIANT Inv\n")
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-simulate", "num=1", "-seed", fmt.Sprint(tc.seed), "-aril", fmt.Sprint(tc.aril), "-depth", "5"}
			var out, errout bytes.Buffer
			code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
			cmd := exec.Command("java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-jar", jar}, append(args, "M.tla")...)...)
			cmd.Dir = dir
			output, _ := cmd.CombinedOutput()
			if code != tc.code || errout.Len() != 0 || cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code {
				t.Fatalf("Go exit %d, Java %v, stderr %s; Go %s; Java %s", code, cmd.ProcessState, errout.String(), out.String(), output)
			}
			states := regexp.MustCompile(`(?m)^x = [0-9]+$`)
			if got, want := states.FindAllString(out.String(), -1), states.FindAllString(string(output), -1); !reflect.DeepEqual(got, want) {
				t.Fatalf("Go states %q; Java states %q", got, want)
			}
		})
	}
}

func TestJavaTLCSimulationContinueAril(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x=0\nNext == x'=1\nInv == x=0", "INIT Init\nNEXT Next\nINVARIANT Inv\n")
	args := []string{"-tool", "-workers", "1", "-fp", "0", "-continue", "-simulate", "num=3", "-seed", "1", "-aril", "17", "-depth", "5"}
	var out, errout bytes.Buffer
	code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
	if code != 255 || errout.Len() != 0 || strings.Count(out.String(), "Simulation using seed 1 and aril 17") != 3 {
		t.Fatalf("Go exit %d, stderr %s, stdout %s", code, errout.String(), out.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		got := continueMessages(readToolMessages(out.String()))
		want := javaContinueMessages(t, jar, dir, args, code)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Go messages %q\nJava messages %q", got, want)
		}
	}
}

func TestJavaTLCSimulationArilLiveness(t *testing.T) {
	for _, aril := range []string{"0", "2", "17"} {
		t.Run(aril, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == x=0\nA == /\\ x=0 /\\ x'=1\nB == /\\ x=0 /\\ x'=2\nStay == /\\ x>0 /\\ UNCHANGED x\nNext == A \\/ B \\/ Stay\nP == []<>(x=1)"
			writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nPROPERTY P\n")
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-simulate", "num=1", "-seed", "1", "-aril", aril, "-depth", "5"}
			var out, errout bytes.Buffer
			code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
			if code != 13 || errout.Len() != 0 {
				t.Fatalf("Go exit %d, stderr %s, stdout %s", code, errout.String(), out.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				got := continueMessages(readToolMessages(out.String()))
				want := javaContinueMessages(t, jar, dir, args, code)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("Go messages %q\nJava messages %q", got, want)
				}
			}
		})
	}
}

func TestJavaTLCSimulationArilWithoutSeed(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x=0\nNext == x'=1\nInv == x=0", "INIT Init\nNEXT Next\nINVARIANT Inv\n")
	cmd := exec.Command("java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-jar", jar, "-tool", "-workers", "1", "-simulate", "num=1", "-aril", "17", "-depth", "1", "M.tla")
	cmd.Dir = dir
	output, _ := cmd.CombinedOutput()
	if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 12 || !strings.Contains(string(output), "and aril 0") {
		t.Fatalf("Java exit %v, output %s", cmd.ProcessState, output)
	}
}

func TestJavaTLCModelCheckingIgnoresAril(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x=0\nNext == x' \\in {0,1}\nInv == x=0", "INIT Init\nNEXT Next\nINVARIANT Inv\n")
	var baseline []string
	for _, aril := range []string{"0", "17", "9223372036854775807"} {
		args := []string{"-tool", "-workers", "1", "-fp", "0", "-seed", "1", "-aril", aril}
		var out, errout bytes.Buffer
		code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
		if code != 12 || errout.Len() != 0 {
			t.Fatalf("aril %s: Go exit %d, stderr %s, stdout %s", aril, code, errout.String(), out.String())
		}
		got := modelContinueMessages(readToolMessages(out.String()))
		if baseline == nil {
			baseline = got
		} else if !reflect.DeepEqual(got, baseline) {
			t.Fatalf("aril %s: messages %q; baseline %q", aril, got, baseline)
		}
		if jar := os.Getenv("TLC_JAR"); jar != "" {
			want := javaModelContinueMessages(t, jar, dir, args, code)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("aril %s: Go %q; Java %q", aril, got, want)
			}
		}
	}
}
