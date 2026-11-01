package main

import (
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestJavaTLCModelLivenessBranches(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	for _, tc := range []struct {
		name, definitions, config string
		liveness                  bool
	}{
		{"eventually", "P == <>(x = 1)", "INIT Init NEXT Next PROPERTY P", true},
		{"often", "P == []<>(x = 1)", "INIT Init NEXT Next PROPERTY P", true},
		{"stability", "P == <>[](x = 1)", "INIT Init NEXT Next PROPERTY P", true},
		{"quantified", "P == \\A v \\in {1, 2} : [](x # v)", "INIT Init NEXT Next PROPERTY P", true},
		{"combined", "P == <>(x = 1)\nQ == []<>(x = 2)", "INIT Init NEXT Next PROPERTY P Q", true},
		{"fairness", "Spec == Init /\\ [][Next]_x /\\ WF_x(Next)\nP == []<>(x = 1)", "SPECIFICATION Spec PROPERTY P", true},
		{"failed assumption", "ASSUME FALSE\nP == <>(x = 1)", "INIT Init NEXT Next PROPERTY P", true},
		{"safety only", "P == [](x < 2)", "INIT Init NEXT Next PROPERTY P", false},
		{"fairness only", "Spec == Init /\\ [][Next]_x /\\ WF_x(Next)", "SPECIFICATION Spec", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 3\n" + tc.definitions
			writeDOTFixture(t, dir, body, tc.config)
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final", "M.tla"}
			got, code, stderr := runGoToolInDir(t, dir, args)
			java, javaCode := runJavaTLCArgs(t, jar, dir, args)
			if code != javaCode || stderr != "" {
				t.Fatalf("Go exit %d, Java exit %d; stderr %s\nJava: %s", code, javaCode, stderr, java)
			}
			if hasToolMessage(readToolMessages(java), 2212) != tc.liveness {
				t.Fatalf("unexpected Java liveness messages: %s", java)
			}
			if g, w := livenessBranchMessages(got), livenessBranchMessages(readToolMessages(java)); !reflect.DeepEqual(g, w) {
				t.Fatalf("Go liveness messages %+v, Java %+v", g, w)
			}
		})
	}
}

func livenessBranchMessages(messages []toolMessage) []toolMessage {
	var selected []toolMessage
	for _, message := range messages {
		if message.code == 2212 || message.code == 2189 || message.code == 2104 {
			selected = append(selected, message)
		}
	}
	return selected
}

func TestJavaTLCModelLivenessBranchMatrix(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("TLC_JAR is not set")
	}
	for _, first := range []string{"<>", "[]<>", "<>[]", "[]"} {
		for _, second := range []string{"<>", "[]<>", "<>[]", "[]"} {
			for _, fair := range []bool{false, true} {
				name := fmt.Sprintf("%s/%s/fair=%t", first, second, fair)
				t.Run(name, func(t *testing.T) {
					dir := t.TempDir()
					body := "VARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 3\nSpec == Init /\\ [][Next]_x"
					if fair {
						body += " /\\ WF_x(Next)"
					}
					body += "\nP == " + first + "(x = 1)\nQ == " + second + "(x = 2)"
					writeDOTFixture(t, dir, body, "SPECIFICATION Spec PROPERTY P Q")
					args := []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final", "M.tla"}
					got, _, _ := runGoToolInDir(t, dir, args)
					java, _ := runJavaTLCArgs(t, jar, dir, args)
					if g, w := livenessBranchMessages(got), livenessBranchMessages(readToolMessages(java)); !reflect.DeepEqual(g, w) {
						t.Fatalf("Go liveness messages %+v, Java %+v", g, w)
					}
				})
			}
		}
	}
}
