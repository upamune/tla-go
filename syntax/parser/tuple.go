package parser

import "github.com/upamune/tla-go/syntax"

func (p *parser) tuple() (*Expression, error) {
	start := p.pos
	t := p.next()
	previous := p.offside
	p.offside = 0
	defer func() { p.offside = previous }()
	elements, err := p.tupleElements()
	if err != nil {
		return nil, err
	}
	return p.tupleEnd(start, t, elements)
}

func (p *parser) tupleElements() ([]*Expression, error) {
	if p.at(">>") || p.at(">>_") {
		return nil, nil
	}
	var elements []*Expression
	for {
		element, err := p.expression(0)
		if err != nil {
			return nil, err
		}
		elements = append(elements, element)
		if !p.accept(",") {
			return elements, nil
		}
	}
}

func (p *parser) tupleEnd(start int, t syntax.Token, elements []*Expression) (*Expression, error) {
	if p.accept(">>_") {
		if len(elements) != 1 {
			return nil, p.error("Expected a single action inside <<A>>_v")
		}
		subscript, err := p.subscript()
		if err != nil {
			return nil, err
		}
		return node("AngleAction", "", p.location(t, start), elements[0], subscript), nil
	}
	if err := p.expect(">>"); err != nil {
		return nil, err
	}
	return node("Tuple", "", p.location(t, start), elements...), nil
}
