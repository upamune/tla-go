package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Do not emit final progress for effectful fairness models until graph counts
// and the full output sequence are reproduced.
func TestPinnedJavaFairnessProgressEffects(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	data, err := os.ReadFile(jar)
	if err != nil || fmt.Sprintf("%x", sha256.Sum256(data)) != "936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88" {
		t.Fatalf("TLC_JAR must be the pinned 1.7.4 jar: %v", err)
	}
	for _, tc := range []struct {
		name, next, property string
		code                 int
		effectsMatch         bool
	}{
		{"weak nondeterministic success", `x' \in {0, 1} /\ PrintT("effect")`, `[]<>(x = 0)`, 0, true},
		{"strong nondeterministic violation", `x' \in {0, 1} /\ PrintT("effect")`, `[]<>(x = 2)`, 13, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			fair := "WF"
			if strings.HasPrefix(tc.name, "strong") {
				fair = "SF"
			}
			writeDOTFixture(t, dir, "VARIABLE x\nInit == x=0\nNext == "+tc.next+"\nSpec == Init /\\ [][Next]_x /\\ "+fair+"_x(Next)\nP == "+tc.property, "SPECIFICATION Spec\nPROPERTY P")
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final", "M.tla"}
			var out, errout bytes.Buffer
			code := run(context.Background(), append(append([]string{}, args[:len(args)-1]...), filepath.Join(dir, "M.tla")), &out, &errout)
			cmd := exec.Command("java", append([]string{"-jar", jar}, args...)...)
			cmd.Dir = dir
			java, err := cmd.CombinedOutput()
			if code != tc.code || errout.Len() != 0 || cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code {
				t.Fatalf("Go exit %d stderr %s; Java %v\n%s", code, &errout, err, java)
			}
			got, want := fairnessProgressStream(out.String()), fairnessProgressStream(string(java))
			goEffects, javaEffects := strings.Count(out.String(), `"effect"`), strings.Count(string(java), `"effect"`)
			if goEffects == 0 || (goEffects == javaEffects) != tc.effectsMatch || hasToolMessage(readToolMessages(out.String()), 2192) || !hasToolMessage(readToolMessages(string(java)), 2192) || !hasToolMessage(readToolMessages(out.String()), 2212) || !hasToolMessage(readToolMessages(string(java)), 2212) {
				t.Fatalf("Go %q Java %q\nGo %s\nJava %s", got, want, &out, java)
			}
		})
	}
}

func fairnessProgressStream(output string) []string {
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		if line == `"effect"` {
			lines[i] = "@!@!@STARTMSG 9999:0 @!@!@\neffect\n@!@!@ENDMSG 9999 @!@!@"
		}
	}
	return temporalProgressMessages(strings.Join(lines, "\n"))
}

func TestJavaTLCTemporalProgress(t *testing.T) {
	for _, tc := range []struct {
		name, body, cfg string
		code            int
	}{
		{"success", "Init == x=0\nNext == x'=(x+1)%3\nP == []<>(x>=0)", "PROPERTY P", 0},
		{"violation", "Init == x=0\nNext == x'=(x+1)%3\nP == []<>(x=4)", "PROPERTY P", 13},
		{"success post print", "Init == x=0\nNext == x'=(x+1)%3\nP == []<>(x>=0)\nPost == PrintT(TLCGet(\"distinct\"))", "PROPERTY P\nPOSTCONDITION Post", 0},
		{"violation post print", "Init == x=0\nNext == x'=(x+1)%3\nP == []<>(x=4)\nPost == PrintT(TLCGet(\"distinct\"))", "PROPERTY P\nPOSTCONDITION Post", 13},
		{"violation false post", "Init == x=0\nNext == x'=(x+1)%3\nP == []<>(x=4)\nPost == PrintT(TLCGet(\"distinct\")) /\\ FALSE", "PROPERTY P\nPOSTCONDITION Post", 13},
		{"violation post error", "Init == x=0\nNext == x'=(x+1)%3\nP == []<>(x=4)\nPost == PrintT(TLCGet(\"distinct\")) /\\ (1 \\div 0 = 1)", "PROPERTY P\nPOSTCONDITION Post", 13},
		{"false post", "Init == x=0\nNext == x'=(x+1)%3\nP == []<>(x>=0)\nPost == PrintT(TLCGet(\"distinct\")) /\\ FALSE", "PROPERTY P\nPOSTCONDITION Post", 10},
		{"post evaluation error", "Init == x=0\nNext == x'=(x+1)%3\nP == []<>(x>=0)\nPost == PrintT(TLCGet(\"distinct\")) /\\ (1 \\div 0 = 1)", "PROPERTY P\nPOSTCONDITION Post", 10},
		{"fairness", "Init == x=0\nNext == x'=(x+1)%3\nSpec == Init /\\ [][Next]_x /\\ WF_x(Next)\nP == []<>(x=1)", "SPECIFICATION Spec\nPROPERTY P", 0},
		{"view", "Init == x=0\nNext == x'=(x+1)%4\nV == x % 2\nP == []<>(x>=0)", "PROPERTY P\nVIEW V", 0},
		{"constraint", "Init == x=0\nNext == x'=(x+1)%4\nC == x<2\nP == []<>(x>=0)", "PROPERTY P\nCONSTRAINT C", 0},
		{"tableau violation", "Init == x=0\nNext == x'=(x+1)%3\nP == <>(x=4)", "PROPERTY P", 13},
		{"tableau success", "Init == x=0\nNext == x'=(x+1)%3\nP == <>(x=0)", "PROPERTY P", 0},
		{"tableau constraint", "Init == x=0\nNext == x'=(x+1)%4\nC == x<2\nP == <>(x=3)", "PROPERTY P\nCONSTRAINT C", 13},
		{"tableau action constraint", "Init == x=0\nNext == x'=(x+1)%4\nC == x'<2\nP == <>(x=3)", "PROPERTY P\nACTION_CONSTRAINT C", 13},
		{"tableau initial effect and constraints", "Init == /\\ x=0 /\\ PrintT(\"init-effect\")\nNext == x'=(x+1)%4\nC == x<2\nA == x'<2\nP == <>(x=3)", "PROPERTY P\nCONSTRAINT C\nACTION_CONSTRAINT A", 13},
		{"multiple branches", "Init == x=0\nNext == x'=(x+1)%3\nP == <>(x=1) /\\ <>(x=2)", "PROPERTY P", 13},
		{"assumption", "ASSUME FALSE\nInit == x=0\nNext == UNCHANGED x\nP == <>(x=1)", "PROPERTY P", 10},
		{"invariant", "Init == x=0\nNext == x'=x+1\nP == <>(x=3)\nI == x<2", "PROPERTY P\nINVARIANT I", 12},
		{"evaluation failure", "Init == x=0\nNext == x'=1 \\div x\nP == []<>(x=1)", "PROPERTY P", 255},
		{"empty", "Init == FALSE /\\ x=0\nNext == UNCHANGED x\nP == []<>(x=1)", "PROPERTY P", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			cfg := "INIT Init\nNEXT Next\n" + tc.cfg
			if strings.Contains(tc.cfg, "SPECIFICATION") {
				cfg = tc.cfg
			}
			writeDOTFixture(t, dir, "VARIABLE x\n"+tc.body, cfg)
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final", "M.tla"}
			var out, errout bytes.Buffer
			goArgs := append(append([]string{}, args[:len(args)-1]...), filepath.Join(dir, "M.tla"))
			code := run(context.Background(), goArgs, &out, &errout)
			if code != tc.code || errout.Len() != 0 {
				t.Fatalf("Go exit %d stderr %s output %s", code, &errout, &out)
			}
			jar := os.Getenv("TLC_JAR")
			if strings.Contains(tc.cfg, "POSTCONDITION") {
				jar = os.Getenv("TLC_MODERN_JAR")
			}
			if jar == "" {
				t.Skip("required TLC_JAR or TLC_MODERN_JAR is not set")
			}
			cmd := exec.Command("java", append([]string{"-jar", jar}, args...)...)
			cmd.Dir = dir
			java, err := cmd.CombinedOutput()
			if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code {
				t.Fatalf("Java: %v\n%s", err, java)
			}
			got, want := temporalProgressMessages(out.String()), temporalProgressMessages(string(java))
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("Go %q, Java %q\nGo output %s\nJava output %s", got, want, &out, java)
			}
		})
	}
}

func temporalProgressMessages(output string) []string {
	var result []string
	// PrintT output is outside tool envelopes; retain it in order with messages.
	output = strings.ReplaceAll(output, "\n3\n", "\n@!@!@STARTMSG 9999:0 @!@!@\n3\n@!@!@ENDMSG 9999 @!@!@\n")
	for _, msg := range readToolMessages(output) {
		switch msg.code {
		case 2192:
			body, _, _ := strings.Cut(msg.body, " at (")
			result = append(result, body)
		case 2267, 2116, 2217, 2218, 2193, 2104, 2110, 2200, 9999:
			result = append(result, strconv.Itoa(msg.code))
		}
	}
	return result
}

func TestTemporalDurationBoundaries(t *testing.T) {
	for _, tc := range []struct {
		ms   int64
		want string
	}{
		{59999, "59s"},
		{60000, "00s"},
		{60001, "01min 00s"},
		{3599999, "59min 59s"},
		{3600000, "00min 00s"},
		{3600001, "01h 00min"},
		{86399999, "23h 59min"},
		{86400000, "00h 00min"},
		{86400001, "1d 00h"},
	} {
		var out bytes.Buffer
		printTemporalCheckEnd(&out, false, time.Duration(tc.ms)*time.Millisecond)
		if !strings.Contains(out.String(), "in "+tc.want+" at ") {
			t.Errorf("%dms: %s; want %s", tc.ms, out.String(), tc.want)
		}
	}
}

func TestTemporalProgressOmitsReducedTableauCount(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x=0\nNext == UNCHANGED x\nP == <>(x=1)\nV == x", "INIT Init\nNEXT Next\nPROPERTY P\nVIEW V")
	var out, errout bytes.Buffer
	if code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout); code != 13 {
		t.Fatalf("exit %d: %s %s", code, &out, &errout)
	}
	for _, code := range []int{2192, 2267} {
		if hasToolMessage(readToolMessages(out.String()), code) {
			t.Fatalf("unexpected message %d: %s", code, &out)
		}
	}
}

func TestJavaTLCTableauProgressWithInitialEffect(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == /\\ x=0 /\\ PrintT(\"init-effect\")\nNext == UNCHANGED x\nP == <>(x=1)", "INIT Init\nNEXT Next\nPROPERTY P")
	args := []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final", "M.tla"}
	var out, errout bytes.Buffer
	goArgs := append(append([]string{}, args[:len(args)-1]...), filepath.Join(dir, "M.tla"))
	code := run(context.Background(), goArgs, &out, &errout)
	cmd := exec.Command("java", append([]string{"-jar", jar}, args...)...)
	cmd.Dir = dir
	java, err := cmd.CombinedOutput()
	if code != 13 || errout.Len() != 0 || cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code {
		t.Fatalf("Go exit %d stderr %s; Java %v\n%s", code, &errout, err, java)
	}
	// Both checkers replay the initial predicate while recovering the trace.
	if strings.Count(out.String(), "init-effect") != 2 || strings.Count(string(java), "init-effect") != 2 {
		t.Fatalf("effect count differs: Go %s\nJava %s", &out, java)
	}
	if got, want := temporalProgressMessages(out.String()), temporalProgressMessages(string(java)); !reflect.DeepEqual(got, want) {
		t.Fatalf("progress differs: Go %q Java %q\nGo %s\nJava %s", got, want, &out, java)
	}
}

func TestJavaTLCTableauProgressWithNextEffect(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x=0\nNext == /\\ UNCHANGED x /\\ PrintT(\"next-effect\")\nP == <>(x=1)", "INIT Init\nNEXT Next\nPROPERTY P")
	args := []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final", "M.tla"}
	var out, errout bytes.Buffer
	code := run(context.Background(), append(append([]string{}, args[:len(args)-1]...), filepath.Join(dir, "M.tla")), &out, &errout)
	cmd := exec.Command("java", append([]string{"-jar", jar}, args...)...)
	cmd.Dir = dir
	java, err := cmd.CombinedOutput()
	if code != 13 || errout.Len() != 0 || cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code {
		t.Fatalf("Go exit %d stderr %s; Java %v\n%s", code, &errout, err, java)
	}
	if !strings.Contains(out.String(), "next-effect") || !strings.Contains(string(java), "next-effect") || hasToolMessage(readToolMessages(out.String()), 2192) || !hasToolMessage(readToolMessages(string(java)), 2192) {
		t.Fatalf("expected effect and conservative omission: Go %s\nJava %s", &out, java)
	}
}

func TestJavaTLCTableauProgressWithReusedInitialPredicate(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	for _, tc := range []struct{ name, init, extra, constraint string }{
		{"state constraint", `x = 0 /\ PrintT("init-effect")`, "", "CONSTRAINT Init"},
		{"action constraint", `x = 0 /\ PrintT("init-effect")`, "", "ACTION_CONSTRAINT Init"},
		{"alias", `x = 0 /\ PrintT("init-effect")`, "C == Init", "CONSTRAINT C"},
		{"dynamic", `x = 0 /\ TLCGet("distinct") >= 0`, "", "CONSTRAINT Init"},
		{"instance", `x = 0 /\ I!PrintT("init-effect")`, "I == INSTANCE TLC\nC == Init", "CONSTRAINT C"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			pre, extra := "", tc.extra
			if after, ok := strings.CutPrefix(extra, "I == INSTANCE TLC\n"); ok {
				pre, extra = "I == INSTANCE TLC\n", after
			}
			writeDOTFixture(t, dir, "VARIABLE x\n"+pre+"Init == "+tc.init+"\n"+extra+"\nNext == UNCHANGED x\nP == <>(x=1)", "INIT Init\nNEXT Next\nPROPERTY P\n"+tc.constraint)
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final", "M.tla"}
			var out, errout bytes.Buffer
			code := run(context.Background(), append(append([]string{}, args[:len(args)-1]...), filepath.Join(dir, "M.tla")), &out, &errout)
			cmd := exec.Command("java", append([]string{"-jar", jar}, args...)...)
			cmd.Dir = dir
			java, err := cmd.CombinedOutput()
			if code != 13 || errout.Len() != 0 || cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code {
				t.Fatalf("Go exit %d stderr %s output %s; Java %v\n%s", code, &errout, &out, err, java)
			}
			if hasToolMessage(readToolMessages(out.String()), 2192) || !hasToolMessage(readToolMessages(string(java)), 2192) {
				t.Fatalf("expected conservative omission: Go %s\nJava %s", &out, java)
			}
		})
	}
}
