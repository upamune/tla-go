package parser

import (
	"fmt"

	"github.com/upamune/tla-go/syntax"
)

type parser struct {
	tokens    []syntax.Token
	pos       int
	module    string
	offside   int
	recursive []map[string]bool
	modules   []*Module
}

func (p *parser) raw() syntax.Token { return p.tokens[p.pos] }
func (p *parser) peek() syntax.Token {
	t := p.raw()
	if p.offside > 0 && t.Column <= p.offside && t.Kind != "End" {
		t.Kind = "End"
		t.Text = ""
	}
	return t
}

func (p *parser) look(offset int) syntax.Token {
	index := min(p.pos+offset, len(p.tokens)-1)
	t := p.tokens[index]
	if p.offside > 0 && t.Column <= p.offside && t.Kind != "End" {
		t.Kind = "End"
		t.Text = ""
	}
	return t
}

func (p *parser) next() syntax.Token {
	t := p.peek()
	if t.Kind != "End" {
		p.pos++
	}
	return t
}

func (p *parser) at(text string) bool {
	t := p.peek()
	return t.Text == text && t.Kind != "String" && t.Kind != "End"
}

func (p *parser) accept(text string) bool {
	if !p.at(text) {
		return false
	}
	p.next()
	return true
}

func (p *parser) error(message string) error {
	t := p.peek()
	return &syntax.SyntaxError{Reason: message, Line: t.Line, Column: t.Column, IsEnd: t.Kind == "End"}
}

func (p *parser) expect(text string) error {
	if !p.accept(text) {
		return p.error(fmt.Sprintf("Expected %q but found %q", text, p.peek().Text))
	}
	return nil
}

func (p *parser) name() (string, error) {
	t := p.peek()
	if t.Kind != "Identifier" {
		return "", p.error(fmt.Sprintf("Expected an identifier but found %q", t.Text))
	}
	p.next()
	return t.Text, nil
}
func position(t syntax.Token) Position { return Position{t.Line, t.Column} }
func (p *parser) span(start int) Range {
	a, b := p.tokens[start], p.tokens[p.pos-1]
	return Range{p.module, position(a), Position{b.Line, b.EndColumn}}
}

func (p *parser) location(t syntax.Token, start int) Location {
	return Location{position(t), p.span(start)}
}

// ParseModule parses a TLA+ module. Unsupported syntax returns an error rather
// than being silently ignored; additional forms are being ported from tla-ts.
func ParseModule(source string) (*Module, error) {
	tokens, err := syntax.Tokenize(source)
	if err != nil {
		return nil, err
	}
	p := parser{tokens: tokens}
	module, err := p.parseModule()
	if err != nil {
		return nil, err
	}
	module.SourceTokens = tokens
	return module, nil
}

func (p *parser) header() (string, error) {
	if err := p.expect("----"); err != nil {
		return "", err
	}
	if err := p.expect("MODULE"); err != nil {
		return "", err
	}
	name, err := p.name()
	if err != nil {
		return "", err
	}
	p.module = name
	return name, p.expect("----")
}

func (p *parser) parseModule() (*Module, error) {
	start := p.pos
	head := p.peek()
	parent := p.module
	p.pushRecursiveScope()
	defer func() { p.popRecursiveScope(); p.module = parent }()
	name, err := p.header()
	if err != nil {
		return nil, err
	}
	m := &Module{Name: name}
	p.modules = append(p.modules, m)
	defer func() { p.modules = p.modules[:len(p.modules)-1] }()
	if p.accept("EXTENDS") {
		m.Extends, err = p.names()
		if err != nil {
			return nil, err
		}
	}
	for p.peek().Kind != "ModuleEnd" && p.peek().Kind != "End" {
		if err = p.unit(m); err != nil {
			return nil, err
		}
	}
	if p.peek().Kind == "ModuleEnd" {
		p.next()
	}
	m.Location = p.location(head, start)
	return m, nil
}

func (p *parser) names() ([]string, error) {
	var names []string
	for {
		name, err := p.name()
		if err != nil {
			return nil, err
		}
		names = append(names, name)
		if !p.accept(",") {
			return names, nil
		}
	}
}

func isAssumption(text string) bool {
	return text == "ASSUME" || text == "ASSUMPTION" || text == "AXIOM"
}

func (p *parser) unit(m *Module) error {
	if p.peek().Kind == "Separator" {
		return p.moduleSeparator(m)
	}
	if p.at("VARIABLE") || p.at("VARIABLES") {
		p.next()
		decls, err := p.declarations(false)
		m.Variables = append(m.Variables, decls...)
		return err
	}
	if p.at("CONSTANT") || p.at("CONSTANTS") {
		p.next()
		decls, err := p.declarations(true)
		m.Constants = append(m.Constants, decls...)
		return err
	}
	if handled, err := p.assertion(m); handled {
		return err
	}
	if p.accept("RECURSIVE") {
		return p.moduleRecursive(m)
	}
	return p.moduleDefinition(m)
}

func (p *parser) assertion(m *Module) (bool, error) {
	text := p.peek().Text
	if isAssumption(text) {
		return true, p.assumption(m)
	}
	if isTheorem(text) {
		return true, p.theorem(m)
	}
	return false, nil
}

func (p *parser) moduleRecursive(m *Module) error {
	decls, err := p.declareRecursive()
	if err != nil {
		return err
	}
	m.RecursiveDeclarations = append(m.RecursiveDeclarations, decls...)
	return nil
}

func (p *parser) moduleSeparator(m *Module) error {
	if p.look(1).Text != "MODULE" {
		p.next()
		return nil
	}
	submodule, err := p.parseModule()
	if err != nil {
		return err
	}
	m.Modules = append(m.Modules, *submodule)
	return nil
}

func (p *parser) moduleDefinition(m *Module) error {
	start := p.pos
	token := p.peek()
	local := p.accept("LOCAL")
	if p.at("INSTANCE") {
		inst, err := p.instance("", nil, local, token, start, -1)
		if err != nil {
			return err
		}
		m.Instances = append(m.Instances, *inst)
		return nil
	}
	def, inst, err := p.definition(local)
	if err != nil {
		return err
	}
	if inst != nil {
		m.Instances = append(m.Instances, *inst)
	} else {
		m.Definitions = append(m.Definitions, def)
	}
	return nil
}

func (p *parser) assumption(m *Module) error {
	start := p.pos
	t := p.next()
	a := Assumption{Axiom: t.Text == "AXIOM", Location: Location{Position: position(t)}}
	if p.peek().Kind == "Identifier" && p.look(1).Text == "==" {
		a.Name = p.next().Text
		p.next()
	}
	expression, err := p.expression(0)
	if err != nil {
		return err
	}
	a.Expression = expression
	a.Location.Range = p.span(start)
	m.Assumptions = append(m.Assumptions, a)
	if a.Name != "" {
		m.Definitions = append(m.Definitions, Definition{Kind: "Operator", Name: a.Name, Body: expression, Location: a.Location})
	}
	return nil
}

func (p *parser) declaration(operatorParams bool) (Declaration, error) {
	start := p.pos
	t := p.peek()
	if operatorParams && p.isOperatorDeclaration() {
		return p.operatorDeclaration()
	}
	name, err := p.name()
	if err != nil {
		return Declaration{}, err
	}
	arity := 0
	if operatorParams && p.accept("(") {
		arity, err = p.placeholders()
		if err != nil {
			return Declaration{}, err
		}
	}
	location := p.location(t, start)
	if operatorParams {
		location.Range = Range{}
	}
	return Declaration{name, arity, location}, nil
}

func (p *parser) declarations(operatorParams bool) ([]Declaration, error) {
	var result []Declaration
	for {
		item, err := p.declaration(operatorParams)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
		if !p.accept(",") {
			return result, nil
		}
	}
}

func (p *parser) placeholders() (int, error) {
	count := 0
	for {
		if err := p.expect("_"); err != nil {
			return 0, err
		}
		count++
		if !p.accept(",") {
			break
		}
	}
	return count, p.expect(")")
}

func (p *parser) definition(local bool) (Definition, *Instance, error) {
	start := p.pos
	t := p.peek()
	if p.isOperatorDefinition() {
		def, err := p.operatorDefinition(local)
		return def, nil, err
	}
	name, err := p.name()
	if err != nil {
		return Definition{}, nil, err
	}
	if p.accept("[") {
		def, err := p.functionDefinition(start, t, name, local)
		return def, nil, err
	}
	definitionStart := p.pos
	params, err := p.parameters()
	if err != nil {
		return Definition{}, nil, err
	}
	if err = p.expect("=="); err != nil {
		return Definition{}, nil, err
	}
	if p.at("INSTANCE") {
		inst, err := p.instance(name, params, local, t, start, definitionStart)
		return Definition{}, inst, err
	}
	bodyStart := p.pos
	body, err := p.expression(0)
	if err != nil {
		return Definition{}, nil, err
	}
	return Definition{Kind: "Operator", Recursive: p.isRecursive(name), Name: name, Params: params, Body: body, Local: local, Location: p.location(t, start), BodyRange: p.span(bodyStart)}, nil, nil
}

func (p *parser) parameters() ([]Declaration, error) {
	if !p.accept("(") {
		return nil, nil
	}
	params, err := p.declarations(true)
	if err != nil {
		return nil, err
	}
	return params, p.expect(")")
}
