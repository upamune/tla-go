package main

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Snapshot produced by the 96ebfc0 executable; see testdata/checkpoint-v1/README.md.
func TestCheckpointV1Recovery(t *testing.T) {
	fixture, err := filepath.Abs("testdata/checkpoint-v1")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, file, suffix string
		flag               string
		want               int
	}{
		{"original", "", "", "", 12},
		{"modern option", "", "", "-modern-tlc-statistics", 255},
		{"source changed", "M.tla", "\n\\* changed\n", "", 255},
		{"config changed", "M.cfg", "\n\\* changed\n", "", 255},
		{"snapshot tampered", "tla-go.chkpt", "x", "", 255},
		{"identity tampered with checksum updated", "tla-go.chkpt", "", "", 255},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, name := range []string{"M.tla", "M.cfg", "tla-go.chkpt"} {
				data, err := os.ReadFile(filepath.Join(fixture, name))
				if err != nil {
					t.Fatal(err)
				}
				if tc.file == name {
					data = append(data, tc.suffix...)
				}
				if tc.name == "identity tampered with checksum updated" && name == "tla-go.chkpt" {
					prefix := []byte(`"Model":[255,`)
					i := bytes.Index(data, prefix)
					if i < 0 {
						t.Fatal("fixture identity not found")
					}
					data[i+len(prefix)-2] = '4'
					checksum := sha256.Sum256(data[:len(data)-sha256.Size])
					copy(data[len(data)-sha256.Size:], checksum[:])
				}
				if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-recover", dir}
			if tc.flag != "" {
				args = append(args, tc.flag)
			}
			out, code, stderr := runGoInDir(t, dir, append(args, "M.tla"))
			if code != tc.want || tc.want == 12 && !strings.Contains(out, "Invariant Inv is violated") || tc.want == 255 && !strings.Contains(out+stderr, "checkpoint") {
				t.Fatalf("exit %d stderr %q output:\n%s", code, stderr, out)
			}
			if tc.want == 255 && tc.name != "snapshot tampered" && !strings.Contains(out+stderr, "checkpoint identity mismatch") {
				t.Fatalf("identity validation was bypassed: %s%s", out, stderr)
			}
			if tc.want == 12 && (!strings.Contains(out, "6 states generated, 6 distinct states found") || !strings.Contains(out, "6: <Next line 5, col 9 to line 5, col 27 of module M>\nx = 5")) {
				t.Fatalf("recovered trace or counts differ from v1:\n%s", out)
			}
		})
	}
}
