package main

import (
	"bytes"
	"context"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/upamune/tla-go/internal/recorderdebug"
)

func TestHumanRecorderObservations(t *testing.T) {
	jar := pinnedFairnessEffectJar(t)
	var classes string
	if jar != "" {
		var err error
		jar, err = filepath.Abs(jar)
		if err != nil {
			t.Fatal(err)
		}
		classes = compileSearchRecorder(t, jar)
	}
	for _, tc := range []struct{ name, body, cfg string }{
		{"empty", "", ""},
		{"finite", "VARIABLE x\nInit == x = 0\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n"},
		{"effects", "VARIABLE x\nInit == x = IF PrintT(\"once\") THEN 1 ELSE 2\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeMaxSetSizeFixture(t, dir, tc.body, tc.cfg)
			args := []string{"-workers", "1", "-fp", "0", filepath.Join(dir, "M.tla")}
			t.Setenv("TLA_GO_DEBUG_RECORDER", "")
			var plain, plainErr bytes.Buffer
			if code := run(context.Background(), args, &plain, &plainErr); code != 0 || plainErr.Len() != 0 {
				t.Fatalf("plain %d: %s %s", code, &plain, &plainErr)
			}
			path := filepath.Join(dir, "human.jsonl")
			t.Setenv("TLA_GO_DEBUG_RECORDER", path)
			var observed, observedErr bytes.Buffer
			if code := run(context.Background(), args, &observed, &observedErr); code != 0 || observedErr.Len() != 0 || normalizeTemporalProgress(plain.String()) != normalizeTemporalProgress(observed.String()) {
				t.Fatalf("observed %d: %s %s; plain %s", code, &observed, &observedErr, &plain)
			}
			if tc.name == "effects" && strings.Count(observed.String(), `"once"`) != 1 {
				t.Fatalf("effect repeated: %s", &observed)
			}
			events := readSearchRecorder(t, path)
			if codes := humanRecorderCodes(events); !reflect.DeepEqual(codes, []int{2185, 2189, 2190, 2193, 2199, 2194, 2186}) {
				t.Fatalf("events: %+v", events)
			}
			if jar != "" {
				java := runSearchRecorderJava(t, jar, classes, dir, 0, 1, false, tc.name == "effects")
				if goEvents := searchComparable(events, false); !reflect.DeepEqual(java, goEvents) {
					t.Fatalf("Java %q; Go %q", java, goEvents)
				}
			}
		})
	}
}

func humanRecorderCodes(events []recorderdebug.Event) []int {
	codes := make([]int, 0, len(events))
	for _, event := range events {
		codes = append(codes, event.Code)
	}
	return codes
}

func TestHumanRecorderInitialFailure(t *testing.T) {
	dir := t.TempDir()
	writeMaxSetSizeFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == UNCHANGED x\nInv == x = 1", "INIT Init\nNEXT Next\nINVARIANT Inv\n")
	args := []string{"-workers", "1", "-fp", "0", filepath.Join(dir, "M.tla")}
	t.Setenv("TLA_GO_DEBUG_RECORDER", "")
	var plain, plainErr bytes.Buffer
	plainCode := run(context.Background(), args, &plain, &plainErr)
	path := filepath.Join(dir, "human.jsonl")
	t.Setenv("TLA_GO_DEBUG_RECORDER", path)
	var observed, observedErr bytes.Buffer
	code := run(context.Background(), args, &observed, &observedErr)
	if code != 12 || code != plainCode || plainErr.String() != observedErr.String() || normalizeTemporalProgress(plain.String()) != normalizeTemporalProgress(observed.String()) {
		t.Fatalf("plain %d %s %s; observed %d %s %s", plainCode, &plain, &plainErr, code, &observed, &observedErr)
	}
	events := readSearchRecorder(t, path)
	for _, event := range events {
		if event.Code == 2193 {
			t.Fatalf("failure falsely recorded as success: %+v", events)
		}
	}
}

// Java emits statistics after many failures and suppresses depth on liveness
// violations. Check the selected non-tool recorder calls against Java.
func TestJavaTLCRecorderHumanOutcomes(t *testing.T) {
	jar := pinnedFairnessEffectJar(t)
	var classes string
	if jar != "" {
		var err error
		jar, err = filepath.Abs(jar)
		if err != nil {
			t.Fatal(err)
		}
		classes = compileSearchRecorder(t, jar)
	}
	for _, tc := range []struct {
		name, body, cfg string
		exit            int
		deadlockOff     bool
		depth           bool
	}{
		{"success", "VARIABLE x\nInit == x=0\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n", 0, true, true},
		{"liveness success", "VARIABLE x\nInit == x=0\nNext == x'=(x+1)%3\nP == []<>(x >= 0)", "INIT Init\nNEXT Next\nPROPERTY P\n", 0, true, true},
		{"liveness violation", "VARIABLE x\nInit == x=0\nNext == x'=(x+1)%3\nP == []<>(x = 4)", "INIT Init\nNEXT Next\nPROPERTY P\n", 13, true, false},
		{"liveness branches", "VARIABLE x\nInit == x=0\nNext == x'=(x+1)%3\nP == <>(x = 1) /\\ <>(x = 2)", "INIT Init\nNEXT Next\nPROPERTY P\n", 13, true, false},
		{"safety violation", "VARIABLE x\nInit == x=0\nNext == x < 3 /\\ x'=x+1\nInv == x < 2", "INIT Init\nNEXT Next\nINVARIANT Inv\n", 12, true, true},
		{"initial violation", "VARIABLE x\nInit == x=0\nNext == UNCHANGED x\nInv == x = 1", "INIT Init\nNEXT Next\nINVARIANT Inv\n", 12, true, false},
		{"deadlock", "VARIABLE x\nInit == x=0\nNext == x < 1 /\\ x'=x+1", "INIT Init\nNEXT Next\n", 11, false, true},
		{"initial error", "VARIABLE x\nInit == x=1 \\div 0\nNext == UNCHANGED x", "INIT Init\nNEXT Next\n", 75, true, false},
		{"next error", "VARIABLE x\nInit == x=0\nNext == x' = 1 \\div 0", "INIT Init\nNEXT Next\n", 255, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, tc.body, tc.cfg)
			args := []string{"-workers", "1", "-fp", "0", "-checkpoint", "0", "-lncheck", "final"}
			if tc.deadlockOff {
				args = append(args, "-deadlock")
			}
			var plain, plainErr bytes.Buffer
			t.Setenv("TLA_GO_DEBUG_RECORDER", "")
			plainCode := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &plain, &plainErr)
			path := filepath.Join(dir, "human.jsonl")
			t.Setenv("TLA_GO_DEBUG_RECORDER", path)
			var observed, observedErr bytes.Buffer
			code := run(context.Background(), append(args, filepath.Join(dir, "M.tla")), &observed, &observedErr)
			if code != tc.exit || code != plainCode || plainErr.String() != observedErr.String() || normalizeTemporalProgress(plain.String()) != normalizeTemporalProgress(observed.String()) {
				t.Fatalf("plain exit=%d stderr=%s; recorder exit=%d stderr=%s\nplain=%s\nrecorder=%s", plainCode, &plainErr, code, &observedErr, &plain, &observed)
			}
			events := readSearchRecorder(t, path)
			if got := slices.Contains(humanRecorderCodes(events), 2194); got != tc.depth {
				t.Fatalf("depth present=%t, want %t: %+v", got, tc.depth, events)
			}
			if jar != "" {
				goEvents := searchComparable(events, false)
				java := runSearchRecorderJava(t, jar, classes, dir, tc.exit, 1, false, false, args...)
				if !reflect.DeepEqual(java, goEvents) {
					t.Fatalf("Java recorder=%q Go recorder=%q", java, goEvents)
				}
			}
		})
	}
}

func TestJavaHumanDuration(t *testing.T) {
	for _, tc := range []struct {
		elapsed time.Duration
		want    string
	}{
		{0, "00s"},
		{time.Second, "01s"},
		{time.Minute, "00s"},
		{time.Minute + time.Millisecond, "01min 00s"},
		{time.Hour, "00min 00s"},
		{time.Hour + time.Millisecond, "01h 00min"},
		{24 * time.Hour, "00h 00min"},
		{24*time.Hour + time.Millisecond, "1d 00h"},
		{365*24*time.Hour + time.Millisecond, "365d 00h"},
		{366*24*time.Hour + time.Millisecond, "1d 00h"},
	} {
		if got := javaHumanDuration(tc.elapsed); got != tc.want {
			t.Errorf("duration %v = %q, want %q", tc.elapsed, got, tc.want)
		}
	}
}
