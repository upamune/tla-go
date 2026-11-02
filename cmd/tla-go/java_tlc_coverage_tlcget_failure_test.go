package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestJavaTLCCoverageUndefinedStatisticsConstraint(t *testing.T) {
	jar := pinnedFairnessEffectJar(t)
	for _, key := range []string{"generated", "stats"} {
		for _, phase := range []string{"initial", "successor"} {
			t.Run(key+"/"+phase, func(t *testing.T) {
				dir := t.TempDir()
				limit := fmt.Sprintf(`TLCGet("%s") = 0`, key)
				if phase == "successor" {
					limit = "IF x = 0 THEN TRUE ELSE " + limit
				}
				body := "EXTENDS Naturals, TLC\nVARIABLE x\nInit == x = 0\nA == /\\ x < 3\n     /\\ x' = x + 1\nB == /\\ x < 3\n     /\\ x' = x + 1\nNext == A \\/ B\nLimit == " + limit
				writeCoverageModule(t, dir, "M", body)
				writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\nCONSTRAINT Limit\nCHECK_DEADLOCK FALSE\n")
				args := []string{"-tool", "-fp", "0", "-workers", "1", "-coverage", "9999", "-checkpoint", "0", "M.tla"}
				out, code, stderr := runGoInDir(t, dir, args)
				want := 75
				if phase == "successor" {
					want = 255
				}
				if code != want || stderr != "" || !strings.Contains(out, fmt.Sprintf("TLCGet(%s) was undefined.", key)) {
					t.Fatalf("Go exit %d stderr %q:\n%s", code, stderr, out)
				}
				if jar != "" {
					java, exit := runJavaTLCArgs(t, jar, dir, args)
					if exit != code || !reflect.DeepEqual(coverageMessages(readToolMessages(out)), coverageMessages(readToolMessages(java))) || !reflect.DeepEqual(errorBehaviorMessages(readToolMessages(out)), errorBehaviorMessages(readToolMessages(java))) {
						t.Fatalf("Java exit %d\nGo:\n%s\nJava:\n%s", exit, out, java)
					}
				}
			})
		}
	}
}

func TestJavaTLCCoverageUndefinedStatisticsThroughDefinition(t *testing.T) {
	dir := t.TempDir()
	body := `EXTENDS Naturals, TLC
VARIABLE x
Init == x = 0
A == UNCHANGED x
B == UNCHANGED x
Next == A \/ B
Read == TLCGet("generated")
Limit == Read = 1`
	writeCoverageModule(t, dir, "M", body)
	writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\nCONSTRAINT Limit\nCHECK_DEADLOCK FALSE\n")
	args := []string{"-tool", "-fp", "0", "-workers", "1", "-coverage", "9999", "-checkpoint", "0", "M.tla"}
	out, code, stderr := runGoInDir(t, dir, args)
	if code != 75 || stderr != "" || !strings.Contains(out, "TLCGet(generated) was undefined.") {
		t.Fatalf("Go exit %d stderr %q:\n%s", code, stderr, out)
	}
	if jar := pinnedFairnessEffectJar(t); jar != "" {
		java, exit := runJavaTLCArgs(t, jar, dir, args)
		if exit != code || !reflect.DeepEqual(coverageMessages(readToolMessages(out)), coverageMessages(readToolMessages(java))) || !reflect.DeepEqual(errorBehaviorMessages(readToolMessages(out)), errorBehaviorMessages(readToolMessages(java))) {
			t.Fatalf("Java exit %d\nGo:\n%s\nJava:\n%s", exit, out, java)
		}
	}
}

func TestJavaTLCCoverageUndefinedStatisticsLaterOrMultipleCandidates(t *testing.T) {
	jar := pinnedFairnessEffectJar(t)
	for _, tc := range []struct{ name, action, limit string }{
		{"later-state", `x' = x + 1`, `IF x < 2 THEN TRUE ELSE TLCGet("stats") = 0`},
		{"multiple-candidates", `x' \in {1, 2}`, `IF x = 0 THEN TRUE ELSE TLCGet("stats") = 0`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "EXTENDS Naturals, TLC\nVARIABLE x\nInit == x = 0\nA == /\\ x < 3\n     /\\ " + tc.action + "\nB == /\\ x < 3\n     /\\ x' = x + 1\nNext == A \\/ B\nLimit == " + tc.limit
			writeCoverageModule(t, dir, "M", body)
			writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\nCONSTRAINT Limit\nCHECK_DEADLOCK FALSE\n")
			args := []string{"-tool", "-fp", "0", "-workers", "1", "-coverage", "9999", "-checkpoint", "0", "M.tla"}
			out, code, stderr := runGoInDir(t, dir, args)
			if tc.name == "multiple-candidates" {
				if code == 0 || stderr != "" || !strings.Contains(out, "-coverage does not yet support this model's action or expression forms") || hasToolMessage(readToolMessages(out), 2201) {
					t.Fatalf("Go exit %d stderr %q:\n%s", code, stderr, out)
				}
				if jar != "" {
					java, exit := runJavaTLCArgs(t, jar, dir, args)
					if exit != 255 || !strings.Contains(java, "TLCGet(stats) was undefined.") || !hasToolMessage(readToolMessages(java), 2201) {
						t.Fatalf("Java exit %d:\n%s", exit, java)
					}
				}
				return
			}
			if code != 255 || stderr != "" || !strings.Contains(out, "TLCGet(stats) was undefined.") {
				t.Fatalf("Go exit %d stderr %q:\n%s", code, stderr, out)
			}
			if jar != "" {
				java, exit := runJavaTLCArgs(t, jar, dir, args)
				if exit != code || !reflect.DeepEqual(coverageMessages(readToolMessages(out)), coverageMessages(readToolMessages(java))) || !reflect.DeepEqual(errorBehaviorMessages(readToolMessages(out)), errorBehaviorMessages(readToolMessages(java))) {
					t.Fatalf("Java exit %d\nGo:\n%s\nJava:\n%s", exit, out, java)
				}
			}
		})
	}
}

func TestJavaTLCCoverageUndefinedStatisticsEffectfulConstraintFailsClosed(t *testing.T) {
	dir := t.TempDir()
	body := `EXTENDS Naturals, TLC
VARIABLE x
Init == x = 0
A == /\ x < 3
     /\ x' = x + 1
B == /\ x < 3
     /\ x' = x + 1
Next == A \/ B
Limit == IF x = 0 THEN TRUE ELSE IF PrintT("effect") THEN TLCGet("stats") = 0 ELSE FALSE`
	writeCoverageModule(t, dir, "M", body)
	writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\nCONSTRAINT Limit\nCHECK_DEADLOCK FALSE\n")
	args := []string{"-tool", "-fp", "0", "-workers", "1", "-coverage", "9999", "-checkpoint", "0", "M.tla"}
	out, code, stderr := runGoInDir(t, dir, args)
	if code == 0 || stderr != "" || !strings.Contains(out, "-coverage does not yet support named TLCGet reads") || strings.Count(out, `"effect"`) != 1 || hasToolMessage(readToolMessages(out), 2201) {
		t.Fatalf("Go exit %d stderr %q:\n%s", code, stderr, out)
	}
	if jar := pinnedFairnessEffectJar(t); jar != "" {
		java, exit := runJavaTLCArgs(t, jar, dir, args)
		if exit != 255 || !strings.Contains(java, "TLCGet(stats) was undefined.") || strings.Count(java, `"effect"`) != 2 || !hasToolMessage(readToolMessages(java), 2201) {
			t.Fatalf("Java exit %d:\n%s", exit, java)
		}
	}
}

func TestJavaTLCCoverageUndefinedStatisticsWithMutableReadsFailsClosed(t *testing.T) {
	for _, tc := range []struct{ name, limit, diagnostic string }{
		{"initial", `TLCGet("level") >= 0 /\ TLCGet("stats") = 0`, "initial error replay with mutable TLC or random operators"},
		{"successor", `IF x = 0 THEN TRUE ELSE TLCGet("level") >= 0 /\ TLCGet("stats") = 0`, "successor constraint error replay"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "EXTENDS Naturals, TLC\nVARIABLE x\nInit == x = 0\nA == /\\ x < 3\n     /\\ x' = x + 1\nB == /\\ x < 3\n     /\\ x' = x + 1\nNext == A \\/ B\nLimit == " + tc.limit
			writeCoverageModule(t, dir, "M", body)
			writeParameterizedConfig(t, dir, "INIT Init\nNEXT Next\nCONSTRAINT Limit\nCHECK_DEADLOCK FALSE\n")
			args := []string{"-tool", "-fp", "0", "-workers", "1", "-coverage", "9999", "-checkpoint", "0", "M.tla"}
			out, code, stderr := runGoInDir(t, dir, args)
			if code == 0 || stderr != "" || !strings.Contains(out, tc.diagnostic) || hasToolMessage(readToolMessages(out), 2201) {
				t.Fatalf("Go exit %d stderr %q:\n%s", code, stderr, out)
			}
			if jar := pinnedFairnessEffectJar(t); jar != "" {
				java, exit := runJavaTLCArgs(t, jar, dir, args)
				if exit == 0 || !strings.Contains(java, "TLCGet(stats) was undefined.") || !hasToolMessage(readToolMessages(java), 2201) {
					t.Fatalf("Java exit %d:\n%s", exit, java)
				}
			}
		})
	}
}
