package checker

import "github.com/upamune/tla-go/syntax/parser"

func isolateLocalDefinitions(module *parser.Module) (*parser.Module, error) {
	replacements := localReplacements(module)
	if len(replacements) == 0 {
		return module, nil
	}
	renamer := alphaRenamer{}
	definitions := make([]parser.Definition, len(module.Definitions))
	for i, definition := range module.Definitions {
		definitions[i] = renamer.definition(definition, nil)
	}
	assumptions := append([]parser.Assumption(nil), module.Assumptions...)
	for i := range assumptions {
		assumptions[i].Expression = renamer.expression(assumptions[i].Expression, nil)
	}
	result := *module
	result.Definitions = renameLocalDefinitions(definitions, replacements)
	result.Assumptions = renameLocalAssumptions(assumptions, replacements)
	result.ProofDirectives = renameLocalProofDirectives(module.ProofDirectives, replacements)
	result.RecursiveDeclarations = renameLocalDeclarations(module.RecursiveDeclarations, replacements)
	return &result, nil
}

func localReplacements(module *parser.Module) map[string]*parser.Expression {
	replacements := map[string]*parser.Expression{}
	for _, d := range module.Definitions {
		if d.Local {
			replacements[d.Name] = &parser.Expression{Kind: "Identifier", Text: "!local!" + module.Name + "!" + d.Name}
		}
	}
	return replacements
}

func renameLocalDefinitions(definitions []parser.Definition, replacements map[string]*parser.Expression) []parser.Definition {
	out := append([]parser.Definition(nil), definitions...)
	for i := range out {
		if replacement, ok := replacements[out[i].Name]; ok {
			out[i].Name = replacement.Text
			out[i].Local = false
		}
		out[i].Body = substitute(out[i].Body, replacements)
		out[i].Bounds = subBounds(out[i].Bounds, replacements)
	}
	return out
}

func renameLocalAssumptions(assumptions []parser.Assumption, replacements map[string]*parser.Expression) []parser.Assumption {
	out := append([]parser.Assumption(nil), assumptions...)
	for i := range out {
		out[i].Expression = substitute(out[i].Expression, replacements)
	}
	return out
}

func renameLocalDeclarations(declarations []parser.Declaration, replacements map[string]*parser.Expression) []parser.Declaration {
	out := append([]parser.Declaration(nil), declarations...)
	for i := range out {
		if replacement, ok := replacements[out[i].Name]; ok {
			out[i].Name = replacement.Text
		}
	}
	return out
}
