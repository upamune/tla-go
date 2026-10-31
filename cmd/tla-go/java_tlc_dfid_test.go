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

type depthFirstCase struct {
	name, spec, body, cfg string
	args                  []string
	code                  int
	counts                string
	states                int
}

// The upstream JUnit tests DepthFirstDieHardTest, DepthFirstErrorTraceTest,
// and DepthFirstTerminate, followed by small models whose Java TLC output does
// not depend on its random choice of states.
var depthFirstCases = []depthFirstCase{
	{name: "DepthFirstDieHardTest", spec: "DieHard", args: []string{"-deadlock", "-dfid", "7"}, code: 12, counts: "876 states generated, 68 distinct states found.", states: 7},
	{name: "DepthFirstErrorTraceTest", spec: "DepthFirstErrorTrace", args: []string{"-deadlock", "-dfid", "9"}, code: 12, counts: "29 states generated, 8 distinct states found.", states: 8},
	{name: "DepthFirstTerminate", spec: "DepthFirstTerminate", args: []string{"-deadlock", "-dfid", "50"}, code: 0, counts: "12514 states generated, 349 distinct states found."},
	{name: "deadlock", body: "Init == x=0\nNext == x<5 /\\ x'=x+1", args: []string{"-dfid", "10"}, code: 11, counts: "21 states generated, 6 distinct states found.", states: 6},
	{name: "depth limit", body: "Init == x=0\nNext == x<5 /\\ x'=x+1", args: []string{"-dfid", "6"}, code: 0, counts: "16 states generated, 6 distinct states found."},
	{name: "depth below two", body: "Init == x=0\nNext == x<5 /\\ x'=x+1", args: []string{"-dfid", "1"}, code: 255, counts: "1 states generated, 1 distinct states found."},
	{name: "initial invariant", body: "Init == x \\in 0..2\nNext == x'=x+1\nInv == x<2", cfg: "INVARIANT Inv\n", args: []string{"-dfid", "5"}, code: 12},
	{name: "action property", body: "Init == x=0\nNext == x'=(x+1)%4\nP == [][x'>x]_x", cfg: "PROPERTY P\n", args: []string{"-dfid", "10"}, code: 13, counts: "11 states generated, 4 distinct states found.", states: 5},
	{name: "action successors", body: "Init == x=0\nNext == x' \\in {1,2,3}\nInv == x#2", cfg: "INVARIANT Inv\n", args: []string{"-deadlock", "-dfid", "5"}, code: 12, counts: "4 states generated, 3 distinct states found.", states: 2},
	{name: "split actions", body: "Init == x=0\nNext == x'=1 \\/ x'=2 \\/ x'=3\nInv == x#2", cfg: "INVARIANT Inv\n", args: []string{"-deadlock", "-dfid", "5"}, code: 12, counts: "3 states generated, 3 distinct states found.", states: 2},
	{name: "operator actions", body: "Init == x=0\nA(i) == x'=x+i\nNext == A(1) \\/ A(2)\nInv == x#1", cfg: "INVARIANT Inv\n", args: []string{"-deadlock", "-dfid", "5"}, code: 12, counts: "2 states generated, 2 distinct states found.", states: 2},
	{name: "state-dependent existential", body: "Init == x=0\nNext == \\E i \\in 1..(x+2) : x'=x+i\nInv == x#1", cfg: "INVARIANT Inv\n", args: []string{"-deadlock", "-dfid", "5"}, code: 12, counts: "3 states generated, 2 distinct states found.", states: 2},
	{name: "incomplete successor", body: "VARIABLE y\nInit == x=0 /\\ y=0\nNext == (x<2 /\\ x'=x+1 /\\ y'=y) \\/ (x=2 /\\ x'=3)", args: []string{"-dfid", "10"}, code: 75, counts: "7 states generated, 3 distinct states found.", states: 4},
	{name: "constraints", body: "VARIABLE y\nInit == x \\in 0..2 /\\ y=0\nNext == (x'=x+1 /\\ y'=y) \\/ (y'=y+1 /\\ x'=x)\nC == x+y<4\nA == ~(x'=2 /\\ y'=1)", cfg: "CONSTRAINT C\nACTION_CONSTRAINT A\n", args: []string{"-deadlock", "-dfid", "20"}, code: 0, counts: "47 states generated, 9 distinct states found."},
	{name: "continue", body: "VARIABLE y\nInit == x \\in 0..2 /\\ y=0\nNext == (x'=x+1 /\\ y'=y) \\/ (y'=y+1 /\\ x'=x)\nC == x+y<4\nInv == ~(x=1 /\\ y=2)", cfg: "CONSTRAINT C\nINVARIANT Inv\n", args: []string{"-deadlock", "-continue", "-dfid", "20"}, code: 0, counts: "49 states generated, 10 distinct states found.", states: 3},
	{name: "invariant failure", body: "Init == x=0\nNext == x<5 /\\ x'=x+1\nInv == IF x=2 THEN 1 \\div 0 = 0 ELSE TRUE", cfg: "INVARIANT Inv\n", args: []string{"-dfid", "10"}, code: 76, counts: "5 states generated, 3 distinct states found.", states: 3},
	{name: "liveness", body: "Init == x=0\nNext == x'=(x+1)%4\nP == <>(x=3)", cfg: "PROPERTY P\n", args: []string{"-dfid", "10"}, code: 255},
}

func TestJavaTLCDepthFirst(t *testing.T) {
	for _, tc := range depthFirstCases {
		t.Run(tc.name, func(t *testing.T) {
			dir, spec := writeDepthFirstCase(t, tc)
			args := append([]string{"-tool", "-workers", "1", "-fp", "0"}, tc.args...)
			var out, errout bytes.Buffer
			code := run(context.Background(), append(args, filepath.Join(dir, spec+".tla")), &out, &errout)
			got := depthFirstMessages(readToolMessages(out.String()))
			if code != tc.code || errout.Len() != 0 || !strings.Contains(out.String(), tc.counts) {
				t.Fatalf("exit %d, want %d; stdout %s; stderr %s", code, tc.code, out.String(), errout.String())
			}
			if states := strings.Count(out.String(), "@!@!@STARTMSG 2216:4 @!@!@"); states != tc.states {
				t.Fatalf("%d trace states, want %d: %s", states, tc.states, out.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				want := javaDepthFirstMessages(t, jar, dir, append(args, spec+".tla"), tc.code)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("Go messages %q\nJava messages %q", got, want)
				}
			}
		})
	}
}

func writeDepthFirstCase(t *testing.T, tc depthFirstCase) (string, string) {
	t.Helper()
	dir := t.TempDir()
	if tc.spec == "" {
		writeDOTFixture(t, dir, "VARIABLE x\n"+tc.body, "INIT Init\nNEXT Next\n"+tc.cfg)
		return dir, "M"
	}
	for _, ext := range []string{".tla", ".cfg"} {
		content, err := os.ReadFile(filepath.Join("testdata/java-tlc/dfid", tc.spec+ext))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, tc.spec+ext), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir, tc.spec
}

var depthFirstProgressTime = regexp.MustCompile(`^Progress\(-1\) at [^:]+:\d\d:\d\d: `)

// depthFirstMessages drops the banner, SANY, timing, and JVM messages that
// differ between implementations.
func depthFirstMessages(messages []toolMessage) []string {
	var result []string
	for _, message := range messages {
		switch message.code {
		case 2262, 2271, 2220, 2219, 2185, 2186, 2401, 2212:
			continue
		}
		body := depthFirstProgressTime.ReplaceAllString(message.body, "Progress(-1): ")
		result = append(result, fmt.Sprintf("%d:%d:%s", message.code, message.level, body))
	}
	return result
}

func javaDepthFirstMessages(t *testing.T, jar, dir string, args []string, code int) []string {
	t.Helper()
	cmd := exec.Command("java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar}, args...)...)
	cmd.Dir = dir
	output, _ := cmd.CombinedOutput()
	if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code {
		t.Fatalf("Java exit %v, output %s", cmd.ProcessState, output)
	}
	return depthFirstMessages(readToolMessages(string(output)))
}

func TestDepthFirstTextOutput(t *testing.T) {
	dir, spec := writeDepthFirstCase(t, depthFirstCases[1])
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"-deadlock", "-dfid", "9", filepath.Join(dir, spec+".tla")}, &out, &errout)
	want := "Checking DepthFirstErrorTrace.tla with DepthFirstErrorTrace.cfg\n" +
		"Running depth-first search Model-Checking with fp 0 and seed 0.\n" +
		"Finished computing initial states: 1 states generated, with 1 of them distinct.\n" +
		"Starting level 2: 1 states generated, 1 distinct states found.\n"
	if code != 12 || errout.Len() != 0 || !strings.HasPrefix(out.String(), want) {
		t.Fatalf("exit %d, stdout %s, stderr %s", code, out.String(), errout.String())
	}
	tail := "Error: Invariant Inv is violated.\nError: The behavior up to this point is:\nState 1:\nx = 0\n\n"
	if !strings.Contains(out.String(), tail) || !strings.HasSuffix(out.String(), "State 8:\nx = 7\n\n29 states generated, 8 distinct states found.\n") {
		t.Fatalf("stdout %s", out.String())
	}
	dir, spec = writeDepthFirstCase(t, depthFirstCases[2])
	out.Reset()
	code = run(context.Background(), []string{"-dfid", "50", filepath.Join(dir, spec+".tla")}, &out, &errout)
	success := "  calculated (optimistic):  val = 2.3E-13\n  based on the actual fingerprints:  val = 1.3E-5\n12514 states generated, 349 distinct states found.\n"
	if code != 0 || !strings.HasSuffix(out.String(), success) {
		t.Fatalf("exit %d, stdout %s", code, out.String())
	}
}

func TestDepthFirstOptions(t *testing.T) {
	for _, tc := range []struct {
		args    []string
		code    int
		message string
	}{
		{[]string{"-dfid"}, 1, "Error: expect a nonnegative integer for -dfid option."},
		{[]string{"-dfid", "-1", "M"}, 1, "Error: expect a nonnegative integer for -dfid option."},
		{[]string{"-dfid", "deep", "M"}, 1, "Errorexpect a nonnegative integer for -dfid option. But encountered deep"},
		{[]string{"-dfid", "3", "-dump", "dot,actionlabels", "g.dot", "M"}, 255, "-dump is not supported in depth-first mode"},
	} {
		var out, errout bytes.Buffer
		code := run(context.Background(), tc.args, &out, &errout)
		if code != tc.code || !strings.HasPrefix(errout.String(), tc.message+"\n") {
			t.Errorf("%q: exit %d, stderr %q", tc.args, code, errout.String())
		}
	}
}

func TestObservedProbability(t *testing.T) {
	for _, tc := range []struct {
		distance int64
		want     string
	}{
		{1, "1.0"},
		{3, ".33"},
		{65 << 10, "1.5E-5"},
		{1 << 62, "2.2E-19"},
		{9223372036854775807, "1.1E-19"},
		{40, ".025"},
		{1995, "5.0E-4"},
	} {
		if got := formatProbability(observedProbability(tc.distance)); got != tc.want {
			t.Errorf("distance %d: %s, want %s", tc.distance, got, tc.want)
		}
	}
}
