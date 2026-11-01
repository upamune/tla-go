package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/upamune/tla-go/internal/javatmp"
)

func TestJavaTLCSimulationContinue(t *testing.T) {
	for _, tc := range []struct {
		name, body, cfg string
		code, reports   int
	}{
		{"invariant", "Init == x=0\nNext == x'=1\nInv == x=0", "INVARIANT Inv", 255, 3},
		{"mixed successful and violating traces", "Init == x=0\nA == /\\ x=0 /\\ x'=1\nB == /\\ x=0 /\\ x'=2\nNext == A \\/ B\nInv == x # 1", "INVARIANT Inv\nCHECK_DEADLOCK FALSE", 255, 2},
		{"invariant after stuttering", "Init == x=0\nNext == UNCHANGED x\nInv == TLCGet(\"level\")=0", "INVARIANT Inv", 255, 3},
		{"action", "Init == x=0\nNext == x'=1\nP == [][x'=x]_x", "PROPERTY P", 255, 3},
		{"deadlock", "Init == x=0\nNext == /\\ x=0 /\\ x'=1", "", 255, 3},
		{"success", "Init == x=0\nNext == UNCHANGED x", "", 0, 0},
		{"initial violation", "Init == x=0\nNext == x'=1\nInv == x=1", "INVARIANT Inv", 12, 0},
		{"invariant evaluation", "Init == x=0\nNext == x'=1\nInv == IF x=0 THEN TRUE ELSE 0=\"a\"", "INVARIANT Inv", 76, 0},
		{"action evaluation", "Init == x=0\nNext == x'=1\nP == [][IF x=0 THEN TRUE ELSE 0=\"a\"]_x", "PROPERTY P", 13, 0},
		{"liveness", "Init == x=0\nNext == UNCHANGED x\nP == <>(x=1)", "PROPERTY P", 13, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, "VARIABLE x\n"+tc.body, "INIT Init\nNEXT Next\n"+tc.cfg+"\n")
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-continue", "-simulate", "num=3", "-seed", "1", "-depth", "5"}
			var out, errout bytes.Buffer
			code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
			if code != tc.code || errout.Len() != 0 {
				t.Fatalf("exit %d; stdout %s; stderr %s", code, out.String(), errout.String())
			}
			if tc.reports > 0 && strings.Count(out.String(), "@!@!@STARTMSG 2210:") != tc.reports {
				t.Fatalf("expected %d reports; stdout %s", tc.reports, out.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				want := javaContinueMessages(t, jar, dir, args, tc.code)
				got := continueMessages(readToolMessages(out.String()))
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("Go messages %q\nJava messages %q", got, want)
				}
			}
		})
	}
}

func javaContinueMessages(t *testing.T, jar, dir string, args []string, code int) []string {
	t.Helper()
	javaArgs := append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-jar", jar}, append(args, "M.tla")...)
	cmd := exec.Command("java", javaArgs...)
	cmd.Dir = dir
	output, _ := javatmp.Apply(cmd).CombinedOutput()
	if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code {
		t.Fatalf("Java exit %v, output %s", cmd.ProcessState, output)
	}
	return continueMessages(readToolMessages(string(output)))
}

func TestJavaTLCSimulationContinueThenEvaluationError(t *testing.T) {
	dir := t.TempDir()
	body := "VARIABLE x\nInit == x=0\nA == x'=1\nB == x'=2\nNext == A \\/ B\nInv == IF x=0 THEN TRUE ELSE IF x=1 THEN FALSE ELSE 0=\"a\""
	writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nINVARIANT Inv\n")
	args := []string{"-tool", "-workers", "1", "-fp", "0", "-continue", "-simulate", "num=3", "-seed", "2", "-depth", "5"}
	var out, errout bytes.Buffer
	code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
	if code != 76 || errout.Len() != 0 || strings.Count(out.String(), "@!@!@STARTMSG 2110:") != 2 {
		t.Fatalf("exit %d; stdout %s; stderr %s", code, out.String(), errout.String())
	}
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		got := continueMessages(readToolMessages(out.String()))
		want := javaContinueMessages(t, jar, dir, args, code)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Go messages %q\nJava messages %q", got, want)
		}
	}
}

func TestJavaTLCSimulationContinueUpstream(t *testing.T) {
	const sourceDir = "testdata/java-tlc/multiple-config/multi-trace"
	source, err := os.ReadFile(filepath.Join(sourceDir, "BasicMultiTrace.tla"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		code int
	}{
		{"MCInv", 255},
		{"MCActionProp", 255},
		{"MCInvInitState", 12},
		{"MCBadInvNonInitState", 76},
		{"MCLivenessProp", 13},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := os.ReadFile(filepath.Join(sourceDir, tc.name+".cfg"))
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			module := strings.Replace(string(source), "MODULE BasicMultiTrace", "MODULE M", 1)
			for name, content := range map[string]string{"M.tla": module, "M.cfg": string(cfg) + "\nCHECK_DEADLOCK FALSE\n"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-continue", "-simulate", "num=3", "-seed", "1", "-depth", "5"}
			var out, errout bytes.Buffer
			code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
			if code != tc.code || errout.Len() != 0 {
				t.Fatalf("exit %d; stdout %s; stderr %s", code, out.String(), errout.String())
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

// Progress counts are scheduling-dependent: Java's worker can run ahead of
// the thread printing previous violations. Compare the semantic messages.
func continueMessages(messages []toolMessage) []string {
	var result []string
	for _, msg := range messages {
		switch msg.code {
		case 2107, 2110, 2111, 2112, 2113, 2114, 2116, 2121, 2122, 2217, 2218, 2269:
			result = append(result, fmt.Sprintf("%d:%s", msg.code, msg.body))
		}
	}
	return result
}
