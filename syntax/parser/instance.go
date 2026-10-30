package parser

import "github.com/upamune/tla-go/syntax"

func (p *parser) instance(name string, params []Declaration, local bool, token syntax.Token, start, definitionStart int) (*Instance, error) {
	instanceStart := p.pos
	if err := p.expect("INSTANCE"); err != nil {
		return nil, err
	}
	module, err := p.name()
	if err != nil {
		return nil, err
	}
	var substitutions []Substitution
	if p.accept("WITH") {
		substitutions, err = p.substitutions()
		if err != nil {
			return nil, err
		}
	}
	result := &Instance{Name: name, Params: params, Module: module, Substitutions: substitutions, Local: local, Location: p.location(token, start), InstanceRange: p.span(instanceStart)}
	if definitionStart >= 0 {
		result.DefinitionRange = p.span(definitionStart)
	}
	return result, nil
}

func (p *parser) substitutions() ([]Substitution, error) {
	var substitutions []Substitution
	for {
		item, err := p.substitution()
		if err != nil {
			return nil, err
		}
		substitutions = append(substitutions, item)
		if !p.accept(",") {
			return substitutions, nil
		}
	}
}

func (p *parser) substitution() (Substitution, error) {
	t := p.next()
	if t.Kind == "End" {
		return Substitution{}, p.error("Expected a substitution target")
	}
	if err := p.expect("<-"); err != nil {
		return Substitution{}, err
	}
	expression, err := p.argument()
	if err != nil {
		return Substitution{}, err
	}
	return Substitution{t.Text, expression}, nil
}

func (p *parser) argument() (*Expression, error) {
	t := p.peek()
	if p.at("LAMBDA") {
		return p.lambda()
	}
	if (t.Kind == "Operator" || t.Kind == "Keyword") && operatorArgument(t.Text) && p.standaloneArgument() {
		start := p.pos
		p.next()
		return node("OperatorReference", t.Text, p.location(t, start)), nil
	}
	return p.expression(0)
}

func operatorArgument(name string) bool {
	if _, ok := infix[name]; ok {
		return true
	}
	if postfixOperator(name) {
		return true
	}
	_, _, _, ok := prefixInfo(name)
	return ok || name == "-."
}

func (p *parser) standaloneArgument() bool {
	t := p.look(1)
	if t.Kind == "Keyword" && expressionStartKeyword(t.Text) {
		return false
	}
	return p.definitionAfterArgument(t) || isArgumentEnd(t)
}

func expressionStartKeyword(name string) bool {
	if _, _, _, ok := prefixInfo(name); ok {
		return true
	}
	switch name {
	case "IF", "CASE", "CHOOSE", "LET", "LAMBDA", "WF_", "SF_":
		return true
	default:
		return false
	}
}

func (p *parser) definitionAfterArgument(t syntax.Token) bool {
	if t.Kind != "Identifier" {
		return false
	}
	next := p.look(2).Text
	if next == "==" {
		return true
	}
	return t.Line > p.peek().Line && (next == "(" || next == "[")
}

func isArgumentEnd(t syntax.Token) bool {
	return t.Kind == "End" || t.Kind == "ModuleEnd" || t.Kind == "Separator" || t.Text == "," || t.Text == ")" || t.Kind == "Keyword"
}
