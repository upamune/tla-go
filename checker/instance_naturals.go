package checker

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
)

func builtinNaturalsModule() *parser.Module {
	module := &parser.Module{Name: "Naturals"}
	module.Definitions = append(module.Definitions, parser.Definition{Kind: "Operator", Name: "Nat", Body: &parser.Expression{Kind: "Identifier", Text: "!naturals!Nat", BuiltinNatural: true}})
	for _, name := range []string{"+", "-", "*", "^", "<", ">", "<=", ">=", "%", "\\div", ".."} {
		definition := parser.Definition{Kind: "Operator", Name: name}
		body := &parser.Expression{Kind: "Apply", Text: "!naturals!" + name, BuiltinNatural: true}
		for i := range 2 {
			param := parser.Declaration{Name: fmt.Sprintf("!naturals-argument-%d", i)}
			definition.Params = append(definition.Params, param)
			body.Args = append(body.Args, &parser.Expression{Kind: "Identifier", Text: param.Name})
		}
		definition.Body = body
		module.Definitions = append(module.Definitions, definition)
	}
	return module
}
