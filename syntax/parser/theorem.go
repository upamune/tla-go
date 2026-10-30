package parser

func isTheorem(text string) bool {
	return text == "THEOREM" || text == "LEMMA" || text == "COROLLARY" || text == "PROPOSITION"
}

func (p *parser) theorem(m *Module) error {
	start := p.pos
	t := p.next()
	name := ""
	if p.peek().Kind == "Identifier" && p.look(1).Text == "==" {
		name = p.next().Text
		p.next()
	}
	statement, err := p.theoremStatement()
	if err != nil {
		return err
	}
	proof, err := p.simpleProof()
	if err != nil {
		return err
	}
	location := p.location(t, start)
	m.Theorems = append(m.Theorems, Theorem{Name: name, Statement: statement, Proof: proof, Location: location})
	if name != "" {
		m.Definitions = append(m.Definitions, Definition{Kind: "Operator", Name: name, Body: statement, Location: location})
	}
	return nil
}

func (p *parser) theoremStatement() (*Expression, error) {
	if !p.at("ASSUME") {
		return p.expression(0)
	}
	start := p.pos
	token := p.next()
	var clauses []*Expression
	for {
		clause, err := p.theoremAssumption()
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, clause)
		if !p.accept(",") {
			break
		}
	}
	if err := p.expect("PROVE"); err != nil {
		return nil, err
	}
	conclusion, err := p.expression(0)
	if err != nil {
		return nil, err
	}
	return node("AssumeProve", "", p.location(token, start), append(clauses, conclusion)...), nil
}

func (p *parser) simpleProof() (*Proof, error) {
	if !p.proofAhead() {
		return nil, nil
	}
	start := p.pos
	t := p.peek()
	p.accept("PROOF")
	keywordStart := p.pos
	if p.accept("BY") {
		return p.byProof(start, t, keywordStart)
	}
	kind := ""
	if p.accept("OBVIOUS") {
		kind = "Obvious"
	} else if p.accept("OMITTED") {
		kind = "Omitted"
	} else {
		return nil, p.error("Unsupported proof form")
	}
	return &Proof{Kind: kind, Keyword: p.span(keywordStart), Location: p.location(t, start)}, nil
}

func (p *parser) proofAhead() bool {
	t := p.peek()
	return t.Kind == "Step" || t.Text == "PROOF" || t.Text == "BY" || t.Text == "OBVIOUS" || t.Text == "OMITTED"
}
