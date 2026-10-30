package checker

import (
	"testing"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func TestActionCacheRespectsLexicalDefinitions(t *testing.T) {
	module, err := parser.ParseModule("---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nF(v) == x' = v\nCall == F(1)\n====")
	if err != nil {
		t.Fatal(err)
	}
	shadow, err := parser.ParseModule("---- MODULE N ----\nEXTENDS Naturals\nVARIABLE x\nF(v) == x' = v+1\n====")
	if err != nil {
		t.Fatal(err)
	}
	root, call := module.Definitions[0], module.Definitions[1].Body
	m := &model{defs: map[string]parser.Definition{"F": root}, actionCache: map[*parser.Expression]*parser.Expression{}, program: evaluator.New(module)}
	g := generator{model: m, lexical: m.program.ExtendScope(nil, nil, nil)}
	first, err := g.actionBody(call)
	if err != nil {
		t.Fatal(err)
	}
	second, err := g.actionBody(call)
	if err != nil || first != second {
		t.Fatalf("module action was not reused: %v", err)
	}
	g.lexical = m.program.ExtendScope(nil, shadow.Definitions, nil)
	local, err := g.actionBody(call)
	if err != nil {
		t.Fatal(err)
	}
	zero, _ := value.Integer(0)
	one, _ := value.Integer(1)
	ctx := evaluator.Context{Current: evaluator.State{"x": zero}, Next: evaluator.State{"x": one}, Lexical: g.lexical}
	got, err := m.program.Evaluate(local, ctx)
	valid, ok := got.Bool()
	if err != nil || !ok || valid {
		t.Fatalf("shadowed action reused module body: %v, %v", got, err)
	}
}

func TestActionCacheRejectsDifferentFormalParameters(t *testing.T) {
	body := &parser.Expression{Kind: "Identifier", Text: "v"}
	root := parser.Definition{Kind: "Operator", Name: "F", Body: body, Params: []parser.Declaration{{Name: "v"}}}
	local := root
	local.Params = []parser.Declaration{{Name: "w"}}
	g := generator{model: &model{defs: map[string]parser.Definition{"F": root}}, lexical: &evaluator.LexicalScope{}}
	if g.cacheableActionBody(&parser.Expression{Kind: "Apply", Text: "F"}, local) {
		t.Fatal("different parameter bindings must not share a cached body")
	}
}

func TestFunctionActionCacheRespectsBounds(t *testing.T) {
	body := &parser.Expression{Kind: "Identifier", Text: "v"}
	domain := &parser.Expression{Kind: "Identifier", Text: "D"}
	root := parser.Definition{Kind: "Function", Name: "F", Body: body, Bounds: []parser.Bound{{Names: []string{"i"}, Set: domain}}}
	local := root
	local.Bounds = []parser.Bound{{Names: []string{"j"}, Set: domain}}
	g := generator{model: &model{defs: map[string]parser.Definition{"F": root}}, lexical: &evaluator.LexicalScope{}}
	call := &parser.Expression{Kind: "Apply", Text: "F"}
	if g.cacheableFunctionAction(call, local) {
		t.Fatal("functions with different bounds must not share a cached body")
	}
	if !g.cacheableFunctionAction(call, root) {
		t.Fatal("unchanged module functions should use the cache")
	}
}
