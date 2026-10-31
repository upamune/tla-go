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
	// skipPositions leaves out the expressions listed by TLC's 2103 message.
	// TLC lists every conjunct still on its continuation-passing call stack
	// and, after an error in Init, the identity of its Tool object; tla-go
	// lists only the expressions enclosing the error.
	skipPositions bool
	// modules are other modules of the case, by name.
	modules map[string]string
}

const depthFirstInner = "---- MODULE Inner ----\nEXTENDS Naturals\nVARIABLE y\nN == y' = y + 1 \\/ y' = y + 2 \\/ y' = y + 3\n===="

// The upstream JUnit tests DepthFirstDieHardTest, DepthFirstErrorTraceTest,
// and DepthFirstTerminate, followed by small models whose Java TLC output does
// not depend on its random choice of states.
var depthFirstCases = []depthFirstCase{
	{name: "DepthFirstDieHardTest", spec: "DieHard", args: []string{"-deadlock", "-dfid", "7"}, code: 12, counts: "876 states generated, 68 distinct states found.", states: 7},
	{name: "DepthFirstErrorTraceTest", spec: "DepthFirstErrorTrace", args: []string{"-deadlock", "-dfid", "9"}, code: 12, counts: "29 states generated, 8 distinct states found.", states: 8},
	{name: "DepthFirstTerminate", spec: "DepthFirstTerminate", args: []string{"-deadlock", "-dfid", "50"}, code: 0, counts: "12514 states generated, 349 distinct states found."},
	{name: "multiple workers", spec: "DepthFirstTerminate", args: []string{"-deadlock", "-workers", "4", "-dfid", "50"}, code: 255},
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
	{name: "multiple bounds", body: "Init == x=0\nNext == \\E i \\in 1..2, j \\in 1..2 : x' = x + 10*i + j\nInv == x \\notin {12, 21}", cfg: "INVARIANT Inv\n", args: []string{"-deadlock", "-dfid", "5"}, code: 12, counts: "3 states generated, 3 distinct states found.", states: 2},
	{name: "repeated bound", body: "Init == x=0\nNext == \\E i, j \\in 1..2 : x' = x + 10*i + j\nInv == x # 21", cfg: "INVARIANT Inv\n", args: []string{"-deadlock", "-dfid", "5"}, code: 12, counts: "3 states generated, 3 distinct states found.", states: 2},
	{name: "instance substitution", body: "I == INSTANCE Inner WITH y <- x\nInit == x=0\nNext == I!N\nInv == x # 2", cfg: "INVARIANT Inv\n", args: []string{"-deadlock", "-dfid", "5"}, code: 12, counts: "4 states generated, 3 distinct states found.", states: 2, modules: map[string]string{"Inner": depthFirstInner}},
	{name: "next-state error", body: "Init == x=0\nNext == x' = IF x = 2 THEN 1 \\div 0 ELSE x + 1", args: []string{"-dfid", "10"}, code: 255, counts: "6 states generated, 3 distinct states found.", states: 3},
	{name: "constraint error", body: "Init == x=0\nNext == x<5 /\\ x'=x+1\nC == IF x=3 THEN 1 \\div 0 = 0 ELSE TRUE", cfg: "CONSTRAINT C\n", args: []string{"-dfid", "10"}, code: 255, counts: "8 states generated, 3 distinct states found.", states: 4},
	{name: "action constraint error", body: "Init == x=0\nNext == x<5 /\\ x'=x+1\nC == IF x'=3 THEN 1 \\div 0 = 0 ELSE TRUE", cfg: "ACTION_CONSTRAINT C\n", args: []string{"-dfid", "10"}, code: 255, counts: "8 states generated, 3 distinct states found.", states: 4},
	{name: "assert", body: "Init == x=0\nNext == x<5 /\\ x'=x+1 /\\ Assert(x#2, \"boom\")", args: []string{"-dfid", "10"}, code: 255, counts: "6 states generated, 3 distinct states found.", states: 3, skipPositions: true},
	{name: "non-boolean next", body: "Init == x=0\nNext == x<5 /\\ x'=x+1 /\\ (IF x=2 THEN 7 ELSE TRUE)", args: []string{"-dfid", "10"}, code: 255, counts: "6 states generated, 3 distinct states found.", states: 3, skipPositions: true},
	{name: "exit", body: "Init == x=0\nNext == x<5 /\\ x'=x+1 /\\ (x=2 => TLCSet(\"exit\", TRUE))", args: []string{"-dfid", "10"}, code: 255, counts: "6 states generated, 3 distinct states found.", states: 3, skipPositions: true},
	{name: "initial error", body: "Init == x = 1 \\div 0\nNext == x'=x", args: []string{"-dfid", "10"}, code: 75, counts: "0 states generated, 0 distinct states found.", skipPositions: true},
	{name: "initial invariant error", body: "Init == x \\in {0, 1}\nNext == UNCHANGED x\nInv == 1 \\div x = 1", cfg: "INVARIANT Inv\n", args: []string{"-dfid", "10"}, code: 0, counts: "2 states generated, 2 distinct states found."},
	{name: "liveness", body: "Init == x=0\nNext == x'=(x+1)%4\nP == <>(x=3)", cfg: "PROPERTY P\n", args: []string{"-dfid", "10"}, code: 255},
}

func TestJavaTLCDepthFirst(t *testing.T) {
	for _, tc := range depthFirstCases {
		t.Run(tc.name, func(t *testing.T) {
			dir, spec := writeDepthFirstCase(t, tc)
			args := append([]string{"-tool", "-workers", "1", "-fp", "0"}, tc.args...)
			var out, errout bytes.Buffer
			code := run(context.Background(), append(args, filepath.Join(dir, spec+".tla")), &out, &errout)
			got := depthFirstMessages(readToolMessages(out.String()), tc.skipPositions)
			if code != tc.code || errout.Len() != 0 || !strings.Contains(out.String(), tc.counts) {
				t.Fatalf("exit %d, want %d; stdout %s; stderr %s", code, tc.code, out.String(), errout.String())
			}
			if states := strings.Count(out.String(), "@!@!@STARTMSG 2216:4 @!@!@"); states != tc.states {
				t.Fatalf("%d trace states, want %d: %s", states, tc.states, out.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				want := javaDepthFirstMessages(t, jar, dir, append(args, spec+".tla"), tc)
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
	for name, source := range tc.modules {
		if err := os.WriteFile(filepath.Join(dir, name+".tla"), []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
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
func depthFirstMessages(messages []toolMessage, skipPositions bool) []string {
	var result []string
	for _, message := range messages {
		switch message.code {
		case 2262, 2271, 2220, 2219, 2185, 2186, 2401, 2212:
			continue
		}
		body := depthFirstProgressTime.ReplaceAllString(message.body, "Progress(-1): ")
		if message.code == 2103 && skipPositions {
			body, _, _ = strings.Cut(body, "positions:\n")
		}
		result = append(result, fmt.Sprintf("%d:%d:%s", message.code, message.level, body))
	}
	return result
}

func javaDepthFirstMessages(t *testing.T, jar, dir string, args []string, tc depthFirstCase) []string {
	t.Helper()
	cmd := exec.Command("java", append([]string{"-Djava.io.tmpdir=" + t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar}, args...)...)
	cmd.Dir = dir
	output, _ := cmd.CombinedOutput()
	if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != tc.code {
		t.Fatalf("Java exit %v, output %s", cmd.ProcessState, output)
	}
	return depthFirstMessages(readToolMessages(string(output)), tc.skipPositions)
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
