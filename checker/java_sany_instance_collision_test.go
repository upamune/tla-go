package checker

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/upamune/tla-go/syntax/parser"
)

// Original models tested against SANY and TLC 1.7.4 at upstream 5a47802b.
func TestJavaTLCInstanceCollisions(t *testing.T) {
	jar := instanceCollisionOracle(t)
	const a = "---- MODULE A ----\nF(x) == 1\n====\n"
	const b = "---- MODULE B ----\nEXTENDS Naturals\nF(x) == 2\nG(x) == F(x) + 10\n====\n"
	for _, tc := range []struct {
		name, root, want, auxiliary string
		invalid                     bool
	}{
		{"anonymous same module", "INSTANCE A\nINSTANCE A\nY == F(1)", "1", b, false},
		{"anonymous collisions retain internal binding", "INSTANCE A\nINSTANCE B\nY == <<F(1), G(1)>>", "<<1, 12>>", b, false},
		{"anonymous reverse order", "INSTANCE B\nINSTANCE A\nY == <<F(1), G(1)>>", "<<2, 12>>", b, false},
		{"local anonymous collision", "INSTANCE A\nLOCAL INSTANCE B\nY == <<F(1), G(1)>>", "<<1, 12>>", b, false},
		{"let collision retains internal binding", "I == INSTANCE A\nY == LET I == INSTANCE B IN <<I!F(1), I!G(1)>>", "<<1, 12>>", b, false},
		{"let before and after", "I == INSTANCE A\nY == LET X == I!F(1) I == INSTANCE B IN <<X, I!F(1), I!G(1)>>", "<<1, 1, 12>>", b, false},
		{"let higher order", "I == INSTANCE A\nUse(Op(_)) == Op(1)\nY == LET I == INSTANCE B IN Use(I!F)", "1", b, false},
		{"let local collision", "I == INSTANCE A\nY == LET LOCAL I == INSTANCE B IN <<I!F(1), I!G(1)>>", "<<1, 12>>", b, false},
		{"duplicate named instances", "I == INSTANCE A\nI == INSTANCE B\nY == <<I!F(1), I!G(1)>>", "<<1, 12>>", b, false},
		{"duplicate let instances", "Y == LET I == INSTANCE A I == INSTANCE B IN <<I!F(1), I!G(1)>>", "<<1, 12>>", b, false},
		{"duplicate instance arity mismatch", "I(x) == INSTANCE A\nI == INSTANCE B\nY == 1", "1", b, true},
		{"outer instance arity mismatch without common exports", "I == INSTANCE A\nY == LET I(z) == INSTANCE B IN 1", "1", "---- MODULE B ----\nG == 2\n====\n", true},
		{"outer definition conflicts with local instance", "I == 1\nY == LET I == INSTANCE B IN 1", "1", "---- MODULE B ----\nG == 2\n====\n", true},
		{"root definition conflicts with instance", "I == 1\nI == INSTANCE A\nY == 1", "1", b, true},
		{"let definition conflicts with instance", "Y == LET I == 1 I == INSTANCE A IN 1", "1", b, true},
		{"outer instance arity match without common exports", "I == INSTANCE A\nY == LET I == INSTANCE B IN I!G", "2", "---- MODULE B ----\nG == 2\n====\n", false},
		{"enclosing definition is not yet visible", "I == LET I == INSTANCE A IN I!F(1)\nY == I", "1", b, false},
		{"nested instance arity mismatch", "Y == LET I == INSTANCE A IN LET I(z) == INSTANCE B IN 1", "1", "---- MODULE B ----\nG == 2\n====\n", true},
		{"let nested", "I == INSTANCE A\nY == LET I == INSTANCE B IN LET I == INSTANCE A IN I!F(1)", "1", b, false},
		{"arity mismatch", "INSTANCE A\nINSTANCE B\nY == 1", "1", "---- MODULE B ----\nF(x, y) == x\n====\n", true},
		{"with substitution", "INSTANCE A\nINSTANCE B WITH C <- 2\nY == <<F(1), G(1)>>", "<<1, 12>>", "---- MODULE B ----\nEXTENDS Naturals\nCONSTANT C\nF(x) == C\nG(x) == F(x) + 10\n====\n", false},
		{"with prior imported substitution", "INSTANCE A\nINSTANCE B WITH C <- F(1)\nY == <<F(1), G(1)>>", "<<1, 11>>", "---- MODULE B ----\nEXTENDS Naturals\nCONSTANT C\nF(x) == C\nG(x) == F(x) + 10\n====\n", false},
		{"recursive internal collision", "INSTANCE A\nINSTANCE B\nY == <<F(1), G(1)>>", "<<1, 12>>", "---- MODULE B ----\nEXTENDS Naturals\nRECURSIVE F(_)\nF(x) == IF x = 0 THEN 2 ELSE F(x - 1)\nG(x) == F(x) + 10\n====\n", false},
		{"existing ordinary definition", "F(x) == 3\nINSTANCE A\nY == F(1)", "3", b, false},
		{"ordinary definition and imported internal binding", "F(x) == 3\nINSTANCE B\nY == <<F(1), G(1)>>", "<<3, 12>>", b, false},
		{"existing operator constant", "CONSTANT F(_)\nINSTANCE A\nY == 1", "1", b, true},
		{"let collision wrong arity", "I == INSTANCE A\nY == LET I == INSTANCE B IN 1", "1", "---- MODULE B ----\nF(x, y) == x\n====\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS Naturals\n" + tc.root + "\nVARIABLE x\nInit == x = Y\nNext == UNCHANGED x\nInv == x = " + tc.want + "\n====\n"
			cfg := "INIT Init\nNEXT Next\nINVARIANT Inv\n"
			loader := func(_ context.Context, name string) (string, error) {
				switch name {
				case "A":
					return a, nil
				case "B":
					return tc.auxiliary, nil
				default:
					return "", os.ErrNotExist
				}
			}
			if jar != "" {
				dir := t.TempDir()
				for name, data := range map[string]string{"M.tla": source, "M.cfg": cfg, "A.tla": a, "B.tla": tc.auxiliary} {
					if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				checkCollisionSANY(t, dir, jar, tc.invalid)
				out, code := runNameResolutionJava(t, dir, "-XX:+UseParallelGC", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "M.tla")
				want := 0
				if tc.invalid {
					want = 150
				}
				if code != want {
					t.Fatalf("Java exit=%d want=%d output=%s", code, want, out)
				}
				if !tc.invalid && strings.Contains(out, "Invariant Inv is violated") {
					t.Fatalf("Java wrong value: %s", out)
				}
			}
			result, err := CheckWithLoader(context.Background(), source, cfg, loader)
			if tc.invalid {
				if err == nil {
					t.Fatalf("accepted invalid model: %+v", result)
				}
				if _, ok := errors.AsType[*SemanticError](err); !ok {
					t.Fatalf("wrong semantic error: %v", err)
				}
				return
			}
			if err != nil || result.Status != Success {
				t.Fatalf("result %+v, error %v", result, err)
			}
		})
	}
}

func TestJavaTLCInstanceCollisionsThroughModules(t *testing.T) {
	jar := instanceCollisionOracle(t)
	const a = "---- MODULE A ----\nF(x) == 1\n====\n"
	const b = "---- MODULE B ----\nEXTENDS Naturals\nF(x) == 2\nG(x) == F(x) + 10\n====\n"
	for _, tc := range []struct {
		name, root, c string
		invalid       bool
	}{
		{"extended", "EXTENDS C, Naturals\nY == <<F(1), G(1)>>", "INSTANCE A\nINSTANCE B", false},
		{"instantiated", "EXTENDS Naturals\nI == INSTANCE C\nY == <<I!F(1), I!G(1)>>", "INSTANCE A\nINSTANCE B", false},
		{"local not exported", "EXTENDS C, Naturals\nY == G(1)", "INSTANCE A\nLOCAL INSTANCE B", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\n" + tc.root + "\nVARIABLE x\nInit == x = Y\nNext == UNCHANGED x\nInv == x = <<1, 12>>\n====\n"
			c := "---- MODULE C ----\n" + tc.c + "\n====\n"
			cfg := "INIT Init\nNEXT Next\nINVARIANT Inv\n"
			loader := func(_ context.Context, n string) (string, error) {
				switch n {
				case "A":
					return a, nil
				case "B":
					return b, nil
				case "C":
					return c, nil
				default:
					return "", os.ErrNotExist
				}
			}
			if jar != "" {
				dir := t.TempDir()
				for name, data := range map[string]string{"M.tla": source, "M.cfg": cfg, "A.tla": a, "B.tla": b, "C.tla": c} {
					if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				checkCollisionSANY(t, dir, jar, tc.invalid)
				out, code := runNameResolutionJava(t, dir, "-XX:+UseParallelGC", "-jar", jar, "-tool", "-workers", "1", "-fp", "0", "M.tla")
				want := 0
				if tc.invalid {
					want = 150
				}
				if code != want {
					t.Fatalf("Java exit %d want %d: %s", code, want, out)
				}
			}
			result, err := CheckWithLoader(context.Background(), source, cfg, loader)
			if tc.invalid {
				if err == nil {
					t.Fatalf("accepted invalid module: %+v", result)
				}
				if _, ok := errors.AsType[*SemanticError](err); !ok {
					t.Fatalf("expected semantic error: %v", err)
				}
				return
			}
			if err != nil || result.Status != Success {
				t.Fatalf("result %+v error %v", result, err)
			}
		})
	}
}

func checkCollisionSANY(t *testing.T, dir, jar string, invalid bool) {
	t.Helper()
	out, code := runNameResolutionJava(t, dir, "-cp", jar, "tla2sany.SANY", "M.tla")
	if code != 0 || strings.Contains(out, "*** Errors:") != invalid || strings.Contains(out, "Parse Error") {
		t.Fatalf("SANY exit %d invalid %t: %s", code, invalid, out)
	}
}

func TestJavaSANYForwardInstanceCollisionProofKinds(t *testing.T) {
	jar := instanceCollisionOracle(t)
	for _, tc := range []struct {
		name, root, aux string
		invalid         bool
	}{
		{"theorem versus definition", "THEOREM F == TRUE\nINSTANCE A", "F == TRUE", true},
		{"assumption versus definition", "ASSUME F == TRUE\nINSTANCE A", "F == TRUE", true},
		{"definition versus theorem", "F == TRUE\nINSTANCE A", "THEOREM F == TRUE", true},
		{"definition versus assumption", "F == TRUE\nINSTANCE A", "ASSUME F == TRUE", true},
		{"theorem versus theorem", "THEOREM F == TRUE\nINSTANCE A", "THEOREM F == FALSE", false},
		{"assumption versus theorem", "ASSUME F == TRUE\nINSTANCE A", "THEOREM F == FALSE", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\n" + tc.root + "\n====\n"
			aux := "---- MODULE A ----\n" + tc.aux + "\n====\n"
			module, err := parser.ParseModule(source)
			if err != nil {
				t.Fatal(err)
			}
			loader := func(context.Context, string) (string, error) { return aux, nil }
			_, err = AnalyzeMetadata(context.Background(), module, loader)
			if (err != nil) != tc.invalid {
				t.Fatalf("Go error %v invalid=%t", err, tc.invalid)
			}
			if jar != "" {
				dir := t.TempDir()
				for name, data := range map[string]string{"M.tla": source, "A.tla": aux} {
					if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				out, code := runNameResolutionJava(t, dir, "-cp", jar, "tla2sany.SANY", "M.tla")
				if code != 0 || strings.Contains(out, "*** Errors:") != tc.invalid || strings.Contains(out, "Parse Error") {
					t.Fatalf("Java exit %d invalid %t: %s", code, tc.invalid, out)
				}
			}
		})
	}
}

func instanceCollisionOracle(t *testing.T) string {
	t.Helper()
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		return ""
	}
	data, err := os.ReadFile(jar)
	if err != nil || fmt.Sprintf("%x", sha256.Sum256(data)) != "936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88" {
		t.Fatalf("TLC_JAR must be pinned Java TLC 1.7.4: %v", err)
	}
	jar, err = filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	return jar
}

func TestInstanceCollisionUnknownSourcePosition(t *testing.T) {
	source := "---- MODULE M ----\nI == INSTANCE A\nY == LET I(z) == INSTANCE B IN 1\n====\n"
	loader := func(_ context.Context, name string) (string, error) {
		switch name {
		case "A":
			return "---- MODULE A ----\nF == 1\n====\n", nil
		case "B":
			return "---- MODULE B ----\nG == 2\n====\n", nil
		default:
			return "", os.ErrNotExist
		}
	}
	module, err := parser.ParseModule(source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AnalyzeMetadata(context.Background(), module, loader); err == nil {
		t.Fatal("known conflicting arities were accepted")
	}
	for i := range module.Definitions {
		if module.Definitions[i].Name == "Y" {
			module.Definitions[i].Body.Instances[0].Location = parser.Location{}
		}
	}
	if _, err := AnalyzeMetadata(context.Background(), module, loader); err != nil {
		t.Fatalf("unknown position must not be guessed: %v", err)
	}
}
