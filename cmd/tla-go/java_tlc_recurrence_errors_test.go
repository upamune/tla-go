package main

import (
	"bytes"
	"context"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// Java LiveCheck.addNextState evaluates action checks before state checks when
// the source state's successors have been generated. Compare the resulting
// failure, search counts and direct recorder calls against TLC 1.7.4.
func TestJavaTLCRecurrenceEvaluationFailures(t *testing.T) {
	jar := pinnedFairnessEffectJar(t)
	if jar == "" {
		t.Skip("set TLC_JAR to pinned Java TLC 1.7.4")
	}
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	classes := compileSearchRecorder(t, jar)
	for _, tc := range []struct{ name, next, property, extra, config string }{
		{"recurrence", "x'=(x+1)%3", `[]<>(1\div (x-1)=0)`, "", ""},
		{"persistence", "x'=(x+1)%3", `<>[](1\div (x-1)=0)`, "", ""},
		{"fairness", "x'=(x+1)%3", `[]<>(1\div (x-1)=0)`, "Spec == Init /\\ [][Next]_x /\\ WF_x(Next)", "SPECIFICATION Spec\n"},
		{"multiple", "x'=(x+1)%3", `[]<>(1\div (x-1)=0) /\ []<>(x=2)`, "", ""},
		{"nondeterministic", `x' \in {0,1,2}`, `[]<>(1\div (x-1)=0)`, "", ""},
		{"constraint", "x'=(x+1)%3", `[]<>(1\div (x-1)=0)`, "C == x<2", "INIT Init\nNEXT Next\nCONSTRAINT C\n"},
		{"view", "x'=(x+1)%3", `[]<>(1\div (x-1)=0)`, "V == x % 2", "INIT Init\nNEXT Next\nVIEW V\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			config := tc.config
			if config == "" {
				config = "INIT Init\nNEXT Next\n"
			}
			writeDOTFixture(t, dir, "VARIABLE x\nInit == x=0\nNext == "+tc.next+"\nP == "+tc.property+"\n"+tc.extra, config+"PROPERTY P\nCHECK_DEADLOCK FALSE\n")
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-checkpoint", "0", "-lncheck", "final"}
			path := filepath.Join(dir, "go.jsonl")
			t.Setenv("TLA_GO_DEBUG_RECORDER", path)
			var out, errout bytes.Buffer
			code := run(context.Background(), append(slices.Clone(args), filepath.Join(dir, "M.tla")), &out, &errout)
			java, javaCode := runJavaTLCArgs(t, jar, dir, append(slices.Clone(args), "M.tla"))
			got, want := recurrenceFailureMessages(out.String()), recurrenceFailureMessages(java)
			if code != 255 || javaCode != code || errout.Len() != 0 || !reflect.DeepEqual(got, want) {
				t.Errorf("exit Go=%d Java=%d stderr=%s\nGo messages %v\nJava messages %v\nGo %s\nJava %s", code, javaCode, &errout, got, want, &out, java)
			}
			goEvents := selectLivenessRecorder(searchComparable(readSearchRecorder(t, path), true))
			javaEvents := selectLivenessRecorder(runSearchRecorderJava(t, jar, classes, dir, 255, 1, true, false, "-checkpoint", "0", "-lncheck", "final"))
			if !reflect.DeepEqual(javaEvents, goEvents) {
				t.Errorf("recorder Java=%q Go=%q", javaEvents, goEvents)
			}
			javaHumanEvents := selectLivenessRecorder(runSearchRecorderJava(t, jar, classes, dir, 255, 1, false, false, "-checkpoint", "0", "-lncheck", "final"))
			if !reflect.DeepEqual(javaHumanEvents, goEvents) {
				t.Errorf("human recorder Java=%q Go=%q", javaHumanEvents, goEvents)
			}
			t.Setenv("TLA_GO_DEBUG_RECORDER", "")
			var human, humanErr bytes.Buffer
			humanArgs := slices.Clone(args[1:])
			humanCode := run(context.Background(), append(humanArgs, filepath.Join(dir, "M.tla")), &human, &humanErr)
			javaHuman, javaHumanCode := runJavaTLCArgs(t, jar, dir, append(slices.Clone(args[1:]), "M.tla"))
			for _, fragment := range []string{"TLC threw an unexpected exception.", "The exception was a tlc2.tool.EvalException", `The second argument of \div is 0.`} {
				if humanCode != 255 || javaHumanCode != humanCode || humanErr.Len() != 0 || !strings.Contains(human.String(), fragment) || !strings.Contains(javaHuman, fragment) {
					t.Errorf("human exit Go=%d Java=%d stderr=%s fragment=%q\nGo %s\nJava %s", humanCode, javaHumanCode, &humanErr, fragment, &human, javaHuman)
				}
			}
		})
	}
}

func recurrenceFailureMessages(output string) []toolMessage {
	var result []toolMessage
	for _, msg := range readToolMessages(output) {
		if slices.Contains([]int{1000, 2121, 2192, 2194, 2199, 2212, 2268}, msg.code) {
			msg.body = strings.TrimSpace(msg.body)
			result = append(result, msg)
		}
	}
	return result
}

// Effectful fairness follows the existing path until its action and state
// evaluation order can be ported together. This deliberately records a known
// Java difference rather than presenting the case as compatible.
func TestJavaTLCRecurrenceEffectfulFairnessStillDeferred(t *testing.T) {
	jar := pinnedFairnessEffectJar(t)
	if jar == "" {
		t.Skip("set TLC_JAR to pinned Java TLC 1.7.4")
	}
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x=0\nNext == "+`x'=(x+1)%3 /\ PrintT("effect")`+"\nSpec == Init /\\ [][Next]_x /\\ WF_x(Next)\nP == "+`[]<>(1\div (x-1)=0)`, "SPECIFICATION Spec\nPROPERTY P\nCHECK_DEADLOCK FALSE\n")
	args := []string{"-tool", "-workers", "1", "-fp", "0", "-checkpoint", "0", "-lncheck", "final"}
	var out, errout bytes.Buffer
	code := run(context.Background(), append(slices.Clone(args), filepath.Join(dir, "M.tla")), &out, &errout)
	java, javaCode := runJavaTLCArgs(t, jar, dir, append(slices.Clone(args), "M.tla"))
	got, want := strings.Count(out.String(), `"effect"`), strings.Count(java, `"effect"`)
	if code != 255 || javaCode != 255 || errout.Len() != 0 || got != 12 || want != 8 {
		t.Fatalf("known effect mismatch: Go exit %d count %d; Java exit %d count %d; stderr %s", code, got, javaCode, want, &errout)
	}
}

func TestJavaTLCRecurrenceNonBooleanFailure(t *testing.T) {
	jar := pinnedFairnessEffectJar(t)
	if jar == "" {
		t.Skip("set TLC_JAR to pinned Java TLC 1.7.4")
	}
	dir := t.TempDir()
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x=0\nNext == x'=(x+1)%3\nP == []<>(x+1)", "INIT Init\nNEXT Next\nPROPERTY P\nCHECK_DEADLOCK FALSE\n")
	args := []string{"-tool", "-workers", "1", "-fp", "0", "-checkpoint", "0", "-lncheck", "final"}
	var out, errout bytes.Buffer
	code := run(context.Background(), append(slices.Clone(args), filepath.Join(dir, "M.tla")), &out, &errout)
	java, javaCode := runJavaTLCArgs(t, jar, dir, append(slices.Clone(args), "M.tla"))
	got, want := recurrenceFailureMessages(out.String()), recurrenceFailureMessages(java)
	if code != 255 || javaCode != 255 || errout.Len() != 0 || !reflect.DeepEqual(got, want) {
		t.Fatalf("Go exit %d messages %v; Java exit %d messages %v; stderr %s", code, got, javaCode, want, &errout)
	}
}
