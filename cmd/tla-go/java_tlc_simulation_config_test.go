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
	"time"
)

func TestJavaTLCModernSimulationConfig(t *testing.T) {
	for _, tc := range []struct {
		name          string
		args          []string
		cfg           string
		code          int
		traces, depth int
		deadlock      string
	}{
		{"bounded", []string{"-simulate", "num=2", "-depth", "2", "-seed", "42", "-aril", "17", "-deadlock"}, "", 0, 2, 2, "FALSE"},
		{"zero count and depth", []string{"-simulate", "num=0", "-depth", "0", "-seed", "42"}, "", 0, 0, 0, "TRUE"},
		{"negative count", []string{"-simulate", "num=-2", "-depth", "1", "-seed", "42"}, "", 0, -2, 1, "TRUE"},
		{"explicit negative one", []string{"-simulate", "num=-1", "-depth", "1", "-seed", "42"}, "", 0, -1, 1, "TRUE"},
		{"unlimited depth", []string{"-simulate", "num=1", "-depth", "-1", "-seed", "42", "-deadlock"}, "", 0, 1, -1, "FALSE"},
		{"maximum depth", []string{"-simulate", "num=1", "-depth", "2147483647", "-seed", "42", "-deadlock"}, "", 0, 1, -1, "FALSE"},
		{"default trace count", []string{"-simulate", "-depth", "1", "-seed", "42"}, "INVARIANT Stop\n", 12, -1, 1, "TRUE"},
		{"maximum trace count", []string{"-simulate", "num=2147483647", "-depth", "0", "-seed", "42"}, "INVARIANT Stop\n", 12, 2147483647, 0, "TRUE"},
		{"default depth", []string{"-simulate", "num=1", "-seed", "42"}, "INVARIANT Stop\n", 12, 1, 100, "TRUE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := `VARIABLE x
C == TLCGet("config")
Fields == DOMAIN C = {"install", "mode", "deadlock", "seed", "worker", "traces", "depth", "aril", "sched"}
Observe(tag) == PrintT(<<tag, C.mode, C.deadlock, C.worker, C.seed, C.traces, C.depth, C.aril, C.sched>>)
ASSUME Observe("assume")
Init == /\ Fields /\ Observe("init") /\ x=0
Next == /\ Observe("next") /\ x=0 /\ x'=1
Stop == x=1`
			writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\n"+tc.cfg)
			args := append([]string{"-tool", "-workers", "1", "-fp", "0"}, tc.args...)
			var out, errout bytes.Buffer
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			code := run(ctx, append(args, filepath.Join(dir, "M.tla")), &out, &errout)
			if code != tc.code || errout.Len() != 0 {
				t.Fatalf("Go exit %d, stderr %s, stdout %s", code, errout.String(), out.String())
			}
			pattern := regexp.MustCompile(`(?m)^<<"(?:assume|init|next)", .+>>$`)
			got := pattern.FindAllString(out.String(), -1)
			assumption := fmt.Sprintf(`<<"assume", "simulate", %s, 1, "42", %d, %d, "0", "random">>`, tc.deadlock, tc.traces, tc.depth)
			if len(got) < 2 || got[0] != assumption || !strings.HasPrefix(got[1], `<<"init",`) {
				t.Fatalf("Go config %q; want %s followed by init; stdout %s", got, assumption, out.String())
			}
			if jar := os.Getenv("TLC_MODERN_JAR"); jar != "" {
				cmd := exec.Command("java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-jar", jar}, append(args, "M.tla")...)...)
				cmd.Dir = dir
				output, _ := cmd.CombinedOutput()
				if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code {
					t.Fatalf("Java exit %v, output %s", cmd.ProcessState, output)
				}
				want := pattern.FindAllString(string(output), -1)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("Go config %q\nJava config %q", got, want)
				}
			}
		})
	}
}

func TestJavaTLCSimulationConfigVersionDifference(t *testing.T) {
	dir := t.TempDir()
	body := "VARIABLE x\nInit == x=0\nNext == /\\ PrintT(TLCGet(\"config\").mode) /\\ UNCHANGED x"
	writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\n")
	args := []string{"-tool", "-workers", "1", "-seed", "42", "-simulate", "num=1", "-depth", "1"}
	var out, errout bytes.Buffer
	code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
	if code != 0 || errout.Len() != 0 || !strings.Contains(out.String(), `"simulate"`) {
		t.Fatalf("Go exit %d, stderr %s, stdout %s", code, errout.String(), out.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		cmd := exec.Command("java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-jar", jar}, append(args, "M.tla")...)...)
		cmd.Dir = dir
		output, _ := cmd.CombinedOutput()
		if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 255 || !strings.Contains(string(output), "TLCGet(config) was undefined.") {
			t.Fatalf("Java 1.7.4 exit %v, output %s", cmd.ProcessState, output)
		}
	}
}

func TestJavaTLCModernSimulationConfigRecord(t *testing.T) {
	dir := t.TempDir()
	body := "VARIABLE x\nInit == /\\ PrintT(TLCGet(\"config\")) /\\ x=0\nNext == x'=1"
	writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\n")
	args := []string{"-tool", "-workers", "1", "-seed", "42", "-simulate", "num=1", "-depth", "0"}
	var out, errout bytes.Buffer
	code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
	if code != 0 || errout.Len() != 0 {
		t.Fatalf("Go exit %d, stderr %s, stdout %s", code, errout.String(), out.String())
	}
	pattern := regexp.MustCompile(`(?ms)^\[ install \|-> .*?sched \|-> "random" \]`)
	got := pattern.FindString(out.String())
	want := "[ install |-> \"unknown\",\n  mode |-> \"simulate\",\n  deadlock |-> TRUE,\n  seed |-> \"42\",\n  worker |-> 1,\n  traces |-> 1,\n  depth |-> 0,\n  aril |-> \"0\",\n  sched |-> \"random\" ]"
	if got != want {
		t.Fatalf("Go record %q; want %q", got, want)
	}
	if jar := os.Getenv("TLC_MODERN_JAR"); jar != "" {
		cmd := exec.Command("java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-jar", jar}, append(args, "M.tla")...)...)
		cmd.Dir = dir
		output, _ := cmd.CombinedOutput()
		if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code {
			t.Fatalf("Java exit %v, output %s", cmd.ProcessState, output)
		}
		javaRecord := pattern.FindString(string(output))
		install := regexp.MustCompile(`(?m)^\[ install \|-> "[^"]*"`)
		javaRecord = install.ReplaceAllString(javaRecord, `[ install |-> "unknown"`)
		if got != javaRecord {
			t.Fatalf("Go record %q\nJava record %q", got, javaRecord)
		}
	}
}
