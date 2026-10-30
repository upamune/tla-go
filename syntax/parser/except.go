package parser

import "github.com/upamune/tla-go/syntax"

func (p *parser) except(start int, t syntax.Token, base *Expression) (*Expression, error) {
	var updates []Update
	for {
		update, err := p.update()
		if err != nil {
			return nil, err
		}
		updates = append(updates, update)
		if !p.accept(",") {
			break
		}
	}
	if err := p.expect("]"); err != nil {
		return nil, err
	}
	result := node("Except", "", p.location(t, start), base)
	result.Updates = updates
	return result, nil
}

func (p *parser) update() (Update, error) {
	if err := p.expect("!"); err != nil {
		return Update{}, err
	}
	path, err := p.updatePath()
	if err != nil {
		return Update{}, err
	}
	if err = p.expect("="); err != nil {
		return Update{}, err
	}
	value, err := p.expression(0)
	return Update{Path: path, Value: value}, err
}

func (p *parser) updatePath() ([]PathStep, error) {
	var path []PathStep
	for p.at(".") || p.at("[") {
		step, err := p.pathStep()
		if err != nil {
			return nil, err
		}
		path = append(path, step)
	}
	return path, nil
}

func (p *parser) pathStep() (PathStep, error) {
	if p.accept(".") {
		field := p.peek()
		if field.Kind != "Identifier" && field.Kind != "Keyword" {
			return PathStep{}, p.error("Expected a field name")
		}
		p.next()
		return PathStep{Kind: "Field", Name: field.Text}, nil
	}
	if err := p.expect("["); err != nil {
		return PathStep{}, err
	}
	args, err := p.delimited("]", false)
	return PathStep{Kind: "Index", Args: args}, err
}
