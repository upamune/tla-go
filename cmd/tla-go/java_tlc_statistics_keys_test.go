package main

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
)

// Java TLC 1.7.4 does not define either key; 1.8.0 adds them.
func TestJavaTLCStatisticsKeyLifecycle(t *testing.T) {
	jar := maxSetOracleJar(t)
	for _, key := range []string{"generated", "stats"} {
		for _, tc := range []struct {
			name, body, config string
			exit, kind         int
			simulate           bool
		}{
			{"initial", `Init == x = IF PrintT(TLCGet("KEY")) THEN 0 ELSE 0`, "", 75, 0, false},
			{"constraint", `Init == x = 0\nC == PrintT(TLCGet("KEY"))`, "CONSTRAINT C", 75, 0, false},
			{"invariant", `Init == x = 0\nI == PrintT(TLCGet("KEY"))`, "INVARIANT I", 75, 0, false},
			{"next", `Init == x = 0\nN == PrintT(TLCGet("KEY")) /\ x' = x`, "", 255, 1, false},
			{"view", `Init == x = 0\nV == IF PrintT(TLCGet("KEY")) THEN x ELSE x`, "VIEW V", 75, 0, false},
			{"simulation", `Init == x = 0\nN == PrintT(TLCGet("KEY")) /\ x' = x`, "", 255, 0, true},
		} {
			t.Run(key+"/"+tc.name, func(t *testing.T) {
				dir := t.TempDir()
				body := strings.ReplaceAll(strings.ReplaceAll(tc.body, `\n`, "\n"), "KEY", key)
				if !strings.Contains(body, "N ==") {
					body += "\nN == UNCHANGED x"
				}
				writeMaxSetSizeFixture(t, dir, "VARIABLE x\n"+body, "INIT Init\nNEXT N\nCHECK_DEADLOCK FALSE\n"+tc.config+"\n")
				args := []string{"-tool", "-workers", "1", "-fp", "0"}
				if tc.simulate {
					args = append(args, "-simulate", "num=1", "-depth", "2", "-seed", "1")
				}
				args = append(args, "M.tla")
				out, code, stderr := runGoInDir(t, dir, args)
				assertUndefinedStatisticsKey(t, "Go", key, out, stderr, code, tc.exit, tc.kind)
				if jar != "" {
					out, code = runJavaTLCArgs(t, jar, dir, args)
					assertUndefinedStatisticsKey(t, "Java", key, out, "", code, tc.exit, tc.kind)
				}
			})
		}
	}
}

func TestJavaTLCModernStatisticsOption(t *testing.T) {
	for _, key := range []string{"generated", "stats"} {
		t.Run(key, func(t *testing.T) {
			dir := t.TempDir()
			body := fmt.Sprintf("VARIABLE x\nInit == x = 0\nN == PrintT(TLCGet(\"%s\")) /\\ UNCHANGED x", key)
			writeMaxSetSizeFixture(t, dir, body, "INIT Init\nNEXT N\nCHECK_DEADLOCK FALSE\n")
			args := []string{"-tool", "-workers", "1", "-fp", "0", "M.tla"}
			goArgs := append([]string{"-modern-tlc-statistics"}, args...)
			out, code, stderr := runGoInDir(t, dir, goArgs)
			if code != 0 || stderr != "" || strings.Contains(out, "was undefined") {
				t.Fatalf("Go exit %d stderr %q output:\n%s", code, stderr, out)
			}
			if jar := os.Getenv("TLC_MODERN_JAR"); jar != "" {
				out, code := runJavaTLCArgs(t, jar, dir, args)
				if code != 0 || strings.Contains(out, "was undefined") {
					t.Fatalf("Java 1.8 exit %d output:\n%s", code, out)
				}
			}
			if key != "generated" {
				return
			}
			args = append(slices.Clone(args[:len(args)-1]), "-simulate", "num=1", "-depth", "2", "-seed", "1", "M.tla")
			goArgs = append([]string{"-modern-tlc-statistics"}, args...)
			out, code, stderr = runGoInDir(t, dir, goArgs)
			assertUndefinedStatisticsKey(t, "Go modern simulation", key, out, stderr, code, 255, 0)
			if jar := os.Getenv("TLC_MODERN_JAR"); jar != "" {
				out, code := runJavaTLCArgs(t, jar, dir, args)
				assertUndefinedStatisticsKey(t, "Java 1.8 simulation", key, out, "", code, 255, 0)
			}
		})
	}
}

func TestJavaTLCStatisticsKeysAndNumericRegisters(t *testing.T) {
	dir := t.TempDir()
	body := "VARIABLE x\nInit == x = 0 /\\ TLCSet(0, 12)\nN == PrintT(TLCGet(0)) /\\ UNCHANGED x"
	writeMaxSetSizeFixture(t, dir, body, "INIT Init\nNEXT N\nCHECK_DEADLOCK FALSE\n")
	args := []string{"-tool", "-workers", "4", "-fp", "0", "M.tla"}
	for _, prefix := range [][]string{nil, {"-modern-tlc-statistics"}} {
		out, code, stderr := runGoInDir(t, dir, append(prefix, args...))
		if code != 0 || stderr != "" || !strings.Contains(out, "\n12\n") {
			t.Fatalf("Go exit %d stderr %q output:\n%s", code, stderr, out)
		}
	}
	if jar := maxSetOracleJar(t); jar != "" {
		out, code := runJavaTLCArgs(t, jar, dir, args)
		if code != 0 || !strings.Contains(out, "\n12\n") {
			t.Fatalf("Java exit %d output:\n%s", code, out)
		}
	}
}

func TestJavaTLCStatisticsKeysInSuccessor(t *testing.T) {
	jar := maxSetOracleJar(t)
	for _, key := range []string{"generated", "stats"} {
		for _, tc := range []struct {
			name, definition, config string
			exit                     int
		}{
			{"constraint", `C == IF x = 0 THEN TRUE ELSE PrintT(TLCGet("KEY"))`, "CONSTRAINT C", 255},
			{"invariant", `I == IF x = 0 THEN TRUE ELSE PrintT(TLCGet("KEY"))`, "INVARIANT I", 76},
			{"view", `V == IF x = 0 THEN x ELSE IF PrintT(TLCGet("KEY")) THEN x ELSE x`, "VIEW V", 255},
		} {
			for _, workers := range []string{"1", "4"} {
				t.Run(key+"/"+tc.name+"/"+workers, func(t *testing.T) {
					dir := t.TempDir()
					body := "VARIABLE x\nInit == x = 0\nN == x < 1 /\\ x' = x+1\n" + strings.ReplaceAll(tc.definition, "KEY", key)
					writeMaxSetSizeFixture(t, dir, body, "INIT Init\nNEXT N\nCHECK_DEADLOCK FALSE\n"+tc.config+"\n")
					args := []string{"-tool", "-workers", workers, "-fp", "0", "M.tla"}
					out, code, stderr := runGoInDir(t, dir, args)
					assertUndefinedStatisticsKey(t, "Go", key, out, stderr, code, tc.exit, 0)
					if tc.exit == 255 && !strings.Contains(out, "tlc2.tool.EvalException:") {
						t.Fatalf("Go missing EvalException wrapper:\n%s", out)
					}
					if jar != "" {
						out, code = runJavaTLCArgs(t, jar, dir, args)
						assertUndefinedStatisticsKey(t, "Java", key, out, "", code, tc.exit, 0)
						if tc.exit == 255 && !strings.Contains(out, "tlc2.tool.EvalException:") {
							t.Fatalf("Java missing EvalException wrapper:\n%s", out)
						}
					}
				})
			}
		}
	}
}

func assertUndefinedStatisticsKey(t *testing.T, engine, key, output, stderr string, got, want, kind int) {
	t.Helper()
	marker := fmt.Sprintf("TLCGet(%s) was undefined.", key)
	if got != want || stderr != "" || !strings.Contains(output, fmt.Sprintf("@!@!@STARTMSG 2145:%d", kind)) || !strings.Contains(output, marker) {
		t.Fatalf("%s exit %d (want %d), stderr %q, output:\n%s", engine, got, want, stderr, output)
	}
}
