package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/upamune/tla-go/internal/recorderdebug"
)

// Compare the selected direct recorder calls with Java TLC 1.7.4. This is
// not evidence of a complete recorder stream or a full upstream JUnit pass.
func TestJavaTLCRecorderLivenessEvents(t *testing.T) {
	jar := pinnedFairnessEffectJar(t)
	if jar == "" {
		t.Skip("set TLC_JAR to pinned Java TLC 1.7.4")
	}
	jar, err := filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	classes := compileSearchRecorder(t, jar)
	for _, tc := range []struct {
		name, property string
		exit           int
	}{
		{"success", "P == []<>(x >= 0)", 0},
		{"violation", "P == []<>(x = 4)", 13},
		{"multiple branches", "P == <>(x = 1) /\\ <>(x = 2)", 13},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, "VARIABLE x\nInit == x=0\nNext == x'=(x+1)%3\n"+tc.property, "INIT Init\nNEXT Next\nPROPERTY P\n")
			args := []string{"-tool", "-deadlock", "-workers", "1", "-fp", "0", "-checkpoint", "0", "-lncheck", "final", filepath.Join(dir, "M.tla")}
			t.Setenv("TLA_GO_DEBUG_RECORDER", "")
			var plain, plainErr bytes.Buffer
			plainCode := run(context.Background(), args, &plain, &plainErr)
			path := filepath.Join(dir, "go.jsonl")
			t.Setenv("TLA_GO_DEBUG_RECORDER", path)
			var observed, observedErr bytes.Buffer
			code := run(context.Background(), args, &observed, &observedErr)
			if code != tc.exit || plainCode != code || plainErr.String() != observedErr.String() || normalizeRecorderToolOutput(plain.String()) != normalizeRecorderToolOutput(observed.String()) {
				t.Fatalf("plain exit=%d stderr=%s; recorder exit=%d stderr=%s\nplain=%s\nrecorder=%s", plainCode, &plainErr, code, &observedErr, &plain, &observed)
			}
			allGoEvents := searchComparable(readSearchRecorder(t, path), true)
			goEvents := selectLivenessRecorder(allGoEvents)
			for _, tool := range []bool{false, true} {
				allJavaEvents := runSearchRecorderJava(t, jar, classes, dir, tc.exit, 1, tool, false, "-deadlock", "-checkpoint", "0", "-lncheck", "final")
				java := selectLivenessRecorder(allJavaEvents)
				if !reflect.DeepEqual(java, goEvents) || len(java) != 2 {
					t.Fatalf("tool=%t Java=%q Go=%q", tool, java, goEvents)
				}
				if tc.name == "success" && tool && !reflect.DeepEqual(allJavaEvents, allGoEvents) {
					t.Fatalf("selected call order Java=%q Go=%q", allJavaEvents, allGoEvents)
				}
			}
		})
	}
}

func TestJavaTLCRecorderRejectsDeferredLivenessError(t *testing.T) {
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
	writeDOTFixture(t, dir, "VARIABLE x\nInit == x=0\nNext == x'=(x+1)%3\nP == <>(1\\div (x-1)=0)", "INIT Init\nNEXT Next\nPROPERTY P\nCHECK_DEADLOCK FALSE\n")
	args := []string{"-tool", "-workers", "1", "-fp", "0", "-checkpoint", "0", "-lncheck", "final", filepath.Join(dir, "M.tla")}
	t.Setenv("TLA_GO_DEBUG_RECORDER", "")
	var plain, plainErr bytes.Buffer
	plainCode := run(context.Background(), args, &plain, &plainErr)
	path := filepath.Join(dir, "go.jsonl")
	t.Setenv("TLA_GO_DEBUG_RECORDER", path)
	var observed, observedErr bytes.Buffer
	code := run(context.Background(), args, &observed, &observedErr)
	if code != 255 || code != plainCode || plainErr.String() != observedErr.String() || normalizeRecorderToolOutput(plain.String()) != normalizeRecorderToolOutput(observed.String()) {
		t.Fatalf("plain exit=%d stderr=%s; recorder exit=%d stderr=%s", plainCode, &plainErr, code, &observedErr)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), recorderdebug.TemporalCheckUncertain) {
		t.Fatalf("missing uncertainty marker: %s", data)
	}
	if events, err := recorderdebug.Decode(bytes.NewReader(data)); err == nil || events != nil {
		t.Fatalf("uncertain recorder stream was accepted: events=%v error=%v", events, err)
	}
	for _, tool := range []bool{false, true} {
		java := selectLivenessRecorder(runSearchRecorderJava(t, jar, classes, dir, 255, 1, tool, false, "-checkpoint", "0", "-lncheck", "final"))
		if !reflect.DeepEqual(java, []string{"2212\t1"}) {
			t.Fatalf("tool=%t Java recorder=%q", tool, java)
		}
	}
	// The tool output is a known pre-existing mismatch and is not a recorder source.
	if !hasToolMessage(readToolMessages(observed.String()), 2192) {
		t.Fatal("expected the currently unsupported tool-output difference")
	}

	// The same uncertain final-check start must fail closed in ordinary mode.
	humanArgs := args[1:]
	path = filepath.Join(dir, "human-go.jsonl")
	t.Setenv("TLA_GO_DEBUG_RECORDER", path)
	observed.Reset()
	observedErr.Reset()
	if code := run(context.Background(), humanArgs, &observed, &observedErr); code != 255 {
		t.Fatalf("human exit=%d stderr=%s stdout=%s", code, &observedErr, &observed)
	}
	data, err = os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), recorderdebug.TemporalCheckUncertain) {
		t.Fatalf("missing human uncertainty marker: %s, %v", data, err)
	}
	if events, err := recorderdebug.Decode(bytes.NewReader(data)); err == nil || events != nil {
		t.Fatalf("uncertain human recorder stream was accepted: events=%v error=%v", events, err)
	}
}

func selectLivenessRecorder(records []string) []string {
	var result []string
	for _, record := range records {
		if strings.HasPrefix(record, "2212\t") || strings.HasPrefix(record, "2192\t") {
			result = append(result, record)
		}
	}
	return result
}
