package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type toolMessage struct {
	code, level int
	body        string
}

func TestJavaTLCToolMessages(t *testing.T) {
	cases := []struct {
		name, body, cfg string
		code            int
		want            []int
	}{
		{"success", "Init == x=0\nNext == x' = (x+1)%3", "INIT Init\nNEXT Next\n", 0, []int{2190, 2193, 2200, 2199}},
		{"multiple initial", "Init == x \\in {0,1,2}\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n", 0, []int{2190, 2193, 2199}},
		{"duplicate initial", "Init == (x=0) \\/ (x=0)\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n", 0, []int{2191, 2193, 2199}},
		{"deadlock", "Init == x=0\nNext == x<2 /\\ x'=x+1", "INIT Init\nNEXT Next\n", 11, []int{2190, 2114, 2121, 2217, 2199}},
		{"initial invariant", "Init == x=0\nNext == UNCHANGED x\nInv == x>0", "INIT Init\nNEXT Next\nINVARIANT Inv\n", 12, []int{2107}},
		{"invariant", "Init == x=0\nNext == x'=x+1\nInv == x<2", "INIT Init\nNEXT Next\nINVARIANT Inv\n", 12, []int{2190, 2110, 2121, 2217, 2199}},
		{"aliased property", "Init == x=0\nNext == x'=1-x\nQ == x=0\nP == []Q", "INIT Init\nNEXT Next\nPROPERTY P\n", 12, []int{2190, 2110, 2121, 2217, 2199}},
		{"liveness", "Init == x=0\nNext == x'=1-x\nProp == <> (x=2)", "INIT Init\nNEXT Next\nPROPERTY Prop\n", 13, []int{2190, 2116, 2264, 2217, 2218, 2199}},
		{"assumption", "ASSUME FALSE\nInit == x=0\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n", 10, []int{2104}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, "VARIABLE x\n"+tc.body, tc.cfg)
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final", filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != tc.code || errout.Len() != 0 {
				t.Fatalf("exit %d, want %d; stdout %s; stderr %s", code, tc.code, out.String(), errout.String())
			}
			messages := readToolMessages(out.String())
			for _, want := range tc.want {
				if !hasToolMessage(messages, want) {
					t.Errorf("missing message %d in %s", want, out.String())
				}
			}
			if !hasToolMessage(messages, 2186) {
				t.Errorf("missing final message: %s", out.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaToolMessages(t, jar, dir, code, messages)
			}
		})
	}
}

func readToolMessages(output string) []toolMessage {
	var result []toolMessage
	type frame struct {
		message toolMessage
		body    []string
	}
	var stack []frame
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		var code, level int
		if _, err := fmt.Sscanf(line, "@!@!@STARTMSG %d:%d @!@!@", &code, &level); err == nil {
			if len(stack) > 0 {
				stack[len(stack)-1].body = append(stack[len(stack)-1].body, line)
			}
			stack = append(stack, frame{message: toolMessage{code: code, level: level}})
		} else if len(stack) > 0 && strings.HasPrefix(line, "@!@!@ENDMSG ") {
			current := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				current.message.body = strings.TrimSpace(strings.Join(current.body, "\n"))
				result = append(result, current.message)
			} else {
				stack[len(stack)-1].body = append(stack[len(stack)-1].body, current.body...)
				stack[len(stack)-1].body = append(stack[len(stack)-1].body, line)
			}
		} else if len(stack) > 0 {
			stack[len(stack)-1].body = append(stack[len(stack)-1].body, line)
		}
	}
	return result
}

func hasToolMessage(messages []toolMessage, code int) bool {
	for _, message := range messages {
		if message.code == code {
			return true
		}
	}
	return false
}

func compareJavaToolMessages(t *testing.T, jar, dir string, code int, got []toolMessage) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "-lncheck", "final", "M.tla")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil || cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != code {
		t.Fatalf("Java TLC: %v\n%s", err, output)
	}
	java := toolConsumerMessages(readToolMessages(string(output)))
	goMessages := toolConsumerMessages(got)
	if !reflect.DeepEqual(java, goMessages) {
		t.Fatalf("consumer messages differ\nJava: %+v\nGo: %+v", java, goMessages)
	}
}

func toolConsumerMessages(messages []toolMessage) []toolMessage {
	var selected []toolMessage
	for _, message := range messages {
		switch message.code {
		case 2104, 2105, 2107, 2108, 2109, 2110, 2111, 2112, 2113, 2114, 2116, 2121, 2122, 2146, 2179, 2190, 2191, 2199, 2217, 2218, 2229, 2230, 2233, 2253, 2264:
			if message.code == 2190 || message.code == 2191 {
				message.body, _, _ = strings.Cut(message.body, " at ")
			}
			selected = append(selected, message)
		}
	}
	return selected
}

func TestJavaTLCToolEvaluationFailures(t *testing.T) {
	for _, tc := range []struct {
		name, body, cfg string
		code, message   int
	}{
		{"next division", "Init == x=0\nNext == x' = 1 \\div 0", "INIT Init\nNEXT Next\n", 255, 2179},
		{"invariant division", "Init == x=1\nNext == x'=0\nInv == 1 \\div x > 0", "INIT Init\nNEXT Next\nINVARIANT Inv\n", 76, 2111},
		{"assumption division", "ASSUME 1 \\div 0 = 0\nInit == x=0\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n", 10, 2105},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, "VARIABLE x\n"+tc.body, tc.cfg)
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout)
			messages := readToolMessages(out.String())
			if code != tc.code || !hasToolMessage(messages, tc.message) || errout.Len() != 0 {
				t.Fatalf("exit %d, want %d; stdout %s; stderr %s", code, tc.code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaToolMessages(t, jar, dir, code, messages)
			}
		})
	}
}

func TestJavaTLCToolConstantConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name, configuration, value string
		code                       int
	}{
		{"true property", "INIT Init\nNEXT Next\nPROPERTY C", "TRUE", 0},
		{"false property", "INIT Init\nNEXT Next\nPROPERTY C", "FALSE", 151},
		{"numeric property", "INIT Init\nNEXT Next\nPROPERTY C", "42", 151},
		{"true invariant", "INIT Init\nNEXT Next\nINVARIANT C", "TRUE", 0},
		{"false invariant", "INIT Init\nNEXT Next\nINVARIANT C", "FALSE", 151},
		{"constant init", "INIT C\nNEXT Next", "TRUE", 151},
		{"constant next", "INIT Init\nNEXT C", "TRUE", 151},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, "CONSTANT C\nVARIABLE x\nInit == x=0\nNext == UNCHANGED x", tc.configuration+"\nCONSTANT C = "+tc.value+"\n")
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout)
			if code != tc.code || errout.Len() != 0 {
				t.Fatalf("exit %d, want %d; stdout %s; stderr %s", code, tc.code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaToolMessages(t, jar, dir, code, readToolMessages(out.String()))
			}
		})
	}
}

func TestJavaTLCToolTupleFailureConstraints(t *testing.T) {
	for _, tc := range []struct {
		name, constraint string
		code             int
		detail           bool
	}{
		{"in model", "", 76, false},
		{"outside model", "CONSTRAINT Bound\n", 75, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == x=0\nNext == x'=x+1\nName == <<\"zero\", \"one\">>\nInv == Name[x+1] # \"none\"\nBound == x<2"
			writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nINVARIANT Inv\n"+tc.constraint)
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{"-tool", filepath.Join(dir, "M.tla")}, &out, &errout)
			messages := readToolMessages(out.String())
			want := "Attempted to apply tuple\n<<\"zero\", \"one\">>\nto integer 3 which is out of domain."
			if code != tc.code || !strings.Contains(out.String(), want) || hasToolMessage(messages, 2103) != tc.detail {
				t.Fatalf("exit %d, want %d; stdout %s; stderr %s", code, tc.code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				compareJavaToolMessages(t, jar, dir, code, messages)
			}
		})
	}
}
