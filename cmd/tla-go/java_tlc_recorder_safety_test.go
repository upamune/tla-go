package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/upamune/tla-go/internal/recorderdebug"
)

// The JVM recorder is the authority for the runtime array type, arguments,
// order, and repetitions of these selected calls; tool text is not used to
// manufacture the Go recorder values.
func TestJavaTLCSafetyRecorderCalls(t *testing.T) {
	jar := pinnedFairnessEffectJar(t)
	var classes string
	if jar != "" {
		var err error
		jar, err = filepath.Abs(jar)
		if err != nil {
			t.Fatal(err)
		}
		classes = compileSafetyRecorder(t, jar)
	}
	for _, tc := range []struct {
		name, body, cfg string
		extra           []string
		exit            int
		want            []string
	}{
		{"deadlock", "VARIABLE x\nInit == x = 0\nNext == x < 1 /\\ x' = x + 1", "INIT Init\nNEXT Next\n", nil, 11, []string{"2114", "2121"}},
		{"initial deadlock", "VARIABLE x\nInit == x = 0\nNext == x < 0 /\\ x' = x + 1", "INIT Init\nNEXT Next\n", nil, 11, []string{"2114", "2121"}},
		{"invariant", "VARIABLE x\nInit == x = 0\nNext == x < 3 /\\ x' = x + 1\nInv == x < 2", "INIT Init\nNEXT Next\nINVARIANT Inv\n", []string{"-deadlock"}, 12, []string{"2110\tInv", "2121"}},
		{"continue", "VARIABLE x\nInit == x = 0\nNext == x < 3 /\\ x' = x + 1\nInv == x < 2", "INIT Init\nNEXT Next\nINVARIANT Inv\n", []string{"-deadlock", "-continue"}, 0, []string{"2110\tInv", "2121", "2110\tInv", "2121"}},
		{"continued different invariants", "VARIABLE x\nInit == x = 0\nNext == x < 3 /\\ x' = x + 1\nInv == x # 1\nInv2 == x # 2", "INIT Init\nNEXT Next\nINVARIANTS Inv Inv2\n", []string{"-deadlock", "-continue"}, 0, []string{"2110\tInv", "2121", "2110\tInv2", "2121"}},
		{"initial invariant", "VARIABLE x\nInit == x = 0\nNext == UNCHANGED x\nInv == x = 1", "INIT Init\nNEXT Next\nINVARIANT Inv\n", []string{"-deadlock"}, 12, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, tc.body, tc.cfg)
			for _, tool := range []bool{false, true} {
				runSafetyRecorderCase(t, jar, classes, dir, tc.extra, tc.exit, tc.want, tool)
			}
		})
	}
}

func runSafetyRecorderCase(t *testing.T, jar, classes, dir string, extra []string, wantExit int, want []string, tool bool) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "events.jsonl")
	args := []string{"-workers", "1", "-fp", "0", "-checkpoint", "0"}
	if tool {
		args = append(args, "-tool")
	}
	args = append(args, extra...)
	args = append(args, filepath.Join(dir, "M.tla"))
	t.Setenv("TLA_GO_DEBUG_RECORDER", "")
	var plain, plainErr bytes.Buffer
	plainCode := run(context.Background(), args, &plain, &plainErr)
	t.Setenv("TLA_GO_DEBUG_RECORDER", path)
	var out, errout bytes.Buffer
	code := run(context.Background(), args, &out, &errout)
	if code != wantExit || code != plainCode || errout.String() != plainErr.String() || normalizeRecorderToolOutput(out.String()) != normalizeRecorderToolOutput(plain.String()) {
		t.Fatalf("tool=%t exit=%d stderr=%s stdout=%s; plain exit=%d stderr=%s stdout=%s", tool, code, &errout, &out, plainCode, &plainErr, &plain)
	}
	events := readSearchRecorder(t, path)
	got := selectedSafetyEvents(events)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tool=%t Go calls %q; want %q", tool, got, want)
	}
	if jar != "" {
		java := runSearchRecorderJava(t, jar, classes, dir, wantExit, 1, tool, false, append([]string{"-checkpoint", "0"}, extra...)...)
		if selected := selectSafetyRecorderLines(java); !reflect.DeepEqual(selected, got) {
			t.Fatalf("tool=%t Java calls %q; Go %q", tool, selected, got)
		}
		if all := searchWithSafety(events, tool); !reflect.DeepEqual(java, all) {
			t.Fatalf("tool=%t Java selected calls %q; Go %q", tool, java, all)
		}
	}
}

func searchWithSafety(events []recorderdebug.Event, tool bool) []string {
	var result []string
	for _, event := range events {
		if !tool && (event.Code == 2220 || event.Code == 2219) {
			continue
		}
		values := event.Values
		if event.Code == 2186 {
			values = []string{"TIME"}
		}
		result = append(result, strings.Join(append([]string{strconv.Itoa(event.Code)}, values...), "\t"))
	}
	return result
}

func compileSafetyRecorder(t *testing.T, jar string) string {
	t.Helper()
	dir := t.TempDir()
	source := strings.Replace(searchRecorderJava, "code == 2185 ||", "code == 2110 || code == 2114 || code == 2121 || code == 2185 ||", 1)
	if source == searchRecorderJava {
		t.Fatal("search recorder filter not found")
	}
	file := filepath.Join(dir, "SearchRecorder.java")
	if err := os.WriteFile(file, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "javac", "-cp", jar, file).CombinedOutput(); err != nil {
		t.Fatalf("javac: %v %s", err, out)
	}
	return dir
}

func selectedSafetyEvents(events []recorderdebug.Event) []string {
	var lines []string
	for _, event := range events {
		if event.Code == 2110 || event.Code == 2114 || event.Code == 2121 {
			lines = append(lines, strings.Join(append([]string{strconv.Itoa(event.Code)}, event.Values...), "\t"))
		}
	}
	return lines
}

func selectSafetyRecorderLines(lines []string) []string {
	var selected []string
	for _, line := range lines {
		code, _, _ := strings.Cut(line, "\t")
		if code == "2110" || code == "2114" || code == "2121" {
			selected = append(selected, line)
		}
	}
	return selected
}
