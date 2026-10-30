package parser

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

// Expected ASTs were produced by tla-ts at the pinned commit in README.md.
func TestUpstreamExamples(t *testing.T) {
	data, err := os.ReadFile("testdata/upstream-examples.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name, Source string
		Want         Module
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			got, err := ParseModule(f.Source)
			if err != nil {
				t.Fatal(err)
			}
			// Source tokens are metadata, not part of the upstream AST projection.
			got.SourceTokens = nil
			if !reflect.DeepEqual(*got, f.Want) {
				a, _ := json.Marshal(got)
				b, _ := json.Marshal(f.Want)
				t.Errorf("got %s\nwant %s", a, b)
			}
		})
	}
}

func TestPrecedenceConflicts(t *testing.T) {
	for _, expr := range []string{"7 * -2", "7 \\div -2", "1 = 2 = 3", "a + b ++ c"} {
		_, err := ParseModule("---- MODULE M ----\nA == " + expr + "\n====")
		if err == nil {
			t.Errorf("accepted %q", expr)
		}
	}
}

func TestSymbolicOperatorCalls(t *testing.T) {
	for _, tc := range []struct{ source, name string }{
		{"+(1, 2)", "+"},
		{"\\cup({1}, {2})", "\\cup"},
		{"^+(1)", "^+"},
		{"-.(1)", "-."},
		{"I!+(1, 2)", "I!+"},
	} {
		t.Run(tc.source, func(t *testing.T) {
			m, err := ParseModule("---- MODULE M ----\nA == " + tc.source + "\n====")
			if err != nil {
				t.Fatal(err)
			}
			if got := m.Definitions[0].Body; got.Kind != "Apply" || got.Text != tc.name {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestParenthesizedJunctionAndMinus(t *testing.T) {
	m, err := ParseModule("---- MODULE M ----\nA == /\\ (TRUE)\n     /\\ (TRUE)\nB == -(4)\n====")
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Definitions[0].Body; got.Kind != "And" || !got.Bulleted || len(got.Args) != 2 {
		t.Fatalf("junction: %+v", got)
	}
	if got := m.Definitions[1].Body; got.Kind != "Apply" || got.Text != "-." || len(got.Args) != 1 {
		t.Fatalf("minus: %+v", got)
	}
}

func TestPrefixAndPostfixOperatorArguments(t *testing.T) {
	for _, name := range []string{"[]", "<>", "ENABLED", "UNCHANGED", "SUBSET", "UNION", "DOMAIN", "'", "^+", "^*", "^#"} {
		t.Run(name, func(t *testing.T) {
			m, err := ParseModule("---- MODULE M ----\nA == F(" + name + ")\n====")
			if err != nil {
				t.Fatal(err)
			}
			arg := m.Definitions[0].Body.Args[0]
			if arg.Kind != "OperatorReference" || arg.Text != name {
				t.Fatalf("got %+v", arg)
			}
		})
	}
	m, err := ParseModule("---- MODULE M ----\nA == F(SUBSET UNION {{1}})\n====")
	if err != nil {
		t.Fatal(err)
	}
	arg := m.Definitions[0].Body.Args[0]
	if arg.Kind != "Apply" || arg.Text != "SUBSET" || arg.Args[0].Text != "UNION" {
		t.Fatalf("nested prefix expression: %+v", arg)
	}
	m, err = ParseModule("---- MODULE M ----\nA == F(\"[]\", \"^+\", \"+\")\n====")
	if err != nil {
		t.Fatal(err)
	}
	for _, arg := range m.Definitions[0].Body.Args {
		if arg.Kind != "String" {
			t.Fatalf("string argument: %+v", arg)
		}
	}
}

func TestUnsupportedSyntaxFails(t *testing.T) {
	_, err := ParseModule("---- MODULE M ----\nA == THEOREM\n====")
	if err == nil {
		t.Fatal("accepted invalid expression")
	}
}

func BenchmarkParseModule(b *testing.B) {
	source := "---- MODULE Counter ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = x + 1\nSpec == Init /\\ [][Next]_x\n===="
	for i := 0; i < b.N; i++ {
		if _, err := ParseModule(source); err != nil {
			b.Fatal(err)
		}
	}
}

func TestParameterizedInstanceRequiresArguments(t *testing.T) {
	source := "---- MODULE M ----\n---- MODULE Inner ----\nVARIABLE x\nFoo == x\n====\nI(v) == INSTANCE Inner\nA == I!Foo(1)\n===="
	_, err := ParseModule(source)
	if err == nil || !strings.Contains(err.Error(), "The operator I requires 1 arguments") {
		t.Fatalf("got %v", err)
	}
}

func TestUnsupportedProofIsRejected(t *testing.T) {
	_, err := ParseModule("---- MODULE M ----\nTHEOREM T == TRUE\n<1>1. TRUE\n<1> QED\n====")
	if err == nil || !strings.Contains(err.Error(), "Unsupported proof form") {
		t.Fatalf("got %v", err)
	}
}
