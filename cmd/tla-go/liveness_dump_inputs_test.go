package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLivenessDOTPreservesInputModules(t *testing.T) {
	for _, mode := range []string{"extended symlink", "extended hardlink", "instance", "builtin module", "library root"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			root := dir
			extends, extra, next := "Naturals, Child", "", `x' = (x + Y) % 2`
			target := filepath.Join(dir, "Child.tla")
			child := "---- MODULE Child ----\nY == 1\n====\n"
			if mode == "instance" {
				extends, extra, next = "Naturals", "I == INSTANCE Child\n", `x' = (x + I!Y) % 2`
			}
			if mode == "builtin module" {
				extends, next, target = "Naturals", `x' \in 0..2`, filepath.Join(dir, "Naturals.tla")
				child = "---- MODULE Naturals ----\n====\n"
			}
			if mode == "library root" {
				root = filepath.Join(dir, "library")
				if err := os.Mkdir(root, 0o700); err != nil {
					t.Fatal(err)
				}
				extends, next, target = "Naturals", `x' \in 0..2`, filepath.Join(root, "M.tla")
			}
			source := "---- MODULE M ----\nEXTENDS " + extends + "\nVARIABLE x\n" + extra + "Init == x = 0\nNext == " + next + "\nP == []<>(x = 0)\n====\n"
			writeLivenessInput(t, filepath.Join(root, "M.tla"), source)
			writeLivenessInput(t, filepath.Join(root, "M.cfg"), "INIT Init\nNEXT Next\nPROPERTY P\nCHECK_DEADLOCK FALSE\n")
			if mode != "library root" {
				writeLivenessInput(t, target, child)
			}
			before, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(dir, "out_liveness.dot")
			if mode == "extended hardlink" {
				err = os.Link(target, link)
			} else {
				err = os.Symlink(target, link)
			}
			if err != nil {
				t.Skipf("links unavailable: %v", err)
			}
			args := []string{"-tool", "-workers", "1", "-fp", "0", "-lncheck", "final", "-dump", "dot", "out.dot"}
			if mode == "library root" {
				args = append(args, "-I", "library", "-config", "M")
			}
			out, code, stderr := runGoInDir(t, dir, append(args, "M.tla"))
			if code != 0 && code != 13 || stderr != "" {
				t.Fatalf("exit %d stderr %q: %s", code, stderr, out)
			}
			after, err := os.ReadFile(target)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("input module changed: %q, %v", after, err)
			}
		})
	}
}

func writeLivenessInput(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
