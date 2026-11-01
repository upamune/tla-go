package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A model's file writes must not be replaced by a reconstructed graph.
func TestLivenessDOTExternalEffects(t *testing.T) {
	for _, tc := range []struct{ name, extra, action string }{
		{"serialize", "", `JsonSerialize("out_liveness.dot", <<x>>)`},
		{"indirect serialize", `Write == JsonSerialize("out_liveness.dot", <<x>>)`, "Write"},
		{"nd serialize", "", `ndJsonSerialize("out_liveness.dot", <<x>>)`},
		{"deserialize", "", `JsonDeserialize("input.json") = <<0>>`},
		{"nd deserialize", "", `ndJsonDeserialize("input.ndjson") = <<0>>`},
		{"assume serialize", `ASSUME JsonSerialize("out_liveness.dot", <<0>>)`, "TRUE"},
		{"indirect assume serialize", "Write == JsonSerialize(\"out_liveness.dot\", <<0>>)\nASSUME Write", "TRUE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS Naturals, Json\nVARIABLE x\nInit == x = 0\n" + tc.extra + "\nNext == /\\ x' = (x + 1) % 2 /\\ " + tc.action + "\nP == []<>(x = 0)\n====\n"
			var results [][]byte
			var exits []int
			for _, dump := range []bool{false, true} {
				dir := t.TempDir()
				writeLivenessEffectsFixture(t, dir, source)
				args := []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final"}
				if dump {
					args = append(args, "-dump", "dot", "out.dot")
				}
				out, code, stderr := runGoInDir(t, dir, append(args, "M.tla"))
				if code != 0 && code != 13 || stderr != "" {
					t.Fatalf("exit %d stderr %q: %s", code, stderr, out)
				}
				data, err := os.ReadFile(filepath.Join(dir, "out_liveness.dot"))
				if err != nil {
					t.Fatal(err)
				}
				results, exits = append(results, data), append(exits, code)
			}
			if !bytes.Equal(results[0], results[1]) || exits[0] != exits[1] || strings.HasPrefix(string(results[1]), "digraph") {
				t.Fatalf("model file changed with dump: no dump %q, dump %q", results[0], results[1])
			}
		})
	}
}

func TestLivenessDOTAssumptionEffectsPreserved(t *testing.T) {
	for _, tc := range []struct {
		statement string
		printed   bool
	}{
		{`ASSUME PrintT("assumption")`, true},
		{"PrintOnce == PrintT(\"assumption\")\nASSUME PrintOnce", true},
		{`THEOREM T == PrintT("assumption")`, true},
	} {
		t.Run(tc.statement, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == x = 0\nNext == UNCHANGED x\nP == <>[](x = 0)\n" + tc.statement
			writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nPROPERTY P\nCHECK_DEADLOCK FALSE\n")
			path := filepath.Join(dir, "out_liveness.dot")
			if err := os.WriteFile(path, []byte("existing"), 0o600); err != nil {
				t.Fatal(err)
			}
			out, code, stderr := runGoInDir(t, dir, []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final", "-dump", "dot", "out.dot", "M.tla"})
			if code != 0 || stderr != "" || strings.Contains(out, `"assumption"`) != tc.printed {
				t.Fatalf("exit %d stderr %q: %s", code, stderr, out)
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != "existing" {
				t.Fatalf("assumption output changed: %q, %v", got, err)
			}
		})
	}
}

func writeLivenessEffectsFixture(t *testing.T, dir, source string) {
	t.Helper()
	files := map[string]string{
		"M.tla": source, "M.cfg": "INIT Init\nNEXT Next\nPROPERTY P\nCHECK_DEADLOCK FALSE\n",
		"input.json": "[0]", "input.ndjson": "0\n", "out_liveness.dot": "existing",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLivenessDOTHigherOrderEffectsPreserved(t *testing.T) {
	dir := t.TempDir()
	body := `VARIABLE x
Init == x = 0
Next == LET F(Op(_)) == Op(x) IN /\ F(PrintT) /\ x' = (x + 1) % 2
P == []<>(x = 0)`
	writeDOTFixture(t, dir, body, "INIT Init\nNEXT Next\nPROPERTY P\nCHECK_DEADLOCK FALSE\n")
	path := filepath.Join(dir, "out_liveness.dot")
	if err := os.WriteFile(path, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, code, stderr := runGoInDir(t, dir, []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final", "-dump", "dot", "out.dot", "M.tla"})
	if code != 0 && code != 13 || stderr != "" {
		t.Fatalf("exit %d stderr %q: %s", code, stderr, out)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "existing" {
		t.Fatalf("higher-order model output changed: %q, %v", got, err)
	}
}
