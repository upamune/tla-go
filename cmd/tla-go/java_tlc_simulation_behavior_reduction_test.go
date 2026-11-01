package main

import (
	"bytes"
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// These recovery failures include non-terminal stutters that differ as full
// states but may have equal VIEW or symmetry-reduced fingerprints.
func TestJavaTLCSimulationBehaviorReduction(t *testing.T) {
	jar := pinnedFairnessEffectJar(t)
	if jar == "" {
		t.Skip("set TLC_JAR to pinned Java TLC 1.7.4")
	}
	const transition = `Next == PrintT(<<"set", TLCGet("level")>>) /\ ((TLCGet("level") < 3 /\ x' \in {x, (x+1)%3}) \/ (TLCGet("level") >= 3 /\ x' = (x+2)%3)) /\ `
	const properties = "P == <>[](x=0)\n"
	for _, tc := range []struct {
		name, module, config string
		states               int
	}{
		{"view", "VARIABLES x, y\nInit == x=0 /\\ y=0\n" + transition + "y' = 1-y\n" + properties + "V == x\n", "INIT Init NEXT Next CHECK_DEADLOCK FALSE PROPERTY P VIEW V", 7},
		{"view-diff", "VARIABLES x, y\nInit == x=0 /\\ y=0\n" + transition + "y' = 1-y\n" + properties + "V == x\n", "INIT Init NEXT Next CHECK_DEADLOCK FALSE PROPERTY P VIEW V", 7},
		{"view-odd-diff", "VARIABLES x, y\nInit == x=0 /\\ y=0\nNext == PrintT(<<\"set\", TLCGet(\"level\")>>) /\\ x' = (IF TLCGet(\"level\") = 1 THEN x ELSE (x+1)%3) /\\ y' = 1-y\n" + properties + "V == x\n", "INIT Init NEXT Next CHECK_DEADLOCK FALSE PROPERTY P VIEW V", 8},
		{"view-odd-diff-terse", "VARIABLES x, y\nInit == x=0 /\\ y=0\nNext == PrintT(<<\"set\", TLCGet(\"level\")>>) /\\ x' = (IF TLCGet(\"level\") = 1 THEN x ELSE (x+1)%3) /\\ y' = 1-y\n" + properties + "V == x\n", "INIT Init NEXT Next CHECK_DEADLOCK FALSE PROPERTY P VIEW V", 8},
		{"symmetry", "CONSTANTS a, b\nVARIABLES x, y\nInit == x=0 /\\ y=a\n" + transition + "y' = IF y=a THEN b ELSE a\n" + properties + "Sym == Permutations({a,b})\n", "CONSTANTS a = a b = b INIT Init NEXT Next CHECK_DEADLOCK FALSE PROPERTY P SYMMETRY Sym", 7},
		{"symmetry-diff", "CONSTANTS a, b\nVARIABLES x, y\nInit == x=0 /\\ y=a\n" + transition + "y' = IF y=a THEN b ELSE a\n" + properties + "Sym == Permutations({a,b})\n", "CONSTANTS a = a b = b INIT Init NEXT Next CHECK_DEADLOCK FALSE PROPERTY P SYMMETRY Sym", 7},
		{"view-and-symmetry", "CONSTANTS a, b\nVARIABLES x, y\nInit == x=0 /\\ y=a\n" + transition + "y' = IF y=a THEN b ELSE a\n" + properties + "Sym == Permutations({a,b})\nV == x\n", "CONSTANTS a = a b = b INIT Init NEXT Next CHECK_DEADLOCK FALSE PROPERTY P SYMMETRY Sym VIEW V", 7},
		{"model-values", "CONSTANTS a, b\nVARIABLES x, y\nInit == x=0 /\\ y=a\n" + transition + "y' = IF y=a THEN b ELSE a\n" + properties + "Sym == Permutations({a,b})\n", "CONSTANTS a = a b = b INIT Init NEXT Next CHECK_DEADLOCK FALSE PROPERTY P", 9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, tc.module, tc.config)
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-simulate", "num=1", "-seed", "7", "-depth", "8", "M.tla"}
			if strings.HasPrefix(tc.name, "view-odd") {
				args[8] = "1"
			}
			if strings.Contains(tc.name, "-diff") {
				args = append([]string{"-difftrace"}, args...)
			}
			if strings.HasSuffix(tc.name, "-terse") {
				args = append([]string{"-terse"}, args...)
			}
			goArgs := append([]string{}, args...)
			goArgs[len(goArgs)-1] = filepath.Join(dir, "M.tla")
			var out, errout bytes.Buffer
			code := run(context.Background(), goArgs, &out, &errout)
			java, javaCode := runJavaTLCArgs(t, jar, dir, args)
			got, want := simulationBehaviorMessages(out.String()), simulationBehaviorMessages(java)
			if code != 255 || javaCode != code || errout.Len() != 0 || !reflect.DeepEqual(got, want) {
				t.Fatalf("Go exit %d stderr %s messages %+v\nJava exit %d messages %+v\nJava output %s", code, errout.String(), got, javaCode, want, java)
			}
			if count := simulationBehaviorStateCount(want); count != tc.states {
				t.Fatalf("Java emitted %d behavior states, want %d", count, tc.states)
			}
		})
	}
}

func simulationBehaviorMessages(output string) []toolMessage {
	var lines []string
	for line := range strings.SplitSeq(output, "\n") {
		if strings.HasPrefix(line, `<<"set"`) {
			lines = append(lines, "@!@!@STARTMSG 9000:0 @!@!@", line, "@!@!@ENDMSG 9000 @!@!@")
		} else {
			lines = append(lines, line)
		}
	}
	var selected []toolMessage
	for _, message := range readToolMessages(strings.Join(lines, "\n")) {
		switch message.code {
		case 9000, 1000, 2116, 2121, 2217, 2264:
			selected = append(selected, message)
		}
	}
	return selected
}

func simulationBehaviorStateCount(messages []toolMessage) int {
	count, inBehavior := 0, false
	for _, message := range messages {
		inBehavior = inBehavior || message.code == 2121
		if inBehavior && message.code == 2217 {
			count++
		}
	}
	return count
}
