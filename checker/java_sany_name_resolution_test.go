package checker

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// These original, minimal models characterize SANY and TLC 1.7.4 at upstream
// commit 5a47802b5c391f59ecdd44117981f4ff8c0656ba. They are not upstream fixtures.
// A gap case checks the Java result but deliberately does not enshrine Go's
// incorrect acceptance; it can start passing Go validation after a real fix.
func TestJavaTLCNameResolutionOrder(t *testing.T) {
	cases := []struct {
		name, definitions string
		invalid, gap      bool
	}{
		{"unused forward call", "Bad == Later(1)\nLater(x) == x", true, true},
		{"unused self call", "Bad(x) == Bad(x)", true, true},
		{"forward operator argument", "Use(F(_)) == F(1)\nBad == Use(Later)\nLater(x) == x", true, true},
		{"forward let", "Bad == LET F == G\n           G == 1\n       IN F", true, true},
		{"unknown name", "Bad == Missing", true, false},
		{"wrong arity", "F(x) == x\nBad == F(1, 2)", true, false},
		{"wrong operator argument arity", "G(x, y) == x\nUse(F(_)) == F(1)\nBad == Use(G)", true, false},
		{"builtin arity", "Bad == Len(<<1>>, <<2>>)", true, false},
		{"existing let name", "F == 2\nBad == LET F == 1 IN F", true, false},
		{"backward call", "F(x) == x\nGood == F(1)", false, false},
		{"recursive declaration", "RECURSIVE F(_)\nF(x) == IF x = 0 THEN 0 ELSE F(x)", false, false},
		{"operator argument", "F(x) == x\nUse(G(_)) == G(1)\nGood == Use(F)", false, false},
		{"backward let", "Good == LET G == 1\n            F == G\n        IN F", false, false},
		{"let before module name", "Good == LET F == 1 IN F\nF == 2", false, false},
		{"builtin", "Good == Len(<<1>>)", false, false},
		{"instance", "I == INSTANCE A\nGood == I!F(1)", false, false},
		{"instance arity", "I == INSTANCE A\nBad == I!F(1, 2)", true, false},
		{"instance before declaration", "Bad == I!F(1)\nI == INSTANCE A", true, false},
	}
	jar := os.Getenv("TLC_JAR")
	if jar != "" {
		data, err := os.ReadFile(jar)
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != "936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88" {
			t.Fatalf("TLC_JAR is not pinned 1.7.4: %s", got)
		}
		jar, err = filepath.Abs(jar)
		if err != nil {
			t.Fatal(err)
		}
	}
	const auxiliary = "---- MODULE A ----\nF(x) == x\n====\n"
	const cfg = "INIT Init\nNEXT Next\n"
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS Naturals, Sequences\n" + tc.definitions + "\nVARIABLE v\nInit == v = 0\nNext == UNCHANGED v\n====\n"
			loader := func(_ context.Context, name string) (string, error) {
				if name != "A" {
					return "", os.ErrNotExist
				}
				return auxiliary, nil
			}
			result, err := CheckWithLoader(context.Background(), source, cfg, loader)
			if tc.gap {
				if err == nil {
					t.Log("known gap: Go accepted a Java semantic error")
				}
			} else if tc.invalid {
				if err == nil {
					t.Fatalf("Go accepted invalid module: %+v", result)
				}
			} else if err != nil || result.Status != Success {
				t.Fatalf("Go rejected valid module: %+v, %v", result, err)
			}
			if jar == "" {
				return
			}
			dir := t.TempDir()
			for name, contents := range map[string]string{"M.tla": source, "M.cfg": cfg, "A.tla": auxiliary} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			sany, sanyCode := runNameResolutionJava(t, dir, "-cp", jar, "tla2sany.SANY", "M.tla")
			if sanyCode != 0 || strings.Contains(sany, "Semantic errors:") != tc.invalid || strings.Contains(sany, "Parse Error") {
				t.Fatalf("SANY exit %d, invalid %t:\n%s", sanyCode, tc.invalid, sany)
			}
			if tc.gap && !strings.Contains(sany, "Unknown operator:") {
				t.Fatalf("SANY did not report an unknown operator:\n%s", sany)
			}
			tlc, tlcCode := runNameResolutionJava(t, dir, "-XX:+UseParallelGC", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "M.tla")
			wantCode := 0
			if tc.invalid {
				wantCode = 150
			}
			if tlcCode != wantCode || strings.Contains(tlc, "@!@!@STARTMSG 3002:1") != tc.invalid {
				t.Fatalf("TLC exit %d, want %d:\n%s", tlcCode, wantCode, tlc)
			}
		})
	}
}

func runNameResolutionJava(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "java", append([]string{"-Xmx256m"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("Java timed out: %v\n%s", ctx.Err(), out)
	}
	if err == nil {
		return string(out), 0
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return string(out), exit.ExitCode()
	}
	t.Fatalf("run Java: %v", err)
	return "", -1
}
