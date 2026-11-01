package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/upamune/tla-go/internal/recorderdebug"
)

type ifContextCase struct {
	name, definitions, property  string
	deferred, exact, modernTrace bool
	states                       int
}

// Java 1.7.4 and 1.8.0 recursively normalize Boolean junctions, but an IF
// inside a state-atom operand is evaluated during exploration. User-defined
// operators are boundaries because Java catches errors during their expansion.
func TestJavaTLCIfBooleanContexts(t *testing.T) {
	jars := ifOracleJars(t)
	if len(jars) == 0 {
		t.Skip("set TLC_JAR or TLC_MODERN_JAR")
	}
	for i := range jars {
		jars[i].classes = compileIfRecorder(t, jars[i])
	}
	for _, tc := range []ifContextCase{
		{"direct", "", `[]<>(IF x=1 THEN 1 ELSE TRUE)`, true, false, false, 0},
		{"conjunction", "", `[]<>(TRUE /\ (IF x=1 THEN 1 ELSE TRUE))`, true, false, false, 0},
		{"disjunction", "", `[]<>(FALSE \/ (IF x=1 THEN 1 ELSE TRUE))`, true, false, false, 0},
		{"negation", "", `[]<>(~(IF x=1 THEN 1 ELSE TRUE))`, true, false, false, 0},
		{"implication", "", `[]<>(TRUE => (IF x=1 THEN 1 ELSE TRUE))`, true, false, false, 0},
		{"boolean guard", "", `[]<>(IF (IF x=1 THEN 1 ELSE TRUE) THEN TRUE ELSE FALSE)`, true, false, false, 0},
		{"constant boolean sibling", "", `[]<>(((IF x=1 THEN {} ELSE {1}) = 1) /\ 1)`, true, false, false, 0},
		{"equality operand", "", `[]<>((IF x=1 THEN {} ELSE {1}) = 1)`, false, true, true, 2},
		{"conjunction atom", "", `[]<>(((IF x=1 THEN {} ELSE {1}) = 1) /\ TRUE)`, false, true, true, 2},
		{"disjunction atom", "", `[]<>(((IF x=1 THEN {} ELSE {1}) = 1) \/ FALSE)`, false, true, true, 2},
		{"membership operand", "", `[]<>(1 \in (IF x=1 THEN 1 ELSE {1}))`, false, false, false, 3},
		{"equivalence operand", "", `[]<>((IF x=1 THEN 1 ELSE TRUE) <=> TRUE)`, false, false, false, 3},
		{"alias boolean", `Q == IF x=1 THEN 1 ELSE TRUE`, `[]<>(Q)`, false, true, false, 3},
		{"alias operand", `Q == IF x=1 THEN {} ELSE {1}`, `[]<>(Q = 1)`, false, true, true, 2},
		{"nested equality", "", `[]<>(((IF x=1 THEN 1 ELSE TRUE) = TRUE) = TRUE)`, false, true, false, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, "VARIABLE x\nInit == x=0\nNext == x'=(x+1)%3\n"+tc.definitions+"\nP == "+tc.property, "INIT Init\nNEXT Next\nPROPERTY P\nCHECK_DEADLOCK FALSE\n")
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-checkpoint", "0", "-lncheck", "final"}
			path := filepath.Join(dir, "go.jsonl")
			t.Setenv("TLA_GO_DEBUG_RECORDER", path)
			var out, errout bytes.Buffer
			code := run(context.Background(), append(slices.Clone(args), filepath.Join(dir, "M.tla")), &out, &errout)
			if code != 255 || errout.Len() != 0 {
				t.Fatalf("Go exit %d stderr %s\n%s", code, &errout, &out)
			}
			goEvents := checkIfGoObservation(t, tc, path, out.String())
			for _, jar := range jars {
				checkIfJavaObservation(t, tc, jar, dir, args, out.String(), goEvents)
			}
		})
	}
}

type ifOracle struct {
	path, classes string
	modern        bool
}

func ifOracleJars(t *testing.T) []ifOracle {
	t.Helper()
	var jars []ifOracle
	for _, oracle := range []ifOracle{{path: pinnedFairnessEffectJar(t)}, {path: pinnedRecorderModernJar(t), modern: true}} {
		if oracle.path != "" {
			path, err := filepath.Abs(oracle.path)
			if err != nil {
				t.Fatal(err)
			}
			oracle.path = path
			jars = append(jars, oracle)
		}
	}
	return jars
}

func compileIfRecorder(t *testing.T, oracle ifOracle) string {
	t.Helper()
	if !oracle.modern {
		return compileSearchRecorder(t, oracle.path)
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "SearchRecorder.java")
	source := strings.ReplaceAll(searchRecorderJava, "MPRecorder", "IMessagePrinterRecorder")
	if err := os.WriteFile(file, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "javac", "-cp", oracle.path, file).CombinedOutput(); err != nil {
		t.Fatalf("javac 1.8 recorder: %v %s", err, out)
	}
	return dir
}

func checkIfGoObservation(t *testing.T, tc ifContextCase, path, output string) []string {
	t.Helper()
	messages := readToolMessages(output)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	events, decodeErr := recorderdebug.Decode(bytes.NewReader(data))
	if tc.deferred {
		if !hasToolMessage(messages, 2192) || hasToolMessage(messages, 2199) || decodeErr == nil || events != nil {
			t.Fatalf("deferred Go stream accepted: messages=%v events=%v error=%v", messages, events, decodeErr)
		}
		return nil
	}
	if decodeErr != nil || hasToolMessage(messages, 2192) || !hasToolMessage(messages, 1000) {
		t.Fatalf("early Go failure: messages=%v events=%v error=%v", messages, events, decodeErr)
	}
	want := fmt.Sprintf("%d states generated, %d distinct states found, 1 states left on queue.", tc.states, tc.states)
	if !strings.Contains(rawToolMessage(output, 2199), want) {
		t.Fatalf("Go missing counts %q: %s", want, output)
	}
	return selectLivenessRecorder(searchComparable(events, true))
}

func checkIfJavaObservation(t *testing.T, tc ifContextCase, jar ifOracle, dir string, args []string, goOutput string, goEvents []string) {
	t.Helper()
	java, exit := runJavaTLCArgs(t, jar.path, dir, append(slices.Clone(args), "M.tla"))
	messages := readToolMessages(java)
	if exit != 255 {
		t.Fatalf("Java %s exit=%d: %s", jar.path, exit, java)
	}
	if tc.deferred {
		if !hasToolMessage(messages, 2215) || hasToolMessage(messages, 2189) {
			t.Fatalf("Java %s did not fail during normalization: %s", jar.path, java)
		}
		return
	}
	if hasToolMessage(messages, 2192) {
		t.Fatalf("Java %s reported final liveness: %s", jar.path, java)
	}
	for _, code := range []int{2199, 2194, 2268} {
		if code == 2268 && tc.modernTrace && jar.modern {
			continue
		}
		if got, want := rawToolMessage(goOutput, code), rawToolMessage(java, code); got != want {
			t.Errorf("Java %s message %d: Go %q Java %q", jar.path, code, got, want)
		}
	}
	if tc.modernTrace && jar.modern {
		if !hasToolMessage(messages, 2121) || hasToolMessage(messages, 1000) {
			t.Errorf("unexpected Java 1.8 equality failure: %s", java)
		}
	} else if tc.exact && rawToolMessage(goOutput, 1000) != rawToolMessage(java, 1000) {
		t.Errorf("Java %s error: Go %q Java %q", jar.path, rawToolMessage(goOutput, 1000), rawToolMessage(java, 1000))
	}
	// The helper appends to its capture file; tool and human use distinct
	// paths. The selected liveness calls are the same in both modes.
	javaEvents := selectLivenessRecorder(runSearchRecorderJava(t, jar.path, jar.classes, dir, 255, 1, !jar.modern, false, "-checkpoint", "0", "-lncheck", "final"))
	if !slices.Equal(goEvents, javaEvents) {
		t.Errorf("Java %s recorder: Go %q Java %q", jar.path, goEvents, javaEvents)
	}
}
