package checker

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
)

// Original cases checked against Java SANY 1.7.4 at upstream commit
// 5a47802b5c391f59ecdd44117981f4ff8c0656ba. Set TLC_JAR to run the oracle.
func TestJavaSANYRecursiveDeclarations(t *testing.T) {
	cases := []struct {
		name, body string
		invalid    bool
	}{
		{"missing", "RECURSIVE F(_)", true},
		{"wrong arity", "RECURSIVE F(_,_)\nF(x) == x", true},
		{"wrong nullary", "RECURSIVE F\nF(x) == x", true},
		{"duplicate", "RECURSIVE F(_), F(_)\nF(x) == x", true},
		{"repeated declaration", "RECURSIVE F(_)\nRECURSIVE F(_)\nF(x) == x", true},
		{"declaration after definition", "F(x) == x\nRECURSIVE F(_)", true},
		{"let missing", "X == LET RECURSIVE F(_) IN 1", true},
		{"let wrong arity", "X == LET RECURSIVE F(_,_) F(x) == x IN 1", true},
		{"let nested missing", "X == LET RECURSIVE F(_) G == LET F(x) == x IN 1 IN G", true},
		{"proof define missing", "THEOREM TRUE\n<1>1. DEFINE X == LET RECURSIVE F(_) IN 1\n<1> QED OBVIOUS", true},
		{"proof define valid", "THEOREM TRUE\n<1>1. DEFINE X == LET RECURSIVE F(_) F(x) == x IN F(1)\n<1> QED OBVIOUS", false},
		{"proof nested statement missing", "THEOREM TRUE\n<1>1. TRUE\n<2>1. LET RECURSIVE F(_) IN TRUE\n<2> QED OBVIOUS\n<1> QED OBVIOUS", true},
		{"proof nested statement valid", "THEOREM TRUE\n<1>1. TRUE\n<2>1. LET RECURSIVE F(_) F(x) == x IN TRUE\n<2> QED OBVIOUS\n<1> QED OBVIOUS", false},
		{"proof bound missing", "THEOREM TRUE\n<1>1. TAKE x \\in (LET RECURSIVE F(_) IN {1})\n<1> QED OBVIOUS", true},
		{"proof bound valid", "THEOREM TRUE\n<1>1. TAKE x \\in (LET RECURSIVE F(_) F(x) == x IN {1})\n<1> QED OBVIOUS", false},
		{"proof witness missing", "THEOREM TRUE\n<1>1. WITNESS LET RECURSIVE F(_) IN 1\n<1> QED OBVIOUS", true},
		{"proof witness valid", "THEOREM TRUE\n<1>1. WITNESS LET RECURSIVE F(_) F(x) == x IN 1\n<1> QED OBVIOUS", false},
		{"proof by missing", "THEOREM TRUE BY LET RECURSIVE F(_) IN TRUE", true},
		{"proof by valid", "THEOREM TRUE BY LET RECURSIVE F(_) F(x) == x IN TRUE", false},
		{"proof directive missing", "USE LET RECURSIVE F(_) IN TRUE", true},
		{"proof directive valid", "USE LET RECURSIVE F(_) F(x) == x IN TRUE", false},
		{"proof step directive missing", "THEOREM TRUE\n<1>1. USE LET RECURSIVE F(_) IN TRUE\n<1> QED OBVIOUS", true},
		{"proof step directive valid", "THEOREM TRUE\n<1>1. USE LET RECURSIVE F(_) F(x) == x IN TRUE\n<1> QED OBVIOUS", false},
		{"function wrong arity", "RECURSIVE F(_)\nF[x \\in {1}] == x", true},
		{"function zero arity", "RECURSIVE F\nF[x \\in {1}] == x", false},
		{"operator parameter first", "RECURSIVE F(_,_)\nF(G(_),x) == x", false},
		{"nullary", "RECURSIVE F\nF == 1", false},
		{"operator", "RECURSIVE F(_)\nF(x) == IF x = 0 THEN 0 ELSE F(x-1)", false},
		{"local definition", "RECURSIVE F(_)\nLOCAL F(x) == x", false},
		{"mutual", "RECURSIVE F(_), G(_)\nF(x) == IF x = 0 THEN 0 ELSE G(x-1)\nG(x) == IF x = 0 THEN 0 ELSE F(x-1)", false},
		{"interleaved", "RECURSIVE F(_), G(_)\nF(x) == x\nH == 1\nG(x) == x", false},
		{"mutual let", "X == LET RECURSIVE F(_), G(_) F(x) == G(x) G(x) == x IN F(1)", false},
		{"instance cannot define", "RECURSIVE F(_)\nINSTANCE A", true},
		{"instance of recursive module", "I == INSTANCE A\nX == I!F(1)", false},
	}
	jar := os.Getenv("TLC_JAR")
	if jar != "" {
		data, err := os.ReadFile(jar)
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != "936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88" {
			t.Fatalf("unexpected jar digest: %s", got)
		}
		jar, err = filepath.Abs(jar)
		if err != nil {
			t.Fatal(err)
		}
	}
	const auxiliary = "---- MODULE A ----\nRECURSIVE F(_)\nF(x) == x\n====\n"
	const cfg = "INIT Init\nNEXT Next\n"
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS Naturals\n" + tc.body + "\nVARIABLE v\nInit == v = 0\nNext == UNCHANGED v\n====\n"
			loader := func(_ context.Context, name string) (string, error) {
				if name == "A" {
					return auxiliary, nil
				}
				return "", os.ErrNotExist
			}
			_, err := CheckWithLoader(context.Background(), source, cfg, loader)
			if (err != nil) != tc.invalid {
				t.Fatalf("Go error %v, invalid %t", err, tc.invalid)
			}
			m, err := parser.ParseModule(source)
			if err != nil {
				t.Fatal(err)
			}
			c, err := config.Parse(cfg)
			if err != nil {
				t.Fatal(err)
			}
			_, err = CheckModuleWithLoader(context.Background(), m, c, loader)
			if (err != nil) != tc.invalid {
				t.Fatalf("direct API error %v, invalid %t", err, tc.invalid)
			}
			if jar == "" {
				return
			}
			dir := t.TempDir()
			for name, text := range map[string]string{"M.tla": source, "A.tla": auxiliary} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			out, code := runNameResolutionJava(t, dir, "-cp", jar, "tla2sany.SANY", "M.tla")
			if code != 0 || strings.Contains(out, "Parse Error") || strings.Contains(out, "Semantic errors:") != tc.invalid {
				t.Fatalf("SANY exit %d invalid %t:\n%s", code, tc.invalid, out)
			}
		})
	}
}

func TestRecursiveDeclarationsInImportedModules(t *testing.T) {
	for _, tc := range []struct {
		name, root, imported string
		invalid              bool
	}{
		{"extension invalid", "EXTENDS A", "RECURSIVE F(_)\nF(x,y) == x", true},
		{"instance invalid", "I == INSTANCE A", "RECURSIVE F(_)", true},
		{"extension valid", "EXTENDS A", "RECURSIVE F(_)\nF(x) == x", false},
		{"instance valid", "I == INSTANCE A", "RECURSIVE F(_)\nF(x) == x", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := "---- MODULE M ----\n" + tc.root + "\nVARIABLE v\nInit == v = 0\nNext == UNCHANGED v\n====\n"
			aux := "---- MODULE A ----\n" + tc.imported + "\n====\n"
			loader := func(_ context.Context, name string) (string, error) {
				if name == "A" {
					return aux, nil
				}
				return "", os.ErrNotExist
			}
			_, err := CheckWithLoader(context.Background(), root, "INIT Init\nNEXT Next\n", loader)
			if (err != nil) != tc.invalid {
				t.Fatalf("Go error %v, invalid %t", err, tc.invalid)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				jar, err = filepath.Abs(jar)
				if err != nil {
					t.Fatal(err)
				}
				dir := t.TempDir()
				for name, text := range map[string]string{"M.tla": root, "A.tla": aux} {
					if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				out, code := runNameResolutionJava(t, dir, "-cp", jar, "tla2sany.SANY", "M.tla")
				if code != 0 || strings.Contains(out, "Semantic errors:") != tc.invalid || strings.Contains(out, "Parse Error") {
					t.Fatalf("SANY exit %d:\n%s", code, out)
				}
			}
		})
	}
}

// These are documented gaps, not assertions that Go should accept them.
func TestJavaSANYRecursiveDeclarationKnownGaps(t *testing.T) {
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		t.Skip("set TLC_JAR to compare with SANY 1.7.4")
	}
	data, err := os.ReadFile(jar)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != "936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88" {
		t.Fatalf("unexpected jar digest: %s", got)
	}
	jar, err = filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, body, diagnostic string }{
		{"operator parameter last", "RECURSIVE F(_)\nF(G(_)) == 1", "different arity"},
		{"constant in recursive section", "RECURSIVE F(_)\nCONSTANT C\nF(x) == x", "recursive definition section"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nVARIABLE v\n" + tc.body + "\nInit == v = 0\nNext == UNCHANGED v\n====\n"
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "M.tla"), []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			out, code := runNameResolutionJava(t, dir, "-cp", jar, "tla2sany.SANY", "M.tla")
			if code != 0 || !strings.Contains(out, "Semantic errors:") || !strings.Contains(out, tc.diagnostic) {
				t.Fatalf("SANY exit %d:\n%s", code, out)
			}
			cfg := "INIT Init\nNEXT Next\n"
			if tc.name == "constant in recursive section" {
				cfg += "CONSTANT C = 1\n"
			}
			_, err := Check(source, cfg)
			if err == nil {
				t.Log("known gap: Go accepted a SANY semantic error")
			}
		})
	}
}
