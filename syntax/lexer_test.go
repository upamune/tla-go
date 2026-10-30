package syntax

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type fixture struct {
	Name   string  `json:"name"`
	Source string  `json:"source"`
	Tokens []Token `json:"tokens"`
}

// All TLA+ fixtures and expected tokens were generated with tla-ts at the
// commit recorded in README.md. The source files are distributed under its MIT license.
func TestUpstreamFixtureTokens(t *testing.T) {
	f, err := os.Open("testdata/upstream-lexer.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var fixtures []fixture
	if err = json.NewDecoder(z).Decode(&fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 139 {
		t.Fatalf("expected 139 upstream fixtures, got %d", len(fixtures))
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			tokens, err := Tokenize(f.Source)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(tokens, f.Tokens) {
				for i := range min(len(tokens), len(f.Tokens)) {
					if tokens[i] != f.Tokens[i] {
						t.Fatalf("token %d: got %+v, want %+v", i, tokens[i], f.Tokens[i])
					}
				}
				t.Fatalf("token count: got %d, want %d", len(tokens), len(f.Tokens))
			}
		})
	}
}

func TestLexicalEdges(t *testing.T) {
	tests := []struct {
		name, source string
		want         []Token
	}{
		{"aliases and tab", "---- MODULE M ----\n\tX == a ∈ S /\\ b /= 1\n====", []Token{{"Separator", "----", 1, 1, 4}, {"Keyword", "MODULE", 1, 6, 11}, {"Identifier", "M", 1, 13, 13}, {"Separator", "----", 1, 15, 18}, {"Identifier", "X", 2, 9, 9}, {"Operator", "==", 2, 11, 12}, {"Identifier", "a", 2, 14, 14}, {"Operator", "\\in", 2, 16, 16}, {"Identifier", "S", 2, 18, 18}, {"Operator", "/\\", 2, 20, 21}, {"Identifier", "b", 2, 23, 23}, {"Operator", "#", 2, 25, 26}, {"Number", "1", 2, 28, 28}, {"ModuleEnd", "====", 3, 1, 4}, {"End", "", 3, 5, 5}}},
		{"nested comment and step", "---- MODULE M ----\n(* outer (* inner *) *) <1>a.\n====", []Token{{"Separator", "----", 1, 1, 4}, {"Keyword", "MODULE", 1, 6, 11}, {"Identifier", "M", 1, 13, 13}, {"Separator", "----", 1, 15, 18}, {"Step", "<1>a.", 2, 25, 29}, {"ModuleEnd", "====", 3, 1, 4}, {"End", "", 3, 5, 5}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Tokenize(tt.source)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v\nwant %#v", got, tt.want)
			}
		})
	}
}

func TestLexicalErrors(t *testing.T) {
	for _, source := range []string{"no module", "---- MODULE M ----\n(* unfinished", "---- MODULE M ----\n\"unfinished\n====", "---- MODULE M ----\n`;\n===="} {
		if _, err := Tokenize(source); err == nil {
			t.Errorf("accepted %q", source)
		}
	}
}

func BenchmarkTokenize(b *testing.B) {
	source := "---- MODULE Counter ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = x + 1\n===="
	for i := 0; i < b.N; i++ {
		if _, err := Tokenize(source); err != nil {
			b.Fatal(err)
		}
	}
}
