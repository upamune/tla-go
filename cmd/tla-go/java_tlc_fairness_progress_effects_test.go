package main

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The graph for these non-tableau, unreduced models has one fingerprint entry
// per expanded state, even when the fairness action prints while evaluated.
func TestJavaTLCFairnessEffectfulFinalProgress(t *testing.T) {
	jar := pinnedFairnessEffectJar(t)
	for _, tc := range []struct {
		name, init, next, extra, cfg, property string
		code, nodes                            int
	}{
		{"deterministic success", "x=0", `x'=(x+1)%3`, "", "", `[]<>(x>=0)`, 0, 3},
		{"nondeterministic success", "x=0", `x' \in {0,1}`, "", "", `[]<>(x>=0)`, 0, 2},
		{"state constraint", "x=0", `x'=(x+1)%4`, "C == x<2", "CONSTRAINT C", `[]<>(x>=0)`, 0, 2},
		{"multiple initial states", `x \in {0,2}`, `x'=(x+1)%4`, "", "", `[]<>(x>=0)`, 0, 4},
	} {
		for _, strategy := range []string{"default", "final"} {
			t.Run(tc.name+"/"+strategy, func(t *testing.T) {
				dir := t.TempDir()
				body := "VARIABLE x\nInit == " + tc.init + "\nNext == " + tc.next + ` /\ PrintT("effect")` + "\nSpec == Init /\\ [][Next]_x /\\ WF_x(Next)\nP == " + tc.property + "\n" + tc.extra
				writeDOTFixture(t, dir, body, "SPECIFICATION Spec\nPROPERTY P\n"+tc.cfg)
				args := []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", strategy}
				var out, errout bytes.Buffer
				code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
				messages := readToolMessages(out.String())
				if code != tc.code || errout.Len() != 0 || !hasToolMessage(messages, 2192) || !hasToolMessage(messages, 2267) {
					t.Fatalf("Go exit %d stderr %s\n%s", code, &errout, &out)
				}
				for _, msg := range messages {
					if msg.code == 2192 && !strings.Contains(msg.body, fmt.Sprintf("%d total distinct states", tc.nodes)) {
						t.Fatalf("unexpected graph count: %s", msg.body)
					}
				}
				if jar != "" {
					java, javaCode := runJavaTLCArgs(t, jar, dir, append(args, "M.tla"))
					got, want := fairnessProgressStream(out.String()), fairnessProgressStream(java)
					if javaCode != code || !reflect.DeepEqual(got, want) {
						t.Fatalf("Go exit %d %q\nJava exit %d %q\nGo %s\nJava %s", code, got, javaCode, want, &out, java)
					}
				}
			})
		}
	}
}

// Counterexample recovery can print effects in a different order from Java.
// Keep progress suppressed unless the simple recurrence holds in every state.
func TestFairnessEffectfulProgressConservativeCases(t *testing.T) {
	for _, tc := range []struct {
		name, property string
		flags          []string
		code           int
	}{
		{"violation", `[]<>(x=4)`, nil, 13},
		{"true but not all states satisfy", `[]<>(x=0)`, nil, 0},
		{"complex pure predicate", `[]<>(x*2>=0)`, nil, 0},
		{"continue", `[]<>(x>=0)`, []string{"-continue"}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == x=0\nNext == " + `x'=(x+1)%3 /\ PrintT("effect")` + "\nSpec == Init /\\ [][Next]_x /\\ WF_x(Next)\nP == " + tc.property
			writeDOTFixture(t, dir, body, "SPECIFICATION Spec\nPROPERTY P")
			args := append([]string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final"}, tc.flags...)
			var out, errout bytes.Buffer
			code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
			messages := readToolMessages(out.String())
			if code != tc.code || errout.Len() != 0 || hasToolMessage(messages, 2192) || hasToolMessage(messages, 2267) {
				t.Fatalf("Go exit %d stderr %s\n%s", code, &errout, &out)
			}
		})
	}
}

func TestFairnessEffectfulProgressSkipsExpansionFailure(t *testing.T) {
	dir := t.TempDir()
	body := "VARIABLE x\nInit == x=0\nNext == " + `x'=(x+1)%3 /\ PrintT("effect") /\ Assert(x<1, "stop")` + "\nSpec == Init /\\ [][Next]_x /\\ WF_x(Next)\nP == []<>(x>=0)"
	writeDOTFixture(t, dir, body, "SPECIFICATION Spec\nPROPERTY P")
	args := []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final"}
	var out, errout bytes.Buffer
	code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
	messages := readToolMessages(out.String())
	if code == 0 || errout.Len() != 0 || hasToolMessage(messages, 2192) || hasToolMessage(messages, 2267) {
		t.Fatalf("Go exit %d stderr %s\n%s", code, &errout, &out)
	}
	if jar := pinnedFairnessEffectJar(t); jar != "" {
		java, javaCode := runJavaTLCArgs(t, jar, dir, append(args, "M.tla"))
		javaMessages := readToolMessages(java)
		if javaCode != code || hasToolMessage(javaMessages, 2192) || hasToolMessage(javaMessages, 2267) {
			t.Fatalf("Go exit %d\nJava exit %d\nGo %s\nJava %s", code, javaCode, &out, java)
		}
	}
}

func TestFairnessEffectfulProgressEmptyInitialStates(t *testing.T) {
	dir := t.TempDir()
	body := "VARIABLE x\nInit == FALSE /\\ x=0\nNext == " + `x'=(x+1)%3 /\ PrintT("effect")` + "\nSpec == Init /\\ [][Next]_x /\\ WF_x(Next)\nP == []<>(x>=0)"
	writeDOTFixture(t, dir, body, "SPECIFICATION Spec\nPROPERTY P")
	args := []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final"}
	var out, errout bytes.Buffer
	code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
	messages := readToolMessages(out.String())
	if errout.Len() != 0 || !hasToolMessage(messages, 2192) || !hasToolMessage(messages, 2267) {
		t.Fatalf("Go exit %d stderr %s\n%s", code, &errout, &out)
	}
	if jar := pinnedFairnessEffectJar(t); jar != "" {
		java, javaCode := runJavaTLCArgs(t, jar, dir, append(args, "M.tla"))
		javaMessages := readToolMessages(java)
		if javaCode != code || !hasToolMessage(javaMessages, 2192) || !hasToolMessage(javaMessages, 2267) || !reflect.DeepEqual(fairnessProgressStream(out.String()), fairnessProgressStream(java)) {
			t.Fatalf("Go exit %d\nJava exit %d\nGo %s\nJava %s", code, javaCode, &out, java)
		}
	}
}
