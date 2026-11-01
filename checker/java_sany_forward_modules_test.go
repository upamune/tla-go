package checker

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
)

// Original cases compared with SANY 1.7.4 at upstream commit
// 5a47802b5c391f59ecdd44117981f4ff8c0656ba, not upstream fixtures.
func TestJavaSANYForwardReferencesAcrossModules(t *testing.T) {
	for _, tc := range []struct {
		name, root, auxiliary, assignments string
		invalid                            bool
	}{
		{"extension forward", "EXTENDS A", "Bad == Later\nLater == 1", "", true},
		{"extension recursive", "EXTENDS A", "RECURSIVE F(_), G(_)\nF(x) == G(x)\nG(x) == x", "", false},
		{"extension forward selector", "EXTENDS A", "Bad == Later!lab\nLater == lab :: 1", "", true},
		{"extension backward selector", "EXTENDS A", "Earlier == lab :: 1\nGood == Earlier!lab", "", false},
		{"extension constant forward", "EXTENDS A", "Bad == Later\nCONSTANT Later", "CONSTANT Later = 1\n", true},
		{"extension constant backward", "EXTENDS A", "CONSTANT C\nGood == C", "CONSTANT C = 1\n", false},
		{"root forward after extension", "EXTENDS A\nBad == Later\nLater == Base", "Base == 1", "", true},
		{"root backward after extension", "EXTENDS A\nGood == Base", "Base == 1", "", false},
		{"nested backward", "Earlier == 1\n---- MODULE Inner ----\nGood == Earlier\n====\nI == INSTANCE Inner", "", "", false},
		{"nested forward", "---- MODULE Inner ----\nBad == Later\n====\nLater == 1\nI == INSTANCE Inner", "", "", true},
		{"nested constant backward", "CONSTANT C\n---- MODULE Inner ----\nGood == C\n====\nI == INSTANCE Inner", "", "CONSTANT C = 1\n", false},
		{"nested constant forward", "---- MODULE Inner ----\nBad == C\n====\nCONSTANT C\nI == INSTANCE Inner", "", "CONSTANT C = 1\n", true},
		{"instance module forward", "I == INSTANCE A", "Bad == Later\nLater == 1", "", true},
		{"instance after extension", "EXTENDS A\nEarlier == 1\nI == INSTANCE B WITH C <- Earlier", "Base == 1", "", false},
		{"instance forward after extension", "EXTENDS A\nI == INSTANCE B WITH C <- Later\nLater == 1", "Base == 1", "", true},
		{"anonymous before instance", "Bad == F\nINSTANCE A", "F == 1", "", true},
		{"anonymous after instance", "INSTANCE A\nGood == F", "F == 1", "", false},
		{"local anonymous before instance", "Bad == F\nLOCAL INSTANCE A", "F == 1", "", true},
		{"local anonymous after instance", "LOCAL INSTANCE A\nGood == F", "F == 1", "", false},
		{"same line anonymous before instance", "Bad == F INSTANCE A", "F == 1", "", true},
		{"same line anonymous after instance", "INSTANCE A Good == F", "F == 1", "", false},
		{"anonymous selector before instance", "Bad == F!lab\nINSTANCE A", "F == lab :: 1", "", true},
		{"anonymous selector after instance", "INSTANCE A\nGood == F!lab", "F == lab :: 1", "", false},
		{"same line anonymous selector before", "Bad == F!lab INSTANCE A", "F == lab :: 1", "", true},
		{"same line anonymous selector after", "INSTANCE A Good == F!lab", "F == lab :: 1", "", false},
		{"theorem before anonymous instance", "THEOREM F = 1\nINSTANCE A", "F == 1", "", true},
		{"theorem after anonymous instance", "INSTANCE A\nTHEOREM F = 1", "F == 1", "", false},
		{"theorem selector before named instance", "THEOREM I!F!lab = 1\nI == INSTANCE A", "F == lab :: 1", "", true},
		{"theorem selector after named instance", "I == INSTANCE A\nTHEOREM I!F!lab = 1", "F == lab :: 1", "", false},
		{"second instance before", "I == INSTANCE B WITH C <- 1\nBad == F\nINSTANCE A", "F == 1", "", true},
		{"second instance after", "I == INSTANCE B WITH C <- 1\nINSTANCE A\nGood == F", "F == 1", "", false},
		{"two anonymous instances forward substitution", "INSTANCE B WITH C <- G\nINSTANCE A", "G == 1", "", true},
		{"two anonymous instances backward substitution", "INSTANCE A\nINSTANCE B WITH C <- G\nGood == F", "G == 1", "", false},
		{"extended anonymous instance", "EXTENDS A\nGood == F", "INSTANCE B WITH C <- 1", "", false},
		{"anonymous instance operator argument before", "Use(Op(_)) == Op(1)\nBad == Use(F)\nINSTANCE A", "F(x) == x", "", true},
		{"anonymous instance operator argument after", "INSTANCE A\nUse(Op(_)) == Op(1)\nGood == Use(F)", "F(x) == x", "", false},
		{"let shadows later anonymous import", "Good == LET F == 2 IN F\nINSTANCE A", "F == 1", "", false},
		{"bound shadows later anonymous import", "Good == \\A F \\in {1}: F = 1\nINSTANCE A", "F == 1", "", false},
		{"parameter shadows later anonymous import", "Good(F) == F\nINSTANCE A", "F == 1", "", false},
		{"implicit constant after anonymous instance", "INSTANCE A\nCONSTANT C", "CONSTANT C\nF == C", "CONSTANT C = 1\n", true},
		{"implicit constant before anonymous instance", "CONSTANT C\nINSTANCE A", "CONSTANT C\nF == C", "CONSTANT C = 1\n", false},
		{"unused implicit constant after instance", "INSTANCE A\nCONSTANT C", "CONSTANT C\nF == 1", "CONSTANT C = 1\n", true},
		{"implicit operator after anonymous instance", "Op(x) == x\nINSTANCE A\nCONSTANT C(_)", "CONSTANT C(_)\nF == C(1)", "CONSTANT C <- Op\n", true},
		{"implicit operator before anonymous instance", "Op(x) == x\nCONSTANT C(_)\nINSTANCE A", "CONSTANT C(_)\nF == C(1)", "CONSTANT C <- Op\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\n" + tc.root + "\nVARIABLE v\nInit == v = 0\nNext == UNCHANGED v\n====\n"
			auxiliary := "---- MODULE A ----\nEXTENDS Naturals\n" + tc.auxiliary + "\n====\n"
			instance := "---- MODULE B ----\nCONSTANT C\nF == C\n====\n"
			loader := func(_ context.Context, name string) (string, error) {
				switch name {
				case "A":
					return auxiliary, nil
				case "B":
					return instance, nil
				}
				return "", os.ErrNotExist
			}
			module, err := parser.ParseModule(source)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Parse("INIT Init\nNEXT Next\n" + tc.assignments)
			if err != nil {
				t.Fatal(err)
			}
			_, checkErr := CheckModuleWithLoader(context.Background(), module, cfg, loader)
			_, sourceErr := CheckWithLoader(context.Background(), source, "INIT Init\nNEXT Next\n"+tc.assignments, loader)
			_, metadataErr := AnalyzeMetadata(context.Background(), module, loader)
			for _, result := range []struct {
				name string
				err  error
			}{{"model check", checkErr}, {"source model check", sourceErr}, {"metadata", metadataErr}} {
				if (result.err != nil) != tc.invalid {
					t.Fatalf("%s error=%v, invalid=%t", result.name, result.err, tc.invalid)
				}
				if tc.invalid {
					if _, ok := errors.AsType[*SemanticError](result.err); !ok {
						t.Fatalf("%s returned non-semantic error: %v", result.name, result.err)
					}
				}
			}
			compareForwardJava(t, source, auxiliary, instance, tc.invalid)
		})
	}
}

func TestForwardDeclarationWithoutSourceProvenance(t *testing.T) {
	for _, extension := range []string{"", "EXTENDS A\n"} {
		t.Run(extension, func(t *testing.T) {
			source := "---- MODULE M ----\n" + extension + "CONSTANT C\nF == C\nVARIABLE v\nInit == v = 0\nNext == UNCHANGED v\n====\n"
			module, err := parser.ParseModule(source)
			if err != nil {
				t.Fatal(err)
			}
			// Simulate a direct API AST with a position but no source origin.
			module.SourceTokens = nil
			module.Constants[0].Location = parser.Location{Line: 20, Column: 10}
			loader := func(context.Context, string) (string, error) { return "---- MODULE A ----\nA == 1\n====\n", nil }
			cfg, err := config.Parse("CONSTANT C = 1\nINIT Init\nNEXT Next\n")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := AnalyzeMetadata(context.Background(), module, loader); err != nil {
				t.Fatalf("metadata guessed unknown provenance: %v", err)
			}
			if _, err := CheckModuleWithLoader(context.Background(), module, cfg, loader); err != nil {
				t.Fatalf("model checker guessed unknown provenance: %v", err)
			}
		})
	}
}

func TestInstanceReferenceWithoutSourceProvenance(t *testing.T) {
	for _, tc := range []struct {
		name, source string
	}{
		{"anonymous instance", "Bad == F\nINSTANCE A"},
		{"named instance in theorem", "THEOREM I!F = 1\nI == INSTANCE A"},
		{"anonymous selector", "Bad == F!lab\nINSTANCE A"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			module, err := parser.ParseModule("---- MODULE M ----\n" + tc.source + "\n====\n")
			if err != nil {
				t.Fatal(err)
			}
			// A direct API caller may supply an AST without instance coordinates.
			module.Instances[0].Location = parser.Location{}
			loader := func(context.Context, string) (string, error) {
				return "---- MODULE A ----\nF == lab :: 1\n====\n", nil
			}
			if _, err := AnalyzeMetadata(context.Background(), module, loader); err != nil {
				t.Fatalf("unknown instance location must not be guessed: %v", err)
			}
		})
	}
}

func TestAnonymousInstanceReferenceJSONRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		invalid      bool
	}{
		{"forward import", "Bad == F\nINSTANCE A", true},
		{"backward import", "INSTANCE A\nGood == F", false},
		{"forward selector", "Bad == F!lab\nINSTANCE A", true},
		{"backward selector", "INSTANCE A\nGood == F!lab", false},
		{"forward theorem", "THEOREM I!F = 1\nI == INSTANCE A", true},
		{"backward theorem", "I == INSTANCE A\nTHEOREM I!F = 1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			module, err := parser.ParseModule("---- MODULE M ----\n" + tc.source + "\n====\n")
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(module)
			if err != nil {
				t.Fatal(err)
			}
			var restored parser.Module
			if err := json.Unmarshal(data, &restored); err != nil {
				t.Fatal(err)
			}
			loader := func(context.Context, string) (string, error) {
				return "---- MODULE A ----\nF == lab :: 1\n====\n", nil
			}
			_, err = AnalyzeMetadata(context.Background(), &restored, loader)
			if (err != nil) != tc.invalid {
				t.Fatalf("round-trip error %v, invalid=%t", err, tc.invalid)
			}
		})
	}
}

func TestJavaSANYForwardImplicitVariableOrder(t *testing.T) {
	const auxiliary = "---- MODULE A ----\nVARIABLE v\nF == v\n====\n"
	for _, tc := range []struct {
		name, declarations string
		invalid            bool
	}{
		{"forward", "INSTANCE A\nVARIABLE v", true},
		{"backward", "VARIABLE v\nINSTANCE A", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\n" + tc.declarations + "\nInit == v = 0\nNext == UNCHANGED v\n====\n"
			loader := func(context.Context, string) (string, error) { return auxiliary, nil }
			result, err := CheckWithLoader(context.Background(), source, "INIT Init\nNEXT Next\n", loader)
			if (err != nil) != tc.invalid || !tc.invalid && result.Status != Success {
				t.Fatalf("result %+v, error %v, invalid %t", result, err, tc.invalid)
			}
			if tc.invalid {
				if _, ok := errors.AsType[*SemanticError](err); !ok {
					t.Fatalf("expected semantic error: %v", err)
				}
			}
			compareForwardJava(t, source, auxiliary, "---- MODULE B ----\n====\n", tc.invalid)
		})
	}
}

func TestForwardDeclarationWithUnknownColumn(t *testing.T) {
	for _, tc := range []struct {
		name, definitions string
		mutate            func(*parser.Expression)
	}{
		{"global operator", "C == 1 Good == C", clearExpressionColumn},
		{"global constant", "CONSTANT C Good == C", clearExpressionColumn},
		{"local operator", "Good == LET X == 1 Y == X IN Y", func(expr *parser.Expression) {
			expr.Definitions[1].Location.Column = 0
			expr.Definitions[1].Location.Range.Start.Column = 0
		}},
		{"selector", "C == lab :: 1 Good == C!lab", clearExpressionColumn},
		{"instance", "I == INSTANCE A Good == I!F", clearExpressionColumn},
	} {
		t.Run(tc.name, func(t *testing.T) {
			module, err := parser.ParseModule("---- MODULE M ----\n" + tc.definitions + "\n====\n")
			if err != nil {
				t.Fatal(err)
			}
			for _, definition := range module.Definitions {
				if definition.Name == "Good" {
					tc.mutate(definition.Body)
				}
			}
			loader := func(context.Context, string) (string, error) { return "---- MODULE A ----\nF == 1\n====\n", nil }
			if _, err := AnalyzeMetadata(context.Background(), module, loader); err != nil {
				t.Fatalf("guessed order from missing column: %v", err)
			}
		})
	}
}

func clearExpressionColumn(expr *parser.Expression) {
	expr.Location.Column = 0
	expr.Location.Range.Start.Column = 0
}

func TestForwardInstanceWithUnknownDeclarationColumn(t *testing.T) {
	module, err := parser.ParseModule("---- MODULE M ----\nI == INSTANCE A Good == I!F\n====\n")
	if err != nil {
		t.Fatal(err)
	}
	module.Instances[0].Location.Column = 0
	module.Instances[0].Location.Range.Start.Column = 0
	loader := func(context.Context, string) (string, error) { return "---- MODULE A ----\nF == 1\n====\n", nil }
	if _, err := AnalyzeMetadata(context.Background(), module, loader); err != nil {
		t.Fatalf("guessed order from missing declaration column: %v", err)
	}
}

func TestInstanceOrderBeforeLocalShadow(t *testing.T) {
	module, err := parser.ParseModule("---- MODULE M ----\nI == INSTANCE A\nGood == LET X == I!F(1) I == INSTANCE A IN X\n====\n")
	if err != nil {
		t.Fatal(err)
	}
	resolver := instanceResolver{root: module}
	if err := resolver.validateReferenceOrder(module.Instances[0]); err != nil {
		t.Fatalf("rejected reference to outer instance before local shadow: %v", err)
	}
}

func compareForwardJava(t *testing.T, source, auxiliary, instance string, invalid bool) {
	t.Helper()
	jar := os.Getenv("TLC_JAR")
	if jar == "" {
		return
	}
	data, err := os.ReadFile(jar)
	if err != nil || fmt.Sprintf("%x", sha256.Sum256(data)) != "936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88" {
		t.Fatalf("TLC_JAR must be the pinned 1.7.4 jar: %v", err)
	}
	jar, err = filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, contents := range map[string]string{"M.tla": source, "A.tla": auxiliary, "B.tla": instance} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	out, code := runNameResolutionJava(t, dir, "-cp", jar, "tla2sany.SANY", "M.tla")
	if code != 0 || strings.Contains(out, "Parse Error") || strings.Contains(out, "Semantic errors:") != invalid {
		t.Fatalf("SANY exit %d, invalid=%t:\n%s", code, invalid, out)
	}
}
