package checker

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
)

// These cases use the pinned Java TLC 1.7.4 SANY oracle in testInlineNestedJava.
func TestJavaSANYNestedInstanceScope(t *testing.T) {
	for _, tc := range []struct {
		name, body, constants string
		invalid               bool
	}{
		{"outer named instance", "---- MODULE N ---- A == 1 ==== J == INSTANCE N ---- MODULE R ---- V == J!A ==== I == INSTANCE R", "", false},
		{"outer named instance multiline", "---- MODULE N ----\nA == 1\n====\nJ == INSTANCE N\n---- MODULE R ----\nV == J!A\n====\nI == INSTANCE R", "", false},
		{"outer local instance", "---- MODULE N ---- A == 1 ==== LOCAL J == INSTANCE N ---- MODULE R ---- V == J!A ==== I == INSTANCE R", "", false},
		{"outer instance WITH", "CONSTANT C ---- MODULE N ---- CONSTANT K A == K ==== J == INSTANCE N WITH K <- C ---- MODULE R ---- V == J!A ==== I == INSTANCE R", "CONSTANT C = 1\n", false},
		{"parameterized outer instance", "---- MODULE N ---- CONSTANT K A == K ==== J(k) == INSTANCE N WITH K <- k ---- MODULE R ---- V == J(1)!A ==== I == INSTANCE R", "", false},
		{"twice nested", "---- MODULE N ---- A == 1 ==== J == INSTANCE N ---- MODULE R ---- ---- MODULE L ---- V == J!A ==== K == INSTANCE L V == K!V ==== I == INSTANCE R", "", false},
		{"inner named instance is exported", "---- MODULE N ---- A == 1 ==== ---- MODULE R ---- J == INSTANCE N V == J!A ==== I == INSTANCE R Leak == I!J!A", "", false},
		{"different outer instance is exported", "---- MODULE N ---- A == 1 ==== J == INSTANCE N ---- MODULE R ---- K == INSTANCE N V == K!A ==== I == INSTANCE R Leak == I!K!A", "", false},
		{"collision uses first binding", "---- MODULE N ---- A == 1 ==== ---- MODULE O ---- A == 2 ==== J == INSTANCE N ---- MODULE R ---- J == INSTANCE O V == J!A ==== I == INSTANCE R ASSUME I!V = 1", "", false},
		{"colliding named instance is private", "---- MODULE N ---- A == 1 ==== J == INSTANCE N ---- MODULE R ---- J == INSTANCE N V == J!A ==== I == INSTANCE R Leak == I!J!A", "", true},
		{"outer instance after nested module", "---- MODULE N ---- A == 1 ==== ---- MODULE R ---- V == J!A ==== J == INSTANCE N I == INSTANCE R", "", true},
		{"inner local instance is private", "---- MODULE N ---- A == 1 ==== ---- MODULE R ---- LOCAL J == INSTANCE N V == J!A ==== I == INSTANCE R Leak == I!J!A", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\n" + tc.body + "\nVARIABLE x\nInit == x = I!V\nNext == UNCHANGED x\n====\n"
			cfgText := "INIT Init\nNEXT Next\n" + tc.constants
			parsed, err := parser.ParseModule(source)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Parse(cfgText)
			if err != nil {
				t.Fatal(err)
			}
			result, err := CheckModuleWithLoader(context.Background(), parsed, cfg, nil)
			if (err != nil) != tc.invalid || !tc.invalid && (result.Status != Success || result.DistinctStates != 1) {
				t.Fatalf("check result %+v error %v, invalid=%t", result, err, tc.invalid)
			}
			metadata, err := AnalyzeMetadata(context.Background(), parsed, nil)
			if (err != nil) != tc.invalid {
				t.Fatalf("metadata error %v, invalid=%t", err, tc.invalid)
			}
			if !tc.invalid {
				checkInlineNestedMetadata(t, metadata, "I!V", 0)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				testInlineNestedJava(t, jar, source, cfgText, tc.invalid, false)
			}
		})
	}
}

func TestInheritedNestedModuleSeesEnclosingInstance(t *testing.T) {
	const outer = "---- MODULE A ----\n---- MODULE N ---- A == 1 ====\nJ == INSTANCE N\n---- MODULE R ---- V == J!A ====\n====\n"
	const source = "---- MODULE M ----\nEXTENDS A\nI == INSTANCE R\nVARIABLE x\nInit == x = I!V\nNext == UNCHANGED x\n====\n"
	loader := func(_ context.Context, name string) (string, error) {
		if name == "A" {
			return outer, nil
		}
		return "", os.ErrNotExist
	}
	result, err := CheckWithLoader(context.Background(), source, "INIT Init\nNEXT Next\n", loader)
	if err != nil || result.Status != Success || result.DistinctStates != 1 {
		t.Fatalf("result %+v, error %v", result, err)
	}
	parsed, err := parser.ParseModule(source)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := AnalyzeMetadata(context.Background(), parsed, loader)
	if err != nil {
		t.Fatal(err)
	}
	checkInlineNestedMetadata(t, metadata, "I!V", 0)
	compareForwardJava(t, source, outer, "---- MODULE B ----\n====\n", false)
	for _, definition := range metadata.Definitions {
		if strings.HasPrefix(definition.Name, "!instance!") {
			t.Fatalf("metadata exposes internal instance name %q", definition.Name)
		}
	}
}

func TestNestedModuleSeesEnclosingInstanceWithStateVariable(t *testing.T) {
	const source = "---- MODULE M ----\nVARIABLE x\n---- MODULE N ---- VARIABLE y A == y ====\nJ == INSTANCE N WITH y <- x\n---- MODULE R ---- V == J!A ====\nI == INSTANCE R\nInit == x = 1 /\\ I!V = 1\nNext == UNCHANGED x\n====\n"
	const cfgText = "INIT Init\nNEXT Next\n"
	result, err := CheckWithLoader(context.Background(), source, cfgText, nil)
	if err != nil || result.Status != Success || result.DistinctStates != 1 {
		t.Fatalf("result %+v, error %v", result, err)
	}
	parsed, err := parser.ParseModule(source)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := AnalyzeMetadata(context.Background(), parsed, nil)
	if err != nil {
		t.Fatal(err)
	}
	checkInlineNestedMetadata(t, metadata, "I!V", 1)
	if jar := os.Getenv("TLC_JAR"); jar != "" {
		testInlineNestedJava(t, jar, source, cfgText, false, false)
	}
}

func TestNestedInstanceWithLineOnlySourcePositions(t *testing.T) {
	const source = "---- MODULE M ----\n---- MODULE N ---- A == 1 ====\nJ == INSTANCE N\n---- MODULE R ---- V == J!A ====\nI == INSTANCE R\nVARIABLE x\nInit == x = I!V\nNext == UNCHANGED x\n====\n"
	module, err := parser.ParseModule(source)
	if err != nil {
		t.Fatal(err)
	}
	clearNestedInstanceColumns(module)
	cfg, err := config.Parse("INIT Init\nNEXT Next\n")
	if err != nil {
		t.Fatal(err)
	}
	result, err := CheckModuleWithLoader(context.Background(), module, cfg, nil)
	if err != nil || result.Status != Success || result.DistinctStates != 1 {
		t.Fatalf("result %+v, error %v", result, err)
	}
	metadata, err := AnalyzeMetadata(context.Background(), module, nil)
	if err != nil {
		t.Fatal(err)
	}
	checkInlineNestedMetadata(t, metadata, "I!V", 0)
}

func clearNestedInstanceColumns(module *parser.Module) {
	for i := range module.Instances {
		module.Instances[i].Location.Position.Column = 0
	}
	for i := range module.Modules {
		module.Modules[i].Location.Range.Start.Column = 0
		clearNestedInstanceColumns(&module.Modules[i])
	}
}
