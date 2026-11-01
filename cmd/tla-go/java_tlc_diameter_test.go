package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// The pinned upstream DiameterTest uses four workers and allows any depth >= 8.
// A single-worker Java 1.7.4 run supplies the exact DOT and depth reference.
func TestJavaTLCDiameterSingleWorkerReference(t *testing.T) {
	dir := t.TempDir()
	for _, ext := range []string{".tla", ".cfg"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "checker", "testdata", "java-tlc", "DieHardTLA"+ext))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "DieHardTLA"+ext), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	wantDOT, err := os.ReadFile(filepath.Join("..", "..", "docs", "investigations", "testdata", "diameter-20dcf5d", "java-1.dot"))
	if err != nil {
		t.Fatal(err)
	}
	for _, workers := range []int{1, 4} {
		t.Run(strconv.Itoa(workers), func(t *testing.T) {
			dump := filepath.Join(t.TempDir(), "go.dot")
			args := []string{"-tool", "-deadlock", "-fp", "0", "-workers", strconv.Itoa(workers), "-checkpoint", "0", "-dump", "dot", dump}
			var out, errout bytes.Buffer
			code := run(context.Background(), append(args, filepath.Join(dir, "DieHardTLA")), &out, &errout)
			if code != 0 || errout.Len() != 0 {
				t.Fatalf("Go exit %d, stderr %s\n%s", code, &errout, &out)
			}
			if err := checkDiameterMessages(out.String(), workers == 1); err != nil {
				t.Fatalf("Go messages: %v\n%s", err, &out)
			}
			if workers == 1 {
				got, err := os.ReadFile(dump)
				if err != nil || !bytes.Equal(got, wantDOT) {
					t.Fatalf("Go DOT differs from captured single-worker Java: %v", err)
				}
			} else {
				assertDiameterCanonicalGraph(t, dump)
			}
		})
	}
	if jar := pinnedFairnessEffectJar(t); jar != "" {
		dump := filepath.Join(t.TempDir(), "java.dot")
		output, code := runJavaTLCArgs(t, jar, dir, []string{"-tool", "-deadlock", "-fp", "0", "-coverage", "1", "-workers", "1", "-checkpoint", "0", "-dump", "dot", dump, "DieHardTLA"})
		if code != 0 {
			t.Fatalf("Java exit %d\n%s", code, output)
		}
		if err := checkDiameterMessages(output, true); err != nil {
			t.Fatalf("Java messages: %v\n%s", err, output)
		}
		got, err := os.ReadFile(dump)
		if err != nil || !bytes.Equal(got, wantDOT) {
			t.Fatalf("Java DOT differs from captured single-worker Java: %v", err)
		}
	}
}

func assertDiameterCanonicalGraph(t *testing.T, dump string) {
	t.Helper()
	want, err := os.ReadFile(filepath.Join("..", "..", "docs", "investigations", "testdata", "diameter-20dcf5d", "graph.canonical"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "run", filepath.Join("..", "..", "docs", "investigations", "continuecanon", "main.go"), dump)
	got, err := cmd.CombinedOutput()
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("canonical graph differs: %v\n%s", err, got)
	}
}

func checkDiameterMessages(output string, single bool) error {
	depth, count := 0, 0
	stats := 0
	for _, message := range readToolMessages(output) {
		switch message.code {
		case 2194:
			count++
			if _, err := fmt.Sscanf(message.body, "The depth of the complete state graph search is %d.", &depth); err != nil || message.body != fmt.Sprintf("The depth of the complete state graph search is %d.", depth) {
				return fmt.Errorf("invalid depth message %q", message.body)
			}
		case 2199:
			stats++
			if message.body != "97 states generated, 16 distinct states found, 0 states left on queue." {
				return fmt.Errorf("invalid statistics %q", message.body)
			}
		}
	}
	if count != 1 || stats != 1 || depth < 8 || single && depth != 8 {
		return fmt.Errorf("depth %d in %d messages, statistics messages %d", depth, count, stats)
	}
	return nil
}

func TestDiameterMessageBoundaries(t *testing.T) {
	makeOutput := func(depth, generated int) string {
		return fmt.Sprintf("@!@!@STARTMSG 2194:0 @!@!@\nThe depth of the complete state graph search is %d.\n@!@!@ENDMSG 2194 @!@!@\n@!@!@STARTMSG 2199:0 @!@!@\n%d states generated, 16 distinct states found, 0 states left on queue.\n@!@!@ENDMSG 2199 @!@!@\n", depth, generated)
	}
	for _, tc := range []struct {
		name              string
		depth, generated  int
		single, wantError bool
	}{
		{"single 8", 8, 97, true, false},
		{"single 9", 9, 97, true, true},
		{"four 8", 8, 97, false, false},
		{"four 9", 9, 97, false, false},
		{"four too shallow", 7, 97, false, true},
		{"wrong generated", 8, 96, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkDiameterMessages(makeOutput(tc.depth, tc.generated), tc.single); (err != nil) != tc.wantError {
				t.Fatalf("got %v, wantError %v", err, tc.wantError)
			}
		})
	}
}
