package checker

import (
	"fmt"
	"maps"

	"github.com/upamune/tla-go/syntax/parser"
)

// alphaRenamer gives binders names that cannot occur in parsed source. Each
// instance has its own renamer so injected expressions cannot be captured.
type alphaRenamer struct {
	next       int
	prefix     string
	sequential bool
}

func alphaInstance(module *parser.Module, instance parser.Instance) (*parser.Module, parser.Instance) {
	renamer := alphaRenamer{}
	result := *module
	result.Definitions = make([]parser.Definition, len(module.Definitions))
	for i, definition := range module.Definitions {
		result.Definitions[i] = renamer.definition(definition, nil)
	}
	instance.Substitutions = append([]parser.Substitution(nil), instance.Substitutions...)
	for i := range instance.Substitutions {
		instance.Substitutions[i].Expression = renamer.expression(instance.Substitutions[i].Expression, nil)
	}
	return &result, instance
}

func (a *alphaRenamer) fresh(name string) string {
	a.next++
	if a.prefix != "" {
		return fmt.Sprintf("!%s!%d!%s", a.prefix, a.next, name)
	}
	return fmt.Sprintf("!bound!%d!%s", a.next, name)
}

func (a *alphaRenamer) definition(definition parser.Definition, outer map[string]string) parser.Definition {
	scope := maps.Clone(outer)
	if scope == nil {
		scope = map[string]string{}
	}
	definition.Params = append([]parser.Declaration(nil), definition.Params...)
	for i := range definition.Params {
		original := definition.Params[i].Name
		definition.Params[i].Name = a.fresh(original)
		scope[original] = definition.Params[i].Name
	}
	definition.Bounds, scope = a.bounds(definition.Bounds, scope)
	definition.Body = a.expression(definition.Body, scope)
	return definition
}

func (a *alphaRenamer) bounds(bounds []parser.Bound, outer map[string]string) ([]parser.Bound, map[string]string) {
	out := append([]parser.Bound(nil), bounds...)
	scope := maps.Clone(outer)
	if scope == nil {
		scope = map[string]string{}
	}
	for i := range out {
		if out[i].Set != nil {
			out[i].Set = a.expression(out[i].Set, scope)
		}
		out[i].Names = append([]string(nil), out[i].Names...)
		for j, name := range out[i].Names {
			out[i].Names[j] = a.fresh(name)
			scope[name] = out[i].Names[j]
		}
	}
	return out, scope
}

func (a *alphaRenamer) expression(expr *parser.Expression, outer map[string]string) *parser.Expression {
	if expr == nil {
		return nil
	}
	result := *expr
	renameAlphaHead(&result, outer)
	scope := maps.Clone(outer)
	if scope == nil {
		scope = map[string]string{}
	}
	result.Bounds, scope = a.bounds(expr.Bounds, scope)
	if expr.Kind == "Lambda" || expr.Kind == "UnboundedQuantifier" {
		result.Names = a.bindNames(expr.Names, scope)
	} else if expr.Kind == "Label" {
		result.Names = renameLabelParameters(expr.Names, scope)
	}
	result.Definitions, scope = a.localDefinitions(expr.Definitions, scope)
	a.expressionChildren(&result, expr, scope)
	return &result
}

func renameAlphaHead(expr *parser.Expression, scope map[string]string) {
	if expr.Kind == "Identifier" || expr.Kind == "Apply" || expr.Kind == "OperatorReference" {
		if name, ok := scope[expr.Text]; ok {
			expr.Text = name
		}
	}
}

func renameLabelParameters(names []string, scope map[string]string) []string {
	result := append([]string(nil), names...)
	for i, name := range result {
		if replacement, ok := scope[name]; ok {
			result[i] = replacement
		}
	}
	return result
}

func (a *alphaRenamer) bindNames(names []string, scope map[string]string) []string {
	out := append([]string(nil), names...)
	for i, name := range out {
		out[i] = a.fresh(name)
		scope[name] = out[i]
	}
	return out
}

func (a *alphaRenamer) localDefinitions(definitions []parser.Definition, outer map[string]string) ([]parser.Definition, map[string]string) {
	if len(definitions) == 0 {
		return nil, outer
	}
	if a.sequential {
		return a.sequentialDefinitions(definitions, outer)
	}
	scope := maps.Clone(outer)
	out := append([]parser.Definition(nil), definitions...)
	for i := range out {
		scope[out[i].Name] = a.fresh(out[i].Name)
	}
	for i := range out {
		out[i] = a.definition(out[i], scope)
		if out[i].SelectorName == "" {
			out[i].SelectorName = definitions[i].Name
		}
		out[i].Name = scope[definitions[i].Name]
	}
	return out, scope
}

func (a *alphaRenamer) sequentialDefinitions(definitions []parser.Definition, outer map[string]string) ([]parser.Definition, map[string]string) {
	scope := maps.Clone(outer)
	out := make([]parser.Definition, len(definitions))
	for i, definition := range definitions {
		fresh := a.fresh(definition.Name)
		scope[definition.Name] = fresh
		out[i] = a.definition(definition, scope)
		if out[i].SelectorName == "" {
			out[i].SelectorName = definition.Name
		}
		out[i].Name = fresh
	}
	return out, scope
}
