package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/upamune/tla-go/internal/recorderdebug"
)

// Compares the actual String[] recorder calls, including nested cost 2775,
// against Java TLC 1.7.4. The stream remains explicitly incomplete.
func TestJavaTLCRecorderCoverageEvents(t *testing.T) {
	jar := pinnedFairnessEffectJar(t)
	if jar == "" {
		t.Skip("set TLC_JAR to pinned Java TLC 1.7.4")
	}
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	classes := compileSearchRecorder(t, jar)
	dir := t.TempDir()
	model := "---- MODULE M ----\nEXTENDS Naturals, FiniteSets\nVARIABLE x\n" +
		"Init == x \\in 0..1\nA == x' = x\nB == x' = x\nNext == A \\/ B\n" +
		"Exp(N) == \\A e \\in SUBSET (1..N) : \\A f \\in e : f \\in 1..N\n" +
		"Inv == /\\ x \\in Nat\n       /\\ Exp(2)\n====\n"
	for name, content := range map[string]string{"M.tla": model, "M.cfg": "INIT Init\nNEXT Next\nINVARIANT Inv\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	args := []string{"-tool", "-workers", "1", "-fp", "0", "-coverage", "0", filepath.Join(dir, "M.tla")}
	t.Setenv("TLA_GO_DEBUG_RECORDER", "")
	var plain, plainErr bytes.Buffer
	plainCode := run(context.Background(), args, &plain, &plainErr)
	path := filepath.Join(dir, "go.jsonl")
	t.Setenv("TLA_GO_DEBUG_RECORDER", path)
	var observed, observedErr bytes.Buffer
	code := run(context.Background(), args, &observed, &observedErr)
	if code != 0 || code != plainCode || plainErr.String() != observedErr.String() || normalizeRecorderToolOutput(plain.String()) != normalizeRecorderToolOutput(observed.String()) {
		t.Fatalf("plain exit=%d stderr=%s; recorder exit=%d stderr=%s\nplain=%s\nrecorder=%s", plainCode, &plainErr, code, &observedErr, &plain, &observed)
	}
	events := readSearchRecorder(t, path)
	if !slices.ContainsFunc(events, func(e recorderdebug.Event) bool { return e.Code == 2775 }) {
		t.Fatalf("missing cost recorder event: %+v", events)
	}
	java := runSearchRecorderJava(t, jar, classes, dir, 0, 1, true, false, "-coverage", "0")
	if got := searchComparable(events, true); !reflect.DeepEqual(java, got) {
		t.Fatalf("Java recorder=%+v Go recorder=%+v", java, got)
	}

	args = args[1:]
	t.Setenv("TLA_GO_DEBUG_RECORDER", "")
	plain.Reset()
	plainErr.Reset()
	plainCode = run(context.Background(), args, &plain, &plainErr)
	path = filepath.Join(dir, "human.jsonl")
	t.Setenv("TLA_GO_DEBUG_RECORDER", path)
	observed.Reset()
	observedErr.Reset()
	code = run(context.Background(), args, &observed, &observedErr)
	if code != 0 || code != plainCode || plainErr.String() != observedErr.String() || normalizeTemporalProgress(plain.String()) != normalizeTemporalProgress(observed.String()) {
		t.Fatalf("human plain exit=%d stderr=%s; recorder exit=%d stderr=%s\nplain=%s\nrecorder=%s", plainCode, &plainErr, code, &observedErr, &plain, &observed)
	}
	events = readSearchRecorder(t, path)
	java = runSearchRecorderJava(t, jar, classes, dir, 0, 1, false, false, "-coverage", "0")
	if got := searchComparable(events, false); !reflect.DeepEqual(java, got) {
		t.Fatalf("human Java recorder=%+v Go recorder=%+v", java, got)
	}
}
