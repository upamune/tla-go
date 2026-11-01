package checker

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
)

// SANY 1.7.4 at the upstream commit in README is the oracle for these cases.
func TestRecursiveInstanceSubstitutions(t *testing.T) {
	const auxiliary = "---- MODULE A ----\nCONSTANT C\nK == C\n====\n"
	loader := func(_ context.Context, name string) (string, error) {
		if name == "A" {
			return auxiliary, nil
		}
		return "", os.ErrNotExist
	}
	for _, tc := range []struct {
		name, body string
		invalid    bool
	}{
		{"module missing", "I == INSTANCE A WITH C <- LET RECURSIVE F(_) IN 1", true},
		{"module arity", "I == INSTANCE A WITH C <- LET RECURSIVE F(_,_) F(x) == x IN 1", true},
		{"module valid", "I == INSTANCE A WITH C <- LET RECURSIVE F(_) F(x) == x IN F(1)", false},
		{"let missing", "X == LET I == INSTANCE A WITH C <- LET RECURSIVE F(_) IN 1 IN I!K", true},
		{"let duplicate", "X == LET I == INSTANCE A WITH C <- LET RECURSIVE F(_), F(_) F(x) == x IN 1 IN I!K", true},
		{"let valid", "X == LET I == INSTANCE A WITH C <- LET RECURSIVE F(_) F(x) == x IN F(1) IN I!K", false},
		{"selector argument missing", "F(x) == <<x, x>>\nI == INSTANCE A WITH C <- F(LET RECURSIVE G(_) IN 1)!1", true},
		{"selector argument valid", "F(x) == <<x, x>>\nI == INSTANCE A WITH C <- F(LET RECURSIVE G(_) G(x) == x IN G(1))!1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\n" + tc.body + "\nVARIABLE v\nInit == v = 0\nNext == UNCHANGED v\n====\n"
			_, err := CheckWithLoader(context.Background(), source, "INIT Init\nNEXT Next\n", loader)
			if (err != nil) != tc.invalid {
				t.Fatalf("Go error %v, invalid %t", err, tc.invalid)
			}
			if tc.invalid && !strings.Contains(err.Error(), "RECURSIVE") {
				t.Fatalf("unexpected error: %v", err)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				jar, err = filepath.Abs(jar)
				if err != nil {
					t.Fatal(err)
				}
				dir := t.TempDir()
				for name, contents := range map[string]string{"M.tla": source, "A.tla": auxiliary} {
					if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				out, code := runNameResolutionJava(t, dir, "-cp", jar, "tla2sany.SANY", "M.tla")
				if code != 0 || strings.Contains(out, "Parse Error") || strings.Contains(out, "Semantic errors:") != tc.invalid {
					t.Fatalf("SANY exit %d, invalid %t:\n%s", code, tc.invalid, out)
				}
			}
		})
	}
}

func TestRecursiveLetJSONProjectionLimitation(t *testing.T) {
	for _, body := range []string{
		"RECURSIVE F(_) IN 1",
		"RECURSIVE F(_,_) F(x) == x IN 1",
		"RECURSIVE F(_), F(_) F(x) == x IN 1",
	} {
		t.Run(body, func(t *testing.T) {
			source := "---- MODULE M ----\nX == LET " + body + "\nVARIABLE v\nInit == v = 0\nNext == UNCHANGED v\n===="
			module, err := parser.ParseModule(source)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Parse("INIT Init\nNEXT Next\n")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := CheckModuleWithLoader(context.Background(), module, cfg, nil); err == nil || !strings.Contains(err.Error(), "RECURSIVE") {
				t.Fatalf("direct module error: %v", err)
			}
			data, err := json.Marshal(module)
			if err != nil {
				t.Fatal(err)
			}
			var projected parser.Module
			if err := json.Unmarshal(data, &projected); err != nil {
				t.Fatal(err)
			}
			if _, err := CheckModuleWithLoader(context.Background(), &projected, cfg, nil); err != nil {
				t.Fatalf("projected module error: %v", err)
			}
		})
	}
}

func TestRecursiveValidationExpressionChildren(t *testing.T) {
	invalid := func() *parser.Expression {
		return &parser.Expression{RecursiveDeclarations: []parser.Declaration{{Name: "Missing"}}}
	}
	for _, tc := range []struct {
		name string
		expr *parser.Expression
	}{
		{"argument", &parser.Expression{Args: []*parser.Expression{invalid()}}},
		{"selector argument", &parser.Expression{Selectors: []parser.Selector{{Args: []*parser.Expression{invalid()}}}}},
		{"bound", &parser.Expression{Bounds: []parser.Bound{{Set: invalid()}}}},
		{"field", &parser.Expression{Fields: []parser.Field{{Value: invalid()}}}},
		{"case condition", &parser.Expression{Arms: []parser.CaseArm{{Condition: invalid()}}}},
		{"case body", &parser.Expression{Arms: []parser.CaseArm{{Body: invalid()}}}},
		{"update value", &parser.Expression{Updates: []parser.Update{{Value: invalid()}}}},
		{"update path argument", &parser.Expression{Updates: []parser.Update{{Path: []parser.PathStep{{Args: []*parser.Expression{invalid()}}}}}}},
		{"definition body", &parser.Expression{Definitions: []parser.Definition{{Body: invalid()}}}},
		{"definition bound", &parser.Expression{Definitions: []parser.Definition{{Bounds: []parser.Bound{{Set: invalid()}}}}}},
		{"instance substitution", &parser.Expression{Instances: []parser.Instance{{Substitutions: []parser.Substitution{{Expression: invalid()}}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateRecursiveExpression(tc.expr); err == nil || !strings.Contains(err.Error(), "Missing") {
				t.Fatalf("expected missing declaration error, got %v", err)
			}
		})
	}
}
