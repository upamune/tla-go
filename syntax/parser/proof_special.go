package parser

func isProofSpecialStep(text string) bool {
	switch text {
	case "PICK", "TAKE", "HAVE", "WITNESS", "CASE":
		return true
	}
	return false
}

func (p *parser) proofSpecialStep(step ProofStep) (ProofStep, error) {
	keyword := p.next().Text
	switch keyword {
	case "PICK", "TAKE":
		return p.proofBindingStep(step, keyword)
	case "WITNESS":
		return p.proofWitnessStep(step)
	default:
		step.Kind = "Have"
		if keyword == "CASE" {
			step.Kind = "Case"
		}
		var err error
		step.Statement, err = p.expression(0)
		return step, err
	}
}

func (p *parser) proofWitnessStep(step ProofStep) (ProofStep, error) {
	step.Kind = "Witness"
	for {
		expr, err := p.expression(0)
		if err != nil {
			return ProofStep{}, err
		}
		step.Arguments = append(step.Arguments, expr)
		if !p.accept(",") {
			return step, nil
		}
	}
}

func (p *parser) proofBindingStep(step ProofStep, keyword string) (ProofStep, error) {
	step.Kind = "Take"
	if keyword == "PICK" {
		step.Kind = "Pick"
	}
	var err error
	if p.boundAhead() {
		step.Bounds, err = p.bounds()
	} else {
		var names []string
		names, err = p.names()
		for _, name := range names {
			step.Bounds = append(step.Bounds, Bound{Names: []string{name}})
		}
	}
	if err != nil || keyword == "TAKE" {
		return step, err
	}
	if err := p.expect(":"); err != nil {
		return ProofStep{}, err
	}
	step.Statement, err = p.expression(0)
	return step, err
}
