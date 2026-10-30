package parser

import (
	"strings"

	"github.com/upamune/tla-go/syntax"
)

func (p *parser) bracket() (*Expression, error) {
	start := p.pos
	t := p.next()
	previous := p.offside
	p.offside = 0
	defer func() { p.offside = previous }()
	if p.peek().Kind == "Identifier" && (p.look(1).Text == "|->" || p.look(1).Text == ":") {
		return p.record(start, t)
	}
	if p.boundAhead() {
		return p.functionConstruction(start, t)
	}
	left, err := p.expression(0)
	if err != nil {
		return nil, err
	}
	if p.accept("->") {
		return p.functionSet(start, t, left)
	}
	if p.accept("EXCEPT") {
		return p.except(start, t, left)
	}
	return p.boxAction(start, t, left)
}

func (p *parser) boxAction(start int, t syntax.Token, left *Expression) (*Expression, error) {
	if err := p.expect("]_"); err != nil {
		return nil, err
	}
	subscript, err := p.subscript()
	if err != nil {
		return nil, err
	}
	return node("BoxAction", "", p.location(t, start), left, subscript), nil
}

func (p *parser) record(start int, t syntax.Token) (*Expression, error) {
	kind, delimiter := "Record", "|->"
	if p.look(1).Text == ":" {
		kind, delimiter = "RecordSet", ":"
	}
	var fields []Field
	var duplicate *Position
	seen := make(map[string]bool)
	for {
		fieldPosition := position(p.peek())
		field, err := p.recordField(delimiter)
		if err != nil {
			return nil, err
		}
		if seen[field.Name] && duplicate == nil {
			duplicate = &fieldPosition
		}
		seen[field.Name] = true
		fields = append(fields, field)
		if !p.accept(",") {
			break
		}
	}
	if err := p.expect("]"); err != nil {
		return nil, err
	}
	result := node(kind, "", p.location(t, start))
	result.Fields = fields
	result.DuplicateField = duplicate
	return result, nil
}

func (p *parser) recordField(delimiter string) (Field, error) {
	name, err := p.name()
	if err != nil {
		return Field{}, err
	}
	if err = p.expect(delimiter); err != nil {
		return Field{}, err
	}
	value, err := p.expression(0)
	return Field{name, value}, err
}

func (p *parser) functionConstruction(start int, t syntax.Token) (*Expression, error) {
	bounds, err := p.bounds()
	if err != nil {
		return nil, err
	}
	if err = p.expect("|->"); err != nil {
		return nil, err
	}
	body, err := p.expression(0)
	if err != nil {
		return nil, err
	}
	if err = p.expect("]"); err != nil {
		return nil, err
	}
	result := node("FunctionConstruction", "", p.location(t, start), body)
	result.Bounds = bounds
	return result, nil
}

func (p *parser) functionSet(start int, t syntax.Token, left *Expression) (*Expression, error) {
	right, err := p.expression(0)
	if err != nil {
		return nil, err
	}
	if err = p.expect("]"); err != nil {
		return nil, err
	}
	return node("FunctionSet", "", p.location(t, start), left, right), nil
}

func (p *parser) subscript() (*Expression, error) {
	if p.peek().Kind == "Identifier" {
		start := p.pos
		t := p.next()
		var name strings.Builder
		name.WriteString(t.Text)
		for p.accept("!") {
			part, err := p.name()
			if err != nil {
				return nil, err
			}
			name.WriteString("!" + part)
		}
		return node("Identifier", name.String(), p.location(t, start)), nil
	}
	if p.at("<<") {
		expr, err := p.primary()
		if err == nil {
			expr.Location.Range = Range{}
		}
		return expr, err
	}
	if p.at("(") {
		return p.primary()
	}
	return nil, p.error("Expected a subscript")
}
