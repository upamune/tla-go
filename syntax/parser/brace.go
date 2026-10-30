package parser

import "github.com/upamune/tla-go/syntax"

func (p *parser) brace() (*Expression, error) {
	start := p.pos
	t := p.next()
	previous := p.offside
	p.offside = 0
	defer func() { p.offside = previous }()
	if p.accept("}") {
		return node("SetEnumeration", "", p.location(t, start)), nil
	}
	if p.boundAhead() && (p.look(1).Text == "\\in" || p.at("<<")) {
		return p.setFilter(start, t)
	}
	return p.setBody(start, t)
}

func (p *parser) setFilter(start int, t syntax.Token) (*Expression, error) {
	checkpoint := p.pos
	pattern, err := p.pattern()
	if err != nil {
		return nil, err
	}
	if err = p.expect("\\in"); err != nil {
		return nil, err
	}
	pattern.Set, err = p.expression(0)
	if err != nil {
		return nil, err
	}
	if !p.accept(":") {
		p.pos = checkpoint
		return p.setBody(start, t)
	}
	predicate, err := p.expression(0)
	if err != nil {
		return nil, err
	}
	if err = p.expect("}"); err != nil {
		return nil, err
	}
	result := node("SetFilter", "", p.location(t, start), predicate)
	result.Bounds = []Bound{pattern}
	return result, nil
}

func (p *parser) setBody(start int, t syntax.Token) (*Expression, error) {
	first, err := p.expression(0)
	if err != nil {
		return nil, err
	}
	if p.accept(":") {
		bounds, err := p.bounds()
		if err != nil {
			return nil, err
		}
		if err = p.expect("}"); err != nil {
			return nil, err
		}
		result := node("SetMap", "", p.location(t, start), first)
		result.Bounds = bounds
		return result, nil
	}
	elements := []*Expression{first}
	for p.accept(",") {
		next, err := p.expression(0)
		if err != nil {
			return nil, err
		}
		elements = append(elements, next)
	}
	if err = p.expect("}"); err != nil {
		return nil, err
	}
	return node("SetEnumeration", "", p.location(t, start), elements...), nil
}
