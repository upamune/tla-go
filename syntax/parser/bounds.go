package parser

import "github.com/upamune/tla-go/syntax"

func (p *parser) boundAhead() bool {
	if p.at("<<") {
		return p.tupleBoundAhead()
	}
	offset := 0
	for p.look(offset).Kind == "Identifier" {
		if p.look(offset+1).Text == "\\in" {
			return true
		}
		if p.look(offset+1).Text != "," {
			return false
		}
		offset += 2
	}
	return false
}

func (p *parser) tupleBoundAhead() bool {
	offset := 1
	for p.look(offset).Kind == "Identifier" || p.look(offset).Text == "," {
		offset++
	}
	return p.look(offset).Text == ">>" && p.look(offset+1).Text == "\\in"
}

func (p *parser) pattern() (Bound, error) {
	if !p.accept("<<") {
		name, err := p.name()
		return Bound{Names: []string{name}}, err
	}
	names, err := p.names()
	if err != nil {
		return Bound{}, err
	}
	if err = p.expect(">>"); err != nil {
		return Bound{}, err
	}
	return Bound{Names: names, Tuple: true}, nil
}

func (p *parser) boundGroup() ([]Bound, error) {
	var patterns []Bound
	for {
		pattern, err := p.pattern()
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, pattern)
		if !p.accept(",") {
			break
		}
	}
	if err := p.expect("\\in"); err != nil {
		return nil, err
	}
	set, err := p.expression(0)
	if err != nil {
		return nil, err
	}
	for i := range patterns {
		patterns[i].Set = set
		if len(patterns) > 1 {
			patterns[i].Group = set
		}
	}
	return patterns, nil
}

func (p *parser) bounds() ([]Bound, error) {
	var result []Bound
	for {
		group, err := p.boundGroup()
		if err != nil {
			return nil, err
		}
		result = append(result, group...)
		if !p.accept(",") {
			return result, nil
		}
	}
}

func (p *parser) quantifier() (*Expression, error) {
	start := p.pos
	t := p.next()
	temporal := t.Text == "\\AA" || t.Text == "\\EE"
	if !temporal && p.boundAhead() {
		return p.boundedQuantifier(start, t)
	}
	names, err := p.names()
	if err != nil {
		return nil, err
	}
	if err = p.expect(":"); err != nil {
		return nil, err
	}
	body, err := p.expression(0)
	if err != nil {
		return nil, err
	}
	result := node("UnboundedQuantifier", "", p.location(t, start), body)
	result.Quantifier = quantifierKind(t.Text)
	result.Temporal = temporal
	result.Names = names
	return result, nil
}

func quantifierKind(text string) string {
	if text == "\\A" || text == "\\AA" {
		return "Forall"
	}
	return "Exists"
}

func (p *parser) boundedQuantifier(start int, t syntax.Token) (*Expression, error) {
	bounds, err := p.bounds()
	if err != nil {
		return nil, err
	}
	if err = p.expect(":"); err != nil {
		return nil, err
	}
	body, err := p.expression(0)
	if err != nil {
		return nil, err
	}
	result := node(quantifierKind(t.Text), "", p.location(t, start), body)
	result.Bounds = bounds
	return result, nil
}

func (p *parser) choose() (*Expression, error) {
	start := p.pos
	t := p.next()
	pattern, err := p.pattern()
	if err != nil {
		return nil, err
	}
	if p.accept("\\in") {
		pattern.Set, err = p.expression(0)
		if err != nil {
			return nil, err
		}
	}
	if err = p.expect(":"); err != nil {
		return nil, err
	}
	body, err := p.expression(0)
	if err != nil {
		return nil, err
	}
	result := node("Choose", "", p.location(t, start), body)
	result.Bounds = []Bound{pattern}
	return result, nil
}
