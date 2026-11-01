package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

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

func TestTemporalProgressOmitsTableauCount(t *testing.T) {
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x=0\nNext == UNCHANGED x\nP == <>(x=1)", "INIT Init\nNEXT Next\nPROPERTY P")
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
