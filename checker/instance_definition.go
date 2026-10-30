package checker

import "github.com/upamune/tla-go/syntax/parser"

func declarationNames(params []parser.Declaration) map[string]bool {
	names := make(map[string]bool, len(params))
	for _, param := range params {
		names[param.Name] = true
	}
	return names
}

func instanceDefinition(definition parser.Definition, instance parser.Instance, replacements map[string]*parser.Expression, generated map[string]string, generatedSet, variables map[string]bool) parser.Definition {
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
	markEnabledInstanceVariables(body, variables, false)
	definition.Body = instanceExpression(body, replacements, generatedSet, instance.Params, self)
	definition.Bounds = instanceBounds(definition.Bounds, replacements, generatedSet, instance.Params)
	definition.Location.Position = instance.Location.Position
	return definition
}

func markEnabledInstanceVariables(expr *parser.Expression, variables map[string]bool, inside bool) {
	if expr == nil {
		return
	}
	inside = inside || expr.Kind == "Enabled"
	if inside {
		markEnabledPrime(expr, variables)
	}
	walkChildren(expr, func(child *parser.Expression) { markEnabledInstanceVariables(child, variables, inside) })
}

func markEnabledPrime(expr *parser.Expression, variables map[string]bool) {
	if expr.Kind != "Prime" || expr.EnabledVariable != "" || len(expr.Args) != 1 {
		return
	}
	arg := expr.Args[0]
	if arg.Kind == "Identifier" && variables[arg.Text] {
		expr.EnabledVariable = arg.Text
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
