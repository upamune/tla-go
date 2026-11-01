package main

import (
	"bytes"
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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

func selectLivenessRecorder(records []string) []string {
	var result []string
	for _, record := range records {
		if strings.HasPrefix(record, "2212\t") || strings.HasPrefix(record, "2192\t") {
			result = append(result, record)
		}
	}
	return result
}
