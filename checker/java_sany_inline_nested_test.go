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

// Original cases checked against SANY and TLC 1.7.4 at upstream commit
// 5a47802b5c391f59ecdd44117981f4ff8c0656ba.
func TestJavaSANYInlineNestedModules(t *testing.T) {
	const cfg = "INIT Init\nNEXT Next\n"
	for _, tc := range []struct {
		name, body, constants string
		invalid, missing      bool
	}{
		{"enclosing constant", "CONSTANT C ---- MODULE Inner ----\nValue == C\n====\nI == INSTANCE Inner\nVARIABLE x\nInit == x = I!Value", "CONSTANT C = 1\n", false, false},
		{"enclosing variable", "VARIABLE x ---- MODULE Inner ----\nValue == x\n====\nI == INSTANCE Inner\nInit == x = 0 /\\ I!Value = 0", "", false, false},
		{"enclosing definition", "C == 1 ---- MODULE Inner ----\nValue == C\n====\nI == INSTANCE Inner\nVARIABLE x\nInit == x = I!Value", "", false, false},
		{"inline nested and instance", "---- MODULE Inner ---- Value == 1 ==== I == INSTANCE Inner\nVARIABLE x\nInit == x = I!Value", "", false, false},
		{"inline nested extension", "---- MODULE One ---- F == 1 ==== ---- MODULE Two ---- EXTENDS One G == F ==== I == INSTANCE Two\nVARIABLE x\nInit == x = I!G", "", false, false},
		{"inline constant and instance", "CONSTANT C ---- MODULE Inner ---- Value == C ==== I == INSTANCE Inner\nVARIABLE x\nInit == x = I!Value", "CONSTANT C = 1\n", false, false},
		{"later enclosing constant", "---- MODULE Inner ----\nValue == C\n==== CONSTANT C\nI == INSTANCE Inner\nVARIABLE x\nInit == x = I!Value", "CONSTANT C = 1\n", true, false},
		{"later enclosing definition", "---- MODULE Inner ---- Value == C ==== C == 1 I == INSTANCE Inner\nVARIABLE x\nInit == x = I!Value", "", true, false},
		{"instance before nested", "I == INSTANCE Inner ---- MODULE Inner ---- Value == 1 ====\nVARIABLE x\nInit == x = I!Value", "", true, true},
		{"extension before sibling", "---- MODULE Two ---- EXTENDS One G == F ==== ---- MODULE One ---- F == 1 ==== I == INSTANCE Two\nVARIABLE x\nInit == x = I!G", "", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\n" + tc.body + "\nNext == UNCHANGED x\n====\n"
			configText := cfg + tc.constants
			parsed, err := parser.ParseModule(source)
			if err != nil {
				t.Fatal(err)
			}
			parsedCfg, err := config.Parse(configText)
			if err != nil {
				t.Fatal(err)
			}
			result, checkErr := CheckModuleWithLoader(context.Background(), parsed, parsedCfg, nil)
			if tc.invalid && checkErr == nil || !tc.invalid && (checkErr != nil || result.Status != Success || result.DistinctStates != 1) {
				t.Fatalf("Go result %+v error %v, invalid=%t", result, checkErr, tc.invalid)
			}
			direct, directErr := Check(source, configText)
			if (directErr != nil) != tc.invalid || !tc.invalid && (direct.Status != Success || direct.DistinctStates != 1) {
				t.Fatalf("source result %+v error %v, invalid=%t", direct, directErr, tc.invalid)
			}
			metadata, err := AnalyzeMetadata(context.Background(), parsed, nil)
			if (err != nil) != tc.invalid {
				t.Fatalf("metadata error %v, invalid=%t", err, tc.invalid)
			}
			if !tc.invalid {
				wantName, wantLevel := "I!Value", 0
				if tc.name == "enclosing variable" {
					wantLevel = 1
				}
				if tc.name == "inline nested extension" {
					wantName = "I!G"
				}
				checkInlineNestedMetadata(t, metadata, wantName, wantLevel)
			}
			if jar := os.Getenv("TLC_JAR"); jar != "" {
				testInlineNestedJava(t, jar, source, configText, tc.invalid, tc.missing)
			}
		})
	}
}

func checkInlineNestedMetadata(t *testing.T, metadata SemanticMetadata, name string, level int) {
	t.Helper()
	if len(metadata.Variables) != 1 || metadata.Variables[0] != "x" {
		t.Fatalf("metadata variables: %v", metadata.Variables)
	}
	for _, def := range metadata.Definitions {
		if def.Name == name {
			if def.Level != level {
				t.Fatalf("metadata %s level = %d; want %d", name, def.Level, level)
			}
			return
		}
	}
	t.Fatalf("definition %s missing from metadata: %+v", name, metadata.Definitions)
}

func testInlineNestedJava(t *testing.T, jar, source, cfg string, invalid, missing bool) {
	t.Helper()
	data, err := os.ReadFile(jar)
	if err != nil {
		t.Fatal(err)
	}
	if hash := fmt.Sprintf("%x", sha256.Sum256(data)); hash != "936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88" {
		t.Fatalf("TLC_JAR is not pinned 1.7.4: %s", hash)
	}
	jar, err = filepath.Abs(jar)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, contents := range map[string]string{"M.tla": source, "M.cfg": cfg} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	sany, sanyCode := runNameResolutionJava(t, dir, "-cp", jar, "tla2sany.SANY", "M.tla")
	if missing && (sanyCode == 0 || !strings.Contains(sany, "Cannot find source file")) {
		t.Fatalf("SANY expected missing module error, exit %d:\n%s", sanyCode, sany)
	}
	if !missing && (sanyCode != 0 || strings.Contains(sany, "Semantic errors:") != invalid || strings.Contains(sany, "Parse Error")) {
		t.Fatalf("SANY exit %d, invalid %t:\n%s", sanyCode, invalid, sany)
	}
	tlc, tlcCode := runNameResolutionJava(t, dir, "-jar", jar, "-tool", "-deadlock", "-workers", "1", "-fp", "0", "M.tla")
	want := 0
	if invalid {
		want = 150
	}
	if tlcCode != want {
		t.Fatalf("TLC exit %d, want %d:\n%s", tlcCode, want, tlc)
	}
}

func TestNestedPositionBeforeMissingCoordinates(t *testing.T) {
	for _, tc := range []struct {
		name            string
		position, start parser.Position
		want            bool
	}{
		{"earlier column", parser.Position{Line: 2, Column: 3}, parser.Position{Line: 2, Column: 8}, true},
		{"same column", parser.Position{Line: 2, Column: 3}, parser.Position{Line: 2, Column: 3}, false},
		{"later column", parser.Position{Line: 2, Column: 8}, parser.Position{Line: 2, Column: 3}, false},
		{"unknown first column", parser.Position{Line: 2}, parser.Position{Line: 2, Column: 3}, false},
		{"unknown second column", parser.Position{Line: 2, Column: 3}, parser.Position{Line: 2}, false},
		{"unknown position line", parser.Position{}, parser.Position{Line: 2}, true},
		{"unknown start line", parser.Position{Line: 2}, parser.Position{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := nestedPositionBefore(tc.position, tc.start); got != tc.want {
				t.Fatalf("got %t; want %t", got, tc.want)
			}
		})
	}
}
