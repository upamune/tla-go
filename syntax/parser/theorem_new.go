package parser

func (p *parser) theoremAssumption() (*Expression, error) {
	if !p.at("NEW") {
		return p.expression(0)
	}
	start := p.pos
	token := p.next()
	kind := "CONSTANT"
	switch p.peek().Text {
	case "CONSTANT", "VARIABLE", "STATE", "ACTION":
		kind = p.next().Text
	}
	decl, err := p.declaration(true)
	if err != nil {
		return nil, err
	}
	var bounds []*Expression
	if p.accept("\\in") {
		if kind != "CONSTANT" || decl.Arity != 0 {
			return nil, p.error("only a nullary constant NEW declaration may be bounded")
		}
		bound, err := p.expression(0)
		if err != nil {
			return nil, err
		}
		bounds = append(bounds, bound)
	}
	result := node("NewDeclaration", kind, p.location(token, start), bounds...)
	result.Names = []string{decl.Name}
	result.ArgumentCounts = []int{decl.Arity}
	return result, nil
}
