package checker

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/upamune/tla-go/syntax/parser"
)

func (r *instanceResolver) instanceSource(name string) (string, *parser.Module, error) {
	if r.loader == nil {
		return r.missingInstanceSource(name, nil)
	}
	source, err := r.loader(r.ctx, name)
	if err != nil {
		return r.missingInstanceSource(name, err)
	}
	return source, nil, nil
}

func (r *instanceResolver) missingInstanceSource(name string, err error) (string, *parser.Module, error) {
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		if module := r.builtinInstanceModule(name); module != nil {
			return "", module, nil
		}
	}
	if err == nil {
		return "", nil, fmt.Errorf("instance module %s requires a module loader", name)
	}
	return "", nil, moduleLoadError("instance module", name, err)
}

func (r *instanceResolver) builtinInstanceModule(name string) *parser.Module {
	switch name {
	case "TLC":
		r.session.builtinTLC = true
		return builtinTLCModule()
	case "Naturals":
		return builtinNaturalsModule()
	default:
		return nil
	}
}

// builtinTLCModule represents the public operators of TLC. Their bodies use
// internal calls so that a named INSTANCE does not import unqualified names.
func builtinTLCModule() *parser.Module {
	operators := []string{"Print", "PrintT", "Assert", "TLCGet", "TLCSet", ":>", "@@", "Permutations", "SortSeq", "RandomElement", "ToString", "TLCEval"}
	module := &parser.Module{Name: "TLC"}
	for _, name := range operators {
		module.Definitions = append(module.Definitions, builtinTLCDefinition(name, tlcOperators[name]))
	}
	for _, name := range []string{"Any", "JavaTime"} {
		module.Definitions = append(module.Definitions, parser.Definition{Kind: "Operator", Name: name, Body: &parser.Expression{Kind: "Identifier", Text: "!tlc!" + name, BuiltinTLC: true}})
	}
	return module
}

func isBuiltinStandardModule(module *parser.Module) bool {
	if len(module.Definitions) == 0 || module.Definitions[0].Body == nil {
		return false
	}
	return module.Definitions[0].Body.BuiltinTLC || module.Definitions[0].Body.BuiltinNatural
}

func addBuiltinInstanceArguments(body *parser.Expression, params []parser.Declaration) {
	if body == nil || !(body.BuiltinTLC || body.BuiltinNatural) || len(params) == 0 {
		return
	}
	args := make([]*parser.Expression, 0, len(params)+len(body.Args))
	for _, param := range params {
		args = append(args, &parser.Expression{Kind: "Identifier", Text: param.Name})
	}
	body.Kind = "Apply"
	body.Args = append(args, body.Args...)
}

func builtinTLCDefinition(name string, arity int) parser.Definition {
	definition := parser.Definition{Kind: "Operator", Name: name}
	body := &parser.Expression{Kind: "Apply", Text: "!tlc!" + name, BuiltinTLC: true}
	for i := range arity {
		param := parser.Declaration{Name: fmt.Sprintf("!tlc-argument-%d", i)}
		if name == "SortSeq" && i == 1 {
			param.Arity = 2
		}
		definition.Params = append(definition.Params, param)
		body.Args = append(body.Args, &parser.Expression{Kind: "Identifier", Text: param.Name})
	}
	definition.Body = body
	return definition
}

func tlcOperatorName(expr *parser.Expression) string {
	if expr.BuiltinTLC {
		return strings.TrimPrefix(expr.Text, "!tlc!")
	}
	return expr.Text
}
