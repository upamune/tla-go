package parser

import "github.com/upamune/tla-go/syntax"

func (p *parser) functionDefinition(start int, t syntax.Token, name string, local bool) (Definition, error) {
	old := p.offside
	p.offside = 0
	bounds, err := p.bounds()
	p.offside = old
	if err != nil {
		return Definition{}, err
	}
	if err = p.expect("]"); err != nil {
		return Definition{}, err
	}
	if err = p.expect("=="); err != nil {
		return Definition{}, err
	}
	body, err := p.expression(0)
	if err != nil {
		return Definition{}, err
	}
	return Definition{Kind: "Function", Recursive: p.isRecursive(name), Bounds: bounds, Name: name, Body: body, Local: local, Location: p.location(t, start)}, nil
}

func (p *parser) isOperatorDefinition() bool {
	t := p.peek()
	if t.Text == "-." && p.look(1).Kind == "Identifier" && p.look(2).Text == "==" {
		return true
	}
	if t.Kind != "Identifier" {
		return false
	}
	return p.isInfixOrPostfixDefinition()
}

func (p *parser) isInfixOrPostfixDefinition() bool {
	op := p.look(1)
	if postfixOperator(op.Text) && p.look(2).Text == "==" {
		return true
	}
	_, ok := infix[op.Text]
	return ok && p.look(2).Kind == "Identifier" && p.look(3).Text == "=="
}

func (p *parser) operatorDefinition(local bool) (Definition, error) {
	start := p.pos
	t := p.next()
	operator, params, err := p.operatorDefinitionHead(t)
	if err != nil {
		return Definition{}, err
	}
	if err := p.expect("=="); err != nil {
		return Definition{}, err
	}
	bodyStart := p.pos
	body, err := p.expression(0)
	if err != nil {
		return Definition{}, err
	}
	return Definition{Kind: "Operator", Recursive: p.isRecursive(operator), Name: operator, Params: params, Body: body, Local: local, Location: p.location(t, start), BodyRange: p.span(bodyStart)}, nil
}

func (p *parser) operatorDefinitionHead(t syntax.Token) (string, []Declaration, error) {
	if t.Text == "-." {
		name, err := p.name()
		return t.Text, []Declaration{{Name: name}}, err
	}
	operator := p.next().Text
	params := []Declaration{{Name: t.Text}}
	if !postfixOperator(operator) {
		name, err := p.name()
		if err != nil {
			return "", nil, err
		}
		params = append(params, Declaration{Name: name})
	}
	return operator, params, nil
}
