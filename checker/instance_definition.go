package checker

import "github.com/upamune/tla-go/syntax/parser"

func declarationNames(params []parser.Declaration) map[string]bool {
	names := make(map[string]bool, len(params))
	for _, param := range params {
		names[param.Name] = true
	}
	return names
}

func instanceDefinition(definition parser.Definition, instance parser.Instance, replacements map[string]*parser.Expression, generated map[string]string, generatedSet, variables map[string]bool, scope string) parser.Definition {
	definition.Origins = instanceOrigins(definition, instance)
	definition.InstanceRange = instance.Location.Range
	definition.InstanceParamCount = len(instance.Params)
	definition.Name = generated[definition.Name]
	definition.Local = definition.Local || instance.Local
	definition.Params = append(append([]parser.Declaration(nil), instance.Params...), definition.Params...)
	self := ""
	if definition.Kind == "Function" {
		self = definition.Name
	}
	body := substitute(definition.Body, nil)
	markEnabledInstanceVariables(body, variables, scope)
	definition.Body = instanceExpression(body, replacements, generatedSet, instance.Params, self)
	definition.Bounds = instanceBounds(definition.Bounds, replacements, generatedSet, instance.Params)
	definition.Location.Position = instance.Location.Position
	return definition
}

func markEnabledInstanceVariables(expr *parser.Expression, variables map[string]bool, scope string) {
	if expr == nil {
		return
	}
	for i := range expr.EnabledScopes {
		expr.EnabledScopes[i] = scope + expr.EnabledScopes[i]
	}
	for i := range expr.EnabledBindings {
		expr.EnabledBindings[i].Scope = scope + expr.EnabledBindings[i].Scope
	}
	if expr.Kind == "Enabled" {
		expr.EnabledScopes = append(expr.EnabledScopes, scope)
	}
	markEnabledPrime(expr, variables, scope)
	walkChildren(expr, func(child *parser.Expression) { markEnabledInstanceVariables(child, variables, scope) })
}

func markEnabledPrime(expr *parser.Expression, variables map[string]bool, scope string) {
	if expr.Kind != "Prime" || len(expr.Args) != 1 {
		return
	}
	arg := expr.Args[0]
	if arg.Kind == "Identifier" && variables[arg.Text] {
		expr.EnabledBindings = append(expr.EnabledBindings, parser.EnabledBinding{Scope: scope, Name: arg.Text})
	}
}

func instanceOrigins(definition parser.Definition, instance parser.Instance) []parser.DefinitionOrigin {
	origins := append([]parser.DefinitionOrigin(nil), definition.Origins...)
	if len(origins) == 0 && definition.Location.Range.Module != "" {
		origins = append(origins, parser.DefinitionOrigin{Module: definition.Location.Range.Module, Name: definition.Name})
	}
	for i := range origins {
		origins[i].Prefix += len(instance.Params)
	}
	return append(origins, parser.DefinitionOrigin{Module: instance.Module, Name: definition.Name, Prefix: len(instance.Params)})
}

func instanceExpression(expr *parser.Expression, replacements map[string]*parser.Expression, generated map[string]bool, params []parser.Declaration, self string) *parser.Expression {
	result := substitute(substitute(expr, replacements), nil)
	if len(params) > 0 {
		addInstanceArguments(result, generated, params, self)
	}
	return result
}

func instanceBounds(bounds []parser.Bound, replacements map[string]*parser.Expression, generated map[string]bool, params []parser.Declaration) []parser.Bound {
	out := append([]parser.Bound(nil), bounds...)
	for i := range out {
		if out[i].Set != nil {
			out[i].Set = instanceExpression(out[i].Set, replacements, generated, params, "")
		}
	}
	return out
}

func addInstanceArguments(expr *parser.Expression, generated map[string]bool, params []parser.Declaration, self string) {
	if (expr.Kind == "Identifier" || expr.Kind == "Apply") && generated[expr.Text] && expr.Text != self {
		args := make([]*parser.Expression, 0, len(params)+len(expr.Args))
		for _, param := range params {
			args = append(args, &parser.Expression{Kind: "Identifier", Text: param.Name})
		}
		expr.Args = append(args, expr.Args...)
		expr.Kind = "Apply"
	}
	walkChildren(expr, func(child *parser.Expression) { addInstanceArguments(child, generated, params, self) })
}
