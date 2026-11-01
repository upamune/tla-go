package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Java TLC 1.7.4 (upstream 5a47802b) evaluates fairness action checks while
// inserting the graph, even with -lncheck final. The marker records the exact
// order of evaluation, including the ENABLED and stuttering checks.
func TestJavaTLCFairnessEffectOrder(t *testing.T) {
	jar := pinnedFairnessEffectJar(t)
	for _, tc := range []struct {
		name, next string
		want       []string
	}{
		{"nondeterministic", `x' \in {0,1}`, []string{
			`<<"next", 0, 0>>`, `<<"next", 0, 1>>`, `<<"next", 0, 0>>`, `<<"next", 0, 1>>`,
			`<<"next", 0, 0>>`, `<<"next", 0, 1>>`, `<<"next", 0, 1>>`, `<<"next", 1, 0>>`,
			`<<"next", 1, 1>>`, `<<"next", 1, 0>>`, `<<"next", 1, 0>>`, `<<"next", 1, 0>>`,
		}},
		{"deterministic", `x' = (x+1)%3`, []string{
			`<<"next", 0, 1>>`, `<<"next", 0, 1>>`, `<<"next", 0, 1>>`, `<<"next", 0, 1>>`,
			`<<"next", 1, 2>>`, `<<"next", 1, 2>>`, `<<"next", 1, 2>>`, `<<"next", 1, 2>>`,
			`<<"next", 2, 0>>`, `<<"next", 2, 0>>`, `<<"next", 2, 0>>`, `<<"next", 2, 0>>`,
		}},
	} {
		for _, strategy := range []string{"default", "final"} {
			t.Run(tc.name+"/"+strategy, func(t *testing.T) {
				dir := t.TempDir()
				body := "VARIABLE x\nInit == x=0\nNext == " + tc.next + ` /\ PrintT(<<"next",x,x'>>)` + "\nSpec == Init /\\ [][Next]_x /\\ WF_x(Next)\nP == []<>(x = 0)"
				writeDOTFixture(t, dir, body, "SPECIFICATION Spec\nPROPERTY P")
				args := []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", strategy}
				var out, errout bytes.Buffer
				code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
				if code != 0 || errout.Len() != 0 || !reflect.DeepEqual(fairnessEffectLines(out.String()), tc.want) {
					t.Fatalf("Go exit %d stderr %s effects %q\n%s", code, &errout, fairnessEffectLines(out.String()), &out)
				}
				if jar != "" {
					java, javaCode := runJavaTLCArgs(t, jar, dir, append(args, "M.tla"))
					if javaCode != 0 || !reflect.DeepEqual(fairnessEffectLines(java), tc.want) {
						t.Fatalf("Java exit %d effects %q\n%s", javaCode, fairnessEffectLines(java), java)
					}
				}
			})
		}
	}
}

// Multiple liveness solutions share a Go cache today. Until it is made
// solution-specific, the single-solution change must not change this case.
func TestJavaTLCFairnessMultipleSolutionsKnownGap(t *testing.T) {
	dir := t.TempDir()
	body := "VARIABLE x\nInit == x=0\nNext == " + `x' \in {0,1} /\ PrintT(<<"next",x,x'>>)` + "\nSpec == Init /\\ [][Next]_x /\\ WF_x(Next)\nP == <>(x=0) /\\ <>(x=1)"
	writeDOTFixture(t, dir, body, "SPECIFICATION Spec\nPROPERTY P")
	args := []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final"}
	var out, errout bytes.Buffer
	code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &out, &errout)
	if code != 0 || errout.Len() != 0 || !strings.Contains(out.String(), "has 2 branches") || len(fairnessEffectLines(out.String())) != 11 {
		t.Fatalf("Go exit %d stderr %s effects %q\n%s", code, &errout, fairnessEffectLines(out.String()), &out)
	}
	if jar := pinnedFairnessEffectJar(t); jar != "" {
		java, javaCode := runJavaTLCArgs(t, jar, dir, append(args, "M.tla"))
		if javaCode != 0 || !strings.Contains(java, "has 2 branches") || len(fairnessEffectLines(java)) != 20 {
			t.Fatalf("Java exit %d effects %q\n%s", javaCode, fairnessEffectLines(java), java)
		}
	}
}

func pinnedFairnessEffectJar(t *testing.T) string {
	t.Helper()
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		return ""
	}
	data, err := os.ReadFile(jar)
	if err != nil || fmt.Sprintf("%x", sha256.Sum256(data)) != "936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88" {
		t.Fatalf("TLC_JAR must be the pinned 1.7.4 jar: %v", err)
	}
	return jar
}

func fairnessEffectLines(out string) []string {
	var result []string
	for line := range strings.SplitSeq(out, "\n") {
		if strings.HasPrefix(line, `<<"next",`) {
			result = append(result, line)
		}
	}
	return result
}
