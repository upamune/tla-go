package checker

import (
	"context"
	"crypto/sha256"
	"errors"
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
func TestJavaTLCNameResolutionOrder(t *testing.T) {
	cases := []struct {
		name, definitions string
		invalid           bool
	}{
		{"unused forward call", "Bad == Later(1)\nLater(x) == x", true},
		{"same line forward", "Bad == Later(1) Later(x) == x", true},
		{"same line backward", "Earlier(x) == x Good == Earlier(1)", false},
		{"local forward", "LOCAL Bad == Later(1)\nLater(x) == x", true},
		{"forward variable", "Bad == v", true},
		{"forward constant", "Bad == Later\nCONSTANT Later", true},
		{"forward assumption", "ASSUME Later = 1\nLater == 1", true},
		{"forward theorem", "THEOREM Later = 1\nLater == 1", true},
		{"unused self call", "Bad(x) == Bad(x)", true},
		{"unused self value", "Bad == Bad", true},
		{"self as operator argument", "Use(F(_)) == F(1)\nBad(x) == Use(Bad)", true},
		{"let self call", "Bad == LET F(x) == F(x) IN 1", true},
		{"forward operator argument", "Use(F(_)) == F(1)\nBad == Use(Later)\nLater(x) == x", true},
		{"forward let", "Bad == LET F == G\n           G == 1\n       IN F", true},
		{"nested let references later parent", "Bad == LET A == LET X == B IN X B == 1 IN A", true},
		{"nested let shadows later parent", "Good == LET X == LET B == 2 IN B B == 1 IN X", false},
		{"forward selector", "Bad == Later!lab\nLater == lab :: 1", true},
		{"backward selector", "Earlier == lab :: 1\nGood == Earlier!lab", false},
		{"forward numbered selector", "Bad == Later!1\nLater == 1 + 1", true},
		{"let forward selector", "Bad == LET F == G!lab G == lab :: 1 IN F", true},
		{"let backward selector", "Good == LET G == lab :: 1 F == G!lab IN F", false},
		{"recursive forward selector", "RECURSIVE F, G\nF == G!lab\nG == lab :: 1", true},
		{"recursive own selector", "RECURSIVE F\nF == lab :: F!lab", true},
		{"unknown name", "Bad == Missing", true},
		{"wrong arity", "F(x) == x\nBad == F(1, 2)", true},
		{"wrong operator argument arity", "G(x, y) == x\nUse(F(_)) == F(1)\nBad == Use(G)", true},
		{"builtin arity", "Bad == Len(<<1>>, <<2>>)", true},
		{"existing let name", "F == 2\nBad == LET F == 1 IN F", true},
		{"duplicate let in hidden definition", "F == LET F == 1\n             F == 2\n         IN F", true},
		{"local operator parameter matches own name", "F == LET F(F) == F IN F(1)", true},
		{"local operator parameter matches own name with arity", "F == LET F(F(_)) == F(1) IN F(LAMBDA x: x)", true},
		{"later parameter matches local operator", "F == LET F(x, F) == x IN F(1, 2)", true},
		{"recursive local parameter matches own name", "F == LET RECURSIVE F(_) F(F) == F IN F(1)", true},
		{"different local operator parameter matches own name", "F == LET G(G) == G IN G(1)", true},
		{"duplicate bound in hidden definition", "F == \\A F \\in {1}, F \\in {1}: F = 1", true},
		{"nested conflict with local name", "F == LET F == 1\n             G == LET F == 2 IN F\n         IN G", true},
		{"recursive name remains visible", "RECURSIVE F(_)\nF(x) == LET F == 1 IN F", true},
		{"other existing bound name", "G == 1\nF == \\A G \\in {1}: G = 1", true},
		{"backward call", "F(x) == x\nGood == F(1)", false},
		{"recursive declaration", "RECURSIVE F(_)\nF(x) == IF x = 0 THEN 0 ELSE F(x)", false},
		{"recursive declaration too late", "A(x) == B(x)\nRECURSIVE B(_)\nB(x) == x", true},
		{"unrelated recursive declaration", "RECURSIVE A(_)\nA(x) == B(x)\nB(x) == x", true},
		{"recursive declaration visible before definition", "RECURSIVE F(_)\nGood == F(1)\nF(x) == x", false},
		{"recursive declaration not yet visible", "Bad == F(1)\nRECURSIVE F(_)\nF(x) == x", true},
		{"same line mutual recursion", "RECURSIVE A(_), B(_) A(x) == B(x) B(x) == x", false},
		{"recursive let", "Good == LET RECURSIVE F(_)\n                F(x) == IF x = 0 THEN 0 ELSE F(x-1)\n            IN F(1)", false},
		{"mutual recursion", "RECURSIVE Even(_), Odd(_)\nEven(x) == IF x = 0 THEN TRUE ELSE Odd(x-1)\nOdd(x) == IF x = 0 THEN FALSE ELSE Even(x-1)\nGood == Even(2)", false},
		{"mutual recursion let", "Good == LET RECURSIVE Even(_), Odd(_)\n                Even(x) == IF x = 0 THEN TRUE ELSE Odd(x-1)\n                Odd(x) == IF x = 0 THEN FALSE ELSE Even(x-1)\n            IN Even(2)", false},
		{"same name let", "F == LET F == 1 IN F", false},
		{"same name bound", "F == \\A F \\in {1}: F = 1", false},
		{"same name local parameter", "F == LET G(F) == F IN G(1)", false},
		{"same name local operator distinct parameter", "F == LET F(x) == x IN F(1)", false},
		{"same name local operator distinct operator parameter", "F == LET F(G(_)) == G(1) IN F(LAMBDA x: x)", false},
		{"same name recursive local operator", "F == LET RECURSIVE F(_) F(x) == IF x = 0 THEN 0 ELSE F(x-1) IN F(1)", false},
		{"recursive function", "F[x \\in 0..1] == IF x = 0 THEN 0 ELSE F[x-1]", false},
		{"bound name", "Good == \\A x \\in {1}: x = 1", false},
		{"operator argument", "F(x) == x\nUse(G(_)) == G(1)\nGood == Use(F)", false},
		{"backward let", "Good == LET G == 1\n            F == G\n        IN F", false},
		{"let before module name", "Good == LET F == 1 IN F\nF == 2", false},
		{"builtin", "Good == Len(<<1>>)", false},
		{"instance", "I == INSTANCE A\nGood == I!F(1)", false},
		{"instance with substitution", "I == INSTANCE B WITH C <- 1\nGood == I!F", false},
		{"instance with forward substitution", "I == INSTANCE B WITH C <- Later\nLater == 1", true},
		{"instance with backward substitution", "Earlier == 1\nI == INSTANCE B WITH C <- Earlier", false},
		{"let instance before definition", "Good == LET I == INSTANCE A F == I!F(1) IN F", false},
		{"instance arity", "I == INSTANCE A\nBad == I!F(1, 2)", true},
		{"instance before declaration", "Bad == I!F(1)\nI == INSTANCE A", true},
		{"same line instance before declaration", "Bad == I!F(1) I == INSTANCE A", true},
		{"same line instance after declaration", "I == INSTANCE A Good == I!F(1)", false},
		{"local instance shadows later instance", "Good == LET I == INSTANCE A X == I!F(1) IN X I == INSTANCE A", false},
		{"nested local instance shadows later instance", "Good == LET X == LET I == INSTANCE A Y == I!F(1) IN Y IN X I == INSTANCE A", false},
		{"local instance operator argument shadows later instance", "Use(F(_)) == F(1) Good == LET I == INSTANCE A X == Use(I!F) IN X I == INSTANCE A", false},
		{"local instance referenced before shadow", "Bad == LET X == I!F(1) I == INSTANCE A IN X I == INSTANCE A", true},
		{"nested local instance referenced before shadow", "Bad == LET X == LET Y == I!F(1) I == INSTANCE A IN Y IN X I == INSTANCE A", true},
		{"local instance self substitution before outer", "Bad == LET I == INSTANCE B WITH C <- I!F IN I!F I == INSTANCE B", true},
		{"local instance referenced in prior definition", "Bad == LET X == I!F I == INSTANCE B WITH C <- 1 IN X I == INSTANCE B", true},
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
			forwardName := map[string]string{
				"unused forward call": "Later",
				"same line forward":   "Later",
				"forward let":         "G",
			}[tc.name]
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
			if tc.invalid {
				if err == nil {
					t.Fatalf("Go accepted invalid module: %+v", result)
				}
				if forwardName != "" {
					if _, ok := errors.AsType[*SemanticError](err); !ok || !strings.Contains(err.Error(), "undefined symbol "+forwardName) {
						t.Fatalf("Go returned the wrong forward-reference error: %v", err)
					}
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
			if (directErr != nil) != tc.invalid || !tc.invalid && direct.Status != Success {
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
			if forwardName != "" && !strings.Contains(sany, "Unknown operator: `"+forwardName+"'.") {
				t.Fatalf("SANY returned the wrong forward-reference error:\n%s", sany)
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
