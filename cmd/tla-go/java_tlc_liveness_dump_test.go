package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Java TLC 1.7.4 writes a separate non-strict liveness graph. The supported
// reconstruction is limited to completed, pure, non-reduced searches.
func TestJavaTLCLivenessDOT(t *testing.T) {
	jar := maxSetOracleJar(t)
	if jar == "" {
		t.Skip("set TLC_JAR to the pinned Java TLC 1.7.4 jar")
	}
	for _, tc := range []struct{ name, init, next, property, options string }{
		{"violation", `x = 0`, `x' \in 0..2`, `<>[](x = 0)`, "dot"},
		{"success", `x = 0`, `x' = (x + 1) % 3`, `[]<>(x = 0)`, "dot"},
		{"initial states", `x \in 0..2`, `UNCHANGED x`, `<>[](x = 0)`, "dot"},
		{"growth", `x = 0`, `x' \in 0..10`, `<>[](x = 0)`, "dot"},
		{"multiple growth", `x = 0`, `x' \in 0..36`, `<>[](x = 0)`, "dot"},
		{"duplicates", `x = 0`, `\/ x' \in 0..2 \/ x' \in 1..3`, `[]<>(x = 0)`, "dot"},
		{"branch", `x = 0`, `IF x < 2 THEN x' \in {x, x + 1} ELSE x' = 0`, `<>[](x = 0)`, "dot"},
		{"labels", `x = 0`, `x' \in 0..2`, `<>[](x = 0)`, "dot,actionlabels"},
		{"color", `x = 0`, `x' \in 0..2`, `<>[](x = 0)`, "dot,colorize,actionlabels"},
		{"stuttering only", `x = 0`, `UNCHANGED x`, `<>[](x = 0)`, "dot"},
		{"structured state", "", "", "", "dot"},
		{"literal constant", "", "", "", "dot"},
		{"fingerprint polynomial", `x = 0`, `x' \in 0..3`, `<>[](x = 0)`, "dot"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			javaDir, goDir := t.TempDir(), t.TempDir()
			body := "VARIABLE x\nInit == " + tc.init + "\nNext == " + tc.next + "\nP == " + tc.property
			if tc.name == "structured state" {
				body = `VARIABLES x, y
Init == /\ x = 0 /\ y = [a |-> "hi", b |-> <<1, 2>>]
Next == /\ x' \in 0..1 /\ UNCHANGED y
P == []<>(x = 0)`
			}
			cfg := "INIT Init\nNEXT Next\nPROPERTY P\nCHECK_DEADLOCK FALSE\n"
			if tc.name == "literal constant" {
				body = "CONSTANT N\nVARIABLE x\nInit == x = 0\nNext == x' \\in 0..N\nP == <>[](x = 0)"
				cfg += "CONSTANT N = 2\n"
			}
			writeDOTFixture(t, javaDir, body, cfg)
			writeDOTFixture(t, goDir, body, cfg)
			args := []string{"-tool", "-fp", "0", "-workers", "1", "-lncheck", "final", "-dump", tc.options, "out.dot", "M.tla"}
			if tc.name == "fingerprint polynomial" {
				args[2] = "5"
			}
			_, javaCode := runJavaTLCArgs(t, jar, javaDir, args)
			out, code, stderr := runGoInDir(t, goDir, args)
			if code != javaCode || stderr != "" {
				t.Fatalf("Java exit %d, Go exit %d stderr %q: %s", javaCode, code, stderr, out)
			}
			want, err := os.ReadFile(filepath.Join(javaDir, "out_liveness.dot"))
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(filepath.Join(goDir, "out_liveness.dot"))
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("Go liveness graph (%v):\n%s\nJava:\n%s", err, got, want)
			}
		})
	}
}

func TestLivenessDOTOutputSafety(t *testing.T) {
	for _, mode := range []string{"input alias", "destination directory", "profile alias", "user file alias", "SpecTE alias", "unrelated symlink", "unrelated hardlink", "existing graph"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == UNCHANGED x\nP == <>[](x = 0)", "INIT Init\nNEXT Next\nPROPERTY P\nCHECK_DEADLOCK FALSE\n")
			path := filepath.Join(dir, "out_liveness.dot")
			args := []string{"-tool", "-fp", "0", "-workers", "1", "-lncheck", "final", "-dump", "dot", "out.dot", "M.tla"}
			switch mode {
			case "input alias":
				if err := os.Link(filepath.Join(dir, "M.tla"), path); err != nil {
					t.Skipf("hard links unavailable: %v", err)
				}
			case "destination directory":
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			case "profile alias":
				args = append([]string{"-debug-cpu-profile", "out_liveness.dot"}, args...)
			case "user file alias":
				args = append([]string{"-userFile", "out_liveness.dot"}, args...)
			case "SpecTE alias":
				if err := os.WriteFile(filepath.Join(dir, "SpecTE.tla"), []byte("existing"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Link(filepath.Join(dir, "SpecTE.tla"), path); err != nil {
					t.Skipf("hard links unavailable: %v", err)
				}
				args = append([]string{"-generateSpecTE"}, args...)
			case "unrelated symlink", "unrelated hardlink":
				if err := os.WriteFile(filepath.Join(dir, "other"), []byte("preserve"), 0o600); err != nil {
					t.Fatal(err)
				}
				var linkErr error
				if mode == "unrelated symlink" {
					linkErr = os.Symlink("other", path)
				} else {
					linkErr = os.Link(filepath.Join(dir, "other"), path)
				}
				if linkErr != nil {
					t.Skipf("links unavailable: %v", linkErr)
				}
			case "existing graph":
				if err := os.WriteFile(path, []byte("old graph"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			original, err := os.ReadFile(filepath.Join(dir, "M.tla"))
			if err != nil {
				t.Fatal(err)
			}
			out, code, stderr := runGoInDir(t, dir, args)
			if code != 0 || stderr != "" {
				t.Fatalf("exit %d stderr %q: %s", code, stderr, out)
			}
			got, err := os.ReadFile(filepath.Join(dir, "M.tla"))
			if err != nil || !bytes.Equal(got, original) {
				t.Fatalf("input changed: %q, %v", got, err)
			}
			if mode == "destination directory" {
				graph, err := os.ReadFile(filepath.Join(dir, "out.dot"))
				if err != nil || !strings.HasSuffix(string(graph), "}") {
					t.Fatalf("normal graph incomplete: %q, %v", graph, err)
				}
			}
			if mode == "unrelated symlink" || mode == "unrelated hardlink" {
				got, err := os.ReadFile(filepath.Join(dir, "other"))
				if err != nil || !strings.HasPrefix(string(got), "digraph DiskGraph") {
					t.Fatalf("linked graph was not written: %q, %v", got, err)
				}
			}
			if mode == "existing graph" {
				got, err := os.ReadFile(path)
				if err != nil || !strings.HasPrefix(string(got), "digraph DiskGraph") {
					t.Fatalf("liveness graph was not replaced: %q, %v", got, err)
				}
			}
			if mode == "profile alias" {
				got, err := os.ReadFile(path)
				if err != nil || len(got) < 2 || got[0] != 0x1f || got[1] != 0x8b {
					t.Fatalf("CPU profile was overwritten: %q, %v", got, err)
				}
			}
			if mode == "user file alias" {
				got, err := os.ReadFile(path)
				if err != nil || len(got) != 0 {
					t.Fatalf("user output file was overwritten: %q, %v", got, err)
				}
			}
			if mode == "SpecTE alias" {
				got, err := os.ReadFile(filepath.Join(dir, "SpecTE.tla"))
				if err != nil || string(got) != "existing" {
					t.Fatalf("SpecTE output was overwritten: %q, %v", got, err)
				}
			}
			files, err := filepath.Glob(filepath.Join(dir, ".tla-liveness-*.tmp"))
			if err != nil || len(files) != 0 {
				t.Fatalf("staging files left behind: %v, %v", files, err)
			}
		})
	}
}

func TestJavaTLCLivenessDOTLinks(t *testing.T) {
	jar := maxSetOracleJar(t)
	if jar == "" {
		t.Skip("set TLC_JAR to the pinned Java TLC 1.7.4 jar")
	}
	for _, kind := range []string{"symlink", "hardlink"} {
		t.Run(kind, func(t *testing.T) {
			javaDir, goDir := t.TempDir(), t.TempDir()
			for _, dir := range []string{javaDir, goDir} {
				writeDOTFixture(t, dir, "VARIABLE x\nInit == x = 0\nNext == UNCHANGED x\nP == <>[](x = 0)", "INIT Init\nNEXT Next\nPROPERTY P\nCHECK_DEADLOCK FALSE\n")
				createLivenessTestLink(t, dir, kind)
			}
			args := []string{"-tool", "-fp", "0", "-workers", "1", "-lncheck", "final", "-dump", "dot", "out.dot", "M.tla"}
			before, err := os.Stat(filepath.Join(goDir, "target"))
			if err != nil {
				t.Fatal(err)
			}
			_, javaCode := runJavaTLCArgs(t, jar, javaDir, args)
			out, code, stderr := runGoInDir(t, goDir, args)
			if code != javaCode || stderr != "" {
				t.Fatalf("Java exit %d, Go exit %d stderr %q: %s", javaCode, code, stderr, out)
			}
			got, gotErr := os.ReadFile(filepath.Join(goDir, "target"))
			want, wantErr := os.ReadFile(filepath.Join(javaDir, "target"))
			after, statErr := os.Stat(filepath.Join(goDir, "target"))
			if gotErr != nil || wantErr != nil || statErr != nil || !bytes.Equal(got, want) || !os.SameFile(before, after) {
				t.Fatalf("linked graph mismatch: Go %q (%v), Java %q (%v), stat %v", got, gotErr, want, wantErr, statErr)
			}
		})
	}
}

func createLivenessTestLink(t *testing.T, dir, kind string) {
	t.Helper()
	target, link := filepath.Join(dir, "target"), filepath.Join(dir, "out_liveness.dot")
	if err := os.WriteFile(target, []byte("existing graph"), 0o600); err != nil {
		t.Fatal(err)
	}
	var err error
	if kind == "symlink" {
		err = os.Symlink("target", link)
	} else {
		err = os.Link(target, link)
	}
	if err != nil {
		t.Skipf("links unavailable: %v", err)
	}
}

func TestJavaTLCLivenessDOTUnusedPath(t *testing.T) {
	jar := maxSetOracleJar(t)
	for _, mode := range []string{"safety cycle", "liveness cycle", "safety profile"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == x = 0\nNext == UNCHANGED x\nP == <>[](x = 0)"
			cfg := "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n"
			if mode == "liveness cycle" {
				cfg += "PROPERTY P\n"
			}
			writeDOTFixture(t, dir, body, cfg)
			args := []string{"-tool", "-fp", "0", "-workers", "1", "-lncheck", "final", "-dump", "dot", "out.dot", "M.tla"}
			path := filepath.Join(dir, "out_liveness.dot")
			if mode == "safety profile" {
				args = append([]string{"-debug-cpu-profile", "out_liveness.dot"}, args...)
			} else if err := os.Symlink("out_liveness.dot", path); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			if mode == "safety cycle" && jar != "" {
				_, javaCode := runJavaTLCArgs(t, jar, dir, args)
				if javaCode != 0 {
					t.Fatalf("Java safety check exited %d", javaCode)
				}
			}
			out, code, stderr := runGoInDir(t, dir, args)
			if code != 0 || stderr != "" {
				t.Fatalf("exit %d stderr %q: %s", code, stderr, out)
			}
			if mode == "safety profile" {
				profile, err := os.ReadFile(path)
				if err != nil || len(profile) < 2 || profile[0] != 0x1f || profile[1] != 0x8b {
					t.Fatalf("CPU profile was overwritten: %q, %v", profile, err)
				}
			} else if target, err := os.Readlink(path); err != nil || target != "out_liveness.dot" {
				t.Fatalf("unused symlink changed: %q, %v", target, err)
			}
		})
	}
}

func TestLivenessDOTUnsupportedPreservesExistingFile(t *testing.T) {
	for _, tc := range []struct{ name, body, config string }{
		{"VIEW", "V == x", "VIEW V\n"},
		{"effectful action", `A == PrintT(x)`, "CONSTRAINT A\n"},
		{"tableau", `Q == [](x = 0 => <>(x = 1))`, "PROPERTY Q\n"},
		{"multiple solutions", `Q == [](x = 0 => <>(x = 1)) /\ [](x = 1 => <>(x = 2))`, "PROPERTY Q\n"},
		{"invariant failure", `I == x = 0`, "INVARIANT I\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "VARIABLE x\nInit == x = 0\nNext == x' \\in 0..2\nP == <>[](x = 0)\n" + tc.body
			cfg := "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n"
			if tc.name != "tableau" && tc.name != "multiple solutions" {
				cfg += "PROPERTY P\n"
			}
			cfg += tc.config
			writeDOTFixture(t, dir, body, cfg)
			path := filepath.Join(dir, "out_liveness.dot")
			if err := os.WriteFile(path, []byte("existing"), 0o600); err != nil {
				t.Fatal(err)
			}
			out, code, stderr := runGoInDir(t, dir, []string{"-tool", "-fp", "0", "-workers", "1", "-lncheck", "final", "-dump", "dot", "out.dot", "M.tla"})
			if code != 0 && code != 12 && code != 13 || stderr != "" {
				t.Fatalf("exit %d stderr %q: %s", code, stderr, out)
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != "existing" {
				t.Fatalf("existing liveness graph changed: %q, %v", got, err)
			}
		})
	}
}
