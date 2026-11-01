package main

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestJavaTLCUnionStateFingerprintError(t *testing.T) {
	jar := maxSetOracleJar(t)
	for _, tc := range []struct {
		name, body string
		terse      bool
		code       int
	}{
		{"initial", "Init == x = UNION {1}\nNext == UNCHANGED x", false, 255},
		{"initial terse", "Init == x = UNION {1}\nNext == UNCHANGED x", true, 75},
		{"successor", "Init == x = {}\nNext == x = {} /\\ x' = UNION {1}", false, 75},
		{"successor terse", "Init == x = {}\nNext == x = {} /\\ x' = UNION {1}", true, 75},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeMaxSetSizeFixture(t, dir, "VARIABLE x\n"+tc.body, "INIT Init\nNEXT Next\n")
			args := append([]string{"-fp", "0"}, maxSetOracleArgs()...)
			if tc.terse {
				args = append([]string{"-terse"}, args...)
			}
			out, code, stderr := runGoInDir(t, dir, args)
			if code != tc.code || stderr != "" || !strings.Contains(out, "Attempted to enumerate UNION(s), but some element of s is nonenumerable.") {
				t.Fatalf("Go exit %d stderr %q output:\n%s", code, stderr, out)
			}
			if jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				if javaCode != code || !reflect.DeepEqual(unionStateErrorMessages(out), unionStateErrorMessages(java)) {
					t.Fatalf("Go exit %d messages %q\nJava exit %d messages %q", code, unionStateErrorMessages(out), javaCode, unionStateErrorMessages(java))
				}
			}
		})
	}
}

func unionStateErrorMessages(out string) []string {
	var result []string
	for _, message := range readToolMessages(out) {
		if slices.Contains([]int{1000, 2102, 2121, 2147, 2199, 2217}, message.code) {
			result = append(result, fmt.Sprintf("%d:%d:%s", message.code, message.level, message.body))
		}
	}
	return result
}

func TestJavaTLCUnionTersePrint(t *testing.T) {
	jar := maxSetOracleJar(t)
	for _, tc := range []struct{ name, set, output string }{
		{"invalid", "UNION {1}", "UNION({1})\n"},
		{"deferred", "UNION {1..2,2..3}", "UNION({1..2, 2..3})\n"},
		{"eager", "UNION {{1},{2}}", "{1, 2}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == x = IF PrintT(" + tc.set + ") THEN 0 ELSE 1\nNext == UNCHANGED x"
			writeMaxSetSizeFixture(t, dir, body, "INIT Init\nNEXT Next\n")
			args := append([]string{"-terse"}, maxSetOracleArgs()...)
			out, code, stderr := runGoInDir(t, dir, args)
			if code != 0 || stderr != "" || maxSetInitialPrints(t, out, "@!@!@STARTMSG 2190:") != tc.output {
				t.Fatalf("Go exit %d stderr %q output:\n%s", code, stderr, out)
			}
			if jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				if javaCode != 0 || maxSetInitialPrints(t, java, "@!@!@STARTMSG 2190:") != tc.output {
					t.Fatalf("Java exit %d output:\n%s", javaCode, java)
				}
			}
		})
	}
}

func TestJavaTLCUnionNestedStateFingerprintError(t *testing.T) {
	jar := maxSetOracleJar(t)
	for _, tc := range []struct {
		name, body string
		terse      bool
		code       int
	}{
		{"initial record", "Init == x = [a |-> UNION {1}]\nNext == UNCHANGED x", false, 255},
		{"initial tuple", "Init == x = <<UNION {1}>>\nNext == UNCHANGED x", false, 255},
		{"initial higher order", "F(op(_)) == op({1})\nInit == x = F(UNION)\nNext == UNCHANGED x", false, 255},
		{"initial higher order terse", "F(op(_)) == op({1})\nInit == x = F(UNION)\nNext == UNCHANGED x", true, 75},
		{"initial record terse", "Init == x = [a |-> UNION {1}]\nNext == UNCHANGED x", true, 75},
		{"successor record", "Init == x = [a |-> {}]\nNext == x = [a |-> {}] /\\ x' = [a |-> UNION {1}]", false, 75},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeMaxSetSizeFixture(t, dir, "VARIABLE x\n"+tc.body, "INIT Init\nNEXT Next\n")
			args := append([]string{"-fp", "0"}, maxSetOracleArgs()...)
			if tc.terse {
				args = append([]string{"-terse"}, args...)
			}
			out, code, stderr := runGoInDir(t, dir, args)
			checkNestedUnionFailure(t, "Go", out, code, stderr, tc.code)
			if jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				checkNestedUnionFailure(t, "Java", java, javaCode, "", tc.code)
			}
		})
	}
}

func checkNestedUnionFailure(t *testing.T, engine, out string, code int, stderr string, want int) {
	t.Helper()
	if code != want || stderr != "" || !strings.Contains(out, "Attempted to enumerate UNION(s), but some element of s is nonenumerable.") {
		t.Fatalf("%s exit %d stderr %q output:\n%s", engine, code, stderr, out)
	}
	if hasToolMessage(readToolMessages(out), 2199) != (want == 75) || hasToolMessage(readToolMessages(out), 2121) {
		t.Fatalf("%s unexpected statistics or trace:\n%s", engine, out)
	}
}

func TestJavaTLCUnionDFIDStateError(t *testing.T) {
	jar := maxSetOracleJar(t)
	for _, tc := range []struct {
		name, body string
		terse      bool
		code       int
	}{
		{"initial", "Init == x = UNION {1}\nNext == UNCHANGED x", false, 255},
		{"initial terse", "Init == x = UNION {1}\nNext == UNCHANGED x", true, 75},
		{"successor", "Init == x = {}\nNext == x = {} /\\ x' = UNION {1}", false, 255},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeMaxSetSizeFixture(t, dir, "VARIABLE x\n"+tc.body, "INIT Init\nNEXT Next\n")
			args := append([]string{"-dfid", "3"}, maxSetOracleArgs()...)
			if tc.terse {
				args = append([]string{"-terse"}, args...)
			}
			out, code, stderr := runGoInDir(t, dir, args)
			if code != tc.code || stderr != "" || !strings.Contains(out, "Attempted to enumerate UNION(s), but some element of s is nonenumerable.") {
				t.Fatalf("Go exit %d stderr %q output:\n%s", code, stderr, out)
			}
			if jar != "" {
				java, javaCode := runJavaTLCArgs(t, jar, dir, args)
				if javaCode != code || !strings.Contains(java, "Attempted to enumerate UNION(s), but some element of s is nonenumerable.") {
					t.Fatalf("Java exit %d output:\n%s", javaCode, java)
				}
			}
		})
	}
}
