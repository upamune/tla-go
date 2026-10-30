package parser

func (p *parser) theoremAssumption() (*Expression, error) {
	if !p.proofDeclarationAhead() {
		return p.expression(0)
	}
	start := p.pos
	token := p.peek()
	p.accept("NEW")
	kind := "CONSTANT"
	if proofDeclarationKind(p.peek().Text) {
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

func (p *parser) proofDeclarationAhead() bool {
	return p.at("NEW") || proofDeclarationKind(p.peek().Text)
}

func proofDeclarationKind(text string) bool {
	switch text {
	case "CONSTANT", "VARIABLE", "STATE", "ACTION", "TEMPORAL":
		return true
	default:
		return false
	}
}
