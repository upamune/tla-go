package parser

func isProofDirective(text string) bool { return text == "USE" || text == "HIDE" }

func (p *parser) proofDirective(module *Module) error {
	start := p.pos
	token := p.next()
	directive := ProofDirective{Kind: token.Text}
	if p.factAhead() {
		facts, err := p.facts()
		if err != nil {
			return err
		}
		directive.Facts = facts
	}
	defs, err := p.directiveDefinitions()
	if err != nil {
		return err
	}
	directive.Defs = defs
	if len(directive.Facts) == 0 && len(directive.Defs) == 0 {
		return p.error("Expected facts or definitions")
	}
	directive.Location = p.location(token, start)
	module.ProofDirectives = append(module.ProofDirectives, directive)
	return nil
}

func (p *parser) directiveDefinitions() ([]*Expression, error) {
	if !p.accept("DEF") && !p.accept("DEFS") {
		return nil, nil
	}
	if !p.factAhead() {
		return nil, p.error("Expected a definition after DEF")
	}
	defs, err := p.facts()
	if err != nil {
		return nil, err
	}
	return defs, p.validateDefs(defs)
}
