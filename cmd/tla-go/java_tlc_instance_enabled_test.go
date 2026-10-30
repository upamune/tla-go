package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// test55 is unchanged from Java TLC v1.7.4, commit 5a47802.
func TestJavaTLCEnabledInstance(t *testing.T) {
	for _, tc := range []struct {
		name, stats string
		files       map[string]string
	}{
		{name: "test55", stats: "3 states generated, 2 distinct states found, 0 states left on queue."},
		{name: "M", stats: "2 states generated, 1 distinct states found, 0 states left on queue.", files: map[string]string{
			"M.tla":     "---- MODULE M ----\nVARIABLE z\nI == INSTANCE Inner WITH x <- z, y <- z\nJ == INSTANCE Inner WITH x <- z, y <- z\nInit == z = 0\nNext == UNCHANGED z\nInv == /\\ I!F /\\ I!H /\\ J!F /\\ ~I!G /\\ I!R /\\ ~ENABLED I!A /\\ ~ENABLED J!A\n====\n",
			"M.cfg":     "INIT Init\nNEXT Next\nINVARIANT Inv\n",
			"Inner.tla": "---- MODULE Inner ----\nVARIABLES x, y\nA == /\\ x' = 0 /\\ y' = 1\nF == ENABLED A\nH == ENABLED (LET L == A IN L)\nG == ENABLED (x # y /\\ x' = 0 /\\ y' = 1)\nR == ENABLED (x' = 0 /\\ y' = 1 /\\ x' = 0)\n====\n",
		}},
		{name: "Nested", stats: "2 states generated, 1 distinct states found, 0 states left on queue.", files: map[string]string{
			"Nested.tla": "---- MODULE Nested ----\nVARIABLE z\nI == INSTANCE Middle WITH p <- z, q <- z\nInit == z = 0\nNext == UNCHANGED z\nInv == /\\ ~I!F /\\ I!H /\\ ~ENABLED I!J!A(1)\n====\n",
			"Nested.cfg": "INIT Init\nNEXT Next\nINVARIANT Inv\n",
			"Middle.tla": "---- MODULE Middle ----\nVARIABLES p, q\nJ == INSTANCE Inner WITH x <- p, y <- q\nF == ENABLED J!A(1)\nH == J!E\n====\n",
			"Inner.tla":  "---- MODULE Inner ----\nVARIABLES x, y\nA(k) == /\\ x' = 0 /\\ y' = k\nE == ENABLED A(1)\n====\n",
		}},
		{name: "Local", stats: "2 states generated, 1 distinct states found, 0 states left on queue.", files: map[string]string{
			"Local.tla": "---- MODULE Local ----\nVARIABLE z\nInit == z = 0\nNext == UNCHANGED z\nInv == LET I == INSTANCE Inner WITH x <- z, y <- z IN /\\ I!F /\\ ~ENABLED I!A\n====\n",
			"Local.cfg": "INIT Init\nNEXT Next\nINVARIANT Inv\n",
			"Inner.tla": "---- MODULE Inner ----\nVARIABLES x, y\nA == /\\ x' = 0 /\\ y' = 1\nF == ENABLED A\n====\n",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.files == nil {
				tc.files = map[string]string{}
				for _, name := range []string{"test55.tla", "test55a.tla", "test55.cfg"} {
					data, err := os.ReadFile(filepath.Join("testdata", "java-tlc", name))
					if err != nil {
						t.Fatal(err)
					}
					tc.files[name] = string(data)
				}
			}
			for name, content := range tc.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errout bytes.Buffer
			code := run(context.Background(), []string{filepath.Join(dir, tc.name+".tla")}, &out, &errout)
			if code != 0 || errout.Len() != 0 || !strings.Contains(out.String(), tc.stats) {
				t.Fatalf("exit %d, stdout:\n%s\nstderr:\n%s", code, out.String(), errout.String())
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				jar, err := filepath.Abs(jar)
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, "java", "-Djava.io.tmpdir="+t.TempDir(), "-Xmx256m", "-XX:+UseParallelGC", "-jar", jar, "-workers", "1", "-fp", "0", tc.name+".tla")
				cmd.Dir = dir
				java, err := cmd.CombinedOutput()
				if err != nil || !strings.Contains(string(java), tc.stats) {
					t.Fatalf("Java TLC %v:\n%s", err, java)
				}
			}
		})
	}
}
