package parser

import (
	"fmt"
)

func (p *parser) primary() (*Expression, error) {
	start := p.pos
	t := p.peek()
	switch t.Kind {
	case "Number", "Decimal", "String":
		p.next()
		return node(t.Kind, t.Text, p.location(t, start)), nil
	case "Identifier":
		return p.identifier()
	}
	return p.primaryForm()
}

func (p *parser) primaryForm() (*Expression, error) {
	t := p.peek()
	switch t.Text {
	case "(":
		p.next()
		expr, err := p.withoutOffside()
		if err != nil {
			return nil, err
		}
		if err = p.expect(")"); err != nil {
			return nil, err
		}
		return expr, nil
	case "{":
		return p.brace()
	case "<<":
		return p.tuple()
	case "[":
		return p.bracket()
	case "IF":
		return p.ifExpression()
	}
	return p.otherPrimary()
}

func (p *parser) otherPrimary() (*Expression, error) {
	t := p.peek()
	switch t.Text {
	case "@":
		start := p.pos
		p.next()
		return node("ExceptAt", "", p.location(t, start)), nil
	case "\\A", "\\E", "\\AA", "\\EE":
		return p.quantifier()
	case "CHOOSE":
		return p.choose()
	case "CASE":
		return p.caseExpression()
	case "LET":
		return p.letExpression()
	case "WF_", "SF_":
		return p.fairness()
	}
	return nil, p.error(fmt.Sprintf("Unexpected %q", t.Text))
}

func (p *parser) collection(kind, end string) (*Expression, error) {
	start := p.pos
	t := p.next()
	args, err := p.delimited(end, true)
	if err != nil {
		return nil, err
	}
	return node(kind, "", p.location(t, start), args...), nil
}

func (p *parser) delimited(end string, empty bool) ([]*Expression, error) {
	previous := p.offside
	p.offside = 0
	defer func() { p.offside = previous }()
	var args []*Expression
	if empty && p.accept(end) {
		return args, nil
	}
	for {
		arg, err := p.expression(0)
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
		if !p.accept(",") {
			break
		}
	}
	return args, p.expect(end)
}

func (p *parser) withoutOffside() (*Expression, error) {
	previous := p.offside
	p.offside = 0
	defer func() { p.offside = previous }()
	return p.expression(0)
}

func (p *parser) ifExpression() (*Expression, error) {
	start := p.pos
	t := p.next()
	condition, err := p.expression(0)
	if err != nil {
		return nil, err
	}
	if err = p.expect("THEN"); err != nil {
		return nil, err
	}
	yes, err := p.expression(0)
	if err != nil {
		return nil, err
	}
	if err = p.expect("ELSE"); err != nil {
		return nil, err
	}
	no, err := p.expression(0)
	if err != nil {
		return nil, err
	}
	return node("If", "", p.location(t, start), condition, yes, no), nil
}
