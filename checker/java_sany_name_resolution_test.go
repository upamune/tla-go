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

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
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
		{"unused self call", "Bad(x) == Bad(x)", true, false},
		{"unused self value", "Bad == Bad", true, false},
		{"self as operator argument", "Use(F(_)) == F(1)\nBad(x) == Use(Bad)", true, false},
		{"let self call", "Bad == LET F(x) == F(x) IN 1", true, false},
		{"forward operator argument", "Use(F(_)) == F(1)\nBad == Use(Later)\nLater(x) == x", true, true},
		{"forward let", "Bad == LET F == G\n           G == 1\n       IN F", true, true},
		{"unknown name", "Bad == Missing", true, false},
		{"wrong arity", "F(x) == x\nBad == F(1, 2)", true, false},
		{"wrong operator argument arity", "G(x, y) == x\nUse(F(_)) == F(1)\nBad == Use(G)", true, false},
		{"builtin arity", "Bad == Len(<<1>>, <<2>>)", true, false},
		{"existing let name", "F == 2\nBad == LET F == 1 IN F", true, false},
		{"duplicate let in hidden definition", "F == LET F == 1\n             F == 2\n         IN F", true, false},
		{"local operator parameter matches own name", "F == LET F(F) == F IN F(1)", true, false},
		{"local operator parameter matches own name with arity", "F == LET F(F(_)) == F(1) IN F(LAMBDA x: x)", true, false},
		{"later parameter matches local operator", "F == LET F(x, F) == x IN F(1, 2)", true, false},
		{"recursive local parameter matches own name", "F == LET RECURSIVE F(_) F(F) == F IN F(1)", true, false},
		{"different local operator parameter matches own name", "F == LET G(G) == G IN G(1)", true, false},
		{"duplicate bound in hidden definition", "F == \\A F \\in {1}, F \\in {1}: F = 1", true, false},
		{"nested conflict with local name", "F == LET F == 1\n             G == LET F == 2 IN F\n         IN G", true, false},
		{"recursive name remains visible", "RECURSIVE F(_)\nF(x) == LET F == 1 IN F", true, false},
		{"other existing bound name", "G == 1\nF == \\A G \\in {1}: G = 1", true, false},
		{"backward call", "F(x) == x\nGood == F(1)", false, false},
		{"recursive declaration", "RECURSIVE F(_)\nF(x) == IF x = 0 THEN 0 ELSE F(x)", false, false},
		{"recursive let", "Good == LET RECURSIVE F(_)\n                F(x) == IF x = 0 THEN 0 ELSE F(x-1)\n            IN F(1)", false, false},
		{"mutual recursion", "RECURSIVE Even(_), Odd(_)\nEven(x) == IF x = 0 THEN TRUE ELSE Odd(x-1)\nOdd(x) == IF x = 0 THEN FALSE ELSE Even(x-1)\nGood == Even(2)", false, false},
		{"mutual recursion let", "Good == LET RECURSIVE Even(_), Odd(_)\n                Even(x) == IF x = 0 THEN TRUE ELSE Odd(x-1)\n                Odd(x) == IF x = 0 THEN FALSE ELSE Even(x-1)\n            IN Even(2)", false, false},
		{"same name let", "F == LET F == 1 IN F", false, false},
		{"same name bound", "F == \\A F \\in {1}: F = 1", false, false},
		{"same name local parameter", "F == LET G(F) == F IN G(1)", false, false},
		{"same name local operator distinct parameter", "F == LET F(x) == x IN F(1)", false, false},
		{"same name local operator distinct operator parameter", "F == LET F(G(_)) == G(1) IN F(LAMBDA x: x)", false, false},
		{"same name recursive local operator", "F == LET RECURSIVE F(_) F(x) == IF x = 0 THEN 0 ELSE F(x-1) IN F(1)", false, false},
		{"recursive function", "F[x \\in 0..1] == IF x = 0 THEN 0 ELSE F[x-1]", false, false},
		{"bound name", "Good == \\A x \\in {1}: x = 1", false, false},
		{"operator argument", "F(x) == x\nUse(G(_)) == G(1)\nGood == Use(F)", false, false},
		{"backward let", "Good == LET G == 1\n            F == G\n        IN F", false, false},
		{"let before module name", "Good == LET F == 1 IN F\nF == 2", false, false},
		{"builtin", "Good == Len(<<1>>)", false, false},
		{"instance", "I == INSTANCE A\nGood == I!F(1)", false, false},
		{"instance with substitution", "I == INSTANCE B WITH C <- 1\nGood == I!F", false, false},
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
	const substituted = "---- MODULE B ----\nCONSTANT C\nF == C\n====\n"
	const cfg = "INIT Init\nNEXT Next\n"
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS Naturals, Sequences\n" + tc.definitions + "\nVARIABLE v\nInit == v = 0\nNext == UNCHANGED v\n====\n"
			loader := func(_ context.Context, name string) (string, error) {
				if name == "B" {
					return substituted, nil
				}
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
			module, err := parser.ParseModule(source)
			if err != nil {
				t.Fatal(err)
			}
			parsedConfig, err := config.Parse(cfg)
			if err != nil {
				t.Fatal(err)
			}
			direct, directErr := CheckModuleWithLoader(context.Background(), module, parsedConfig, loader)
			if !tc.gap && ((directErr != nil) != tc.invalid || !tc.invalid && direct.Status != Success) {
				t.Fatalf("direct API: result %+v, error %v, invalid %t", direct, directErr, tc.invalid)
			}
			if jar == "" {
				return
			}
			dir := t.TempDir()
			for name, contents := range map[string]string{"M.tla": source, "M.cfg": cfg, "A.tla": auxiliary, "B.tla": substituted} {
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

func TestSameNameBindingsEvaluate(t *testing.T) {
	for _, definition := range []string{
		"F == LET F == 1 IN F",
		"F == LET G(F) == F IN G(1)",
		"F == LET F(x) == x IN F(1)",
		"F == LET RECURSIVE F(_) F(x) == IF x = 0 THEN 1 ELSE F(x-1) IN F(1)",
	} {
		t.Run(definition, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS Naturals\n" + definition + "\nVARIABLE v\nInit == v = F\nNext == UNCHANGED v\nInv == v = 1\n====\n"
			result, err := Check(source, "INIT Init\nNEXT Next\nINVARIANT Inv\n")
			if err != nil || result.Status != Success {
				t.Fatalf("result %+v, error %v", result, err)
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
