package parser

func (p *parser) letExpression() (*Expression, error) {
	start := p.pos
	t := p.next()
	p.pushRecursiveScope()
	defer p.popRecursiveScope()
	definitions, instances, err := p.letUnits()
	if err != nil {
		return nil, err
	}
	body, err := p.expression(0)
	if err != nil {
		return nil, err
	}
	result := node("Let", "", p.location(t, start), body)
	result.Definitions = definitions
	result.Instances = instances
	return result, nil
}

type letDefinitions struct {
	definitions []Definition
	instances   []Instance
}

func (u *letDefinitions) add(d Definition, inst *Instance) {
	if inst != nil {
		u.instances = append(u.instances, *inst)
	} else {
		u.definitions = append(u.definitions, d)
	}
}

func (p *parser) letUnits() ([]Definition, []Instance, error) {
	var result letDefinitions
	for !p.accept("IN") {
		if p.accept("RECURSIVE") {
			if err := p.letRecursive(); err != nil {
				return nil, nil, err
			}
			continue
		}
		d, inst, err := p.definition(false)
		if err != nil {
			return nil, nil, err
		}
		result.add(d, inst)
	}
	return result.definitions, result.instances, nil
}
func (p *parser) letRecursive() error { _, err := p.declareRecursive(); return err }
func (p *parser) fairness() (*Expression, error) {
	start := p.pos
	t := p.next()
	subscript, err := p.subscript()
	if err != nil {
		return nil, err
	}
	previous := p.offside
	p.offside = 0
	defer func() { p.offside = previous }()
	if err = p.expect("("); err != nil {
		return nil, err
	}
	action, err := p.expression(0)
	if err != nil {
		return nil, err
	}
	if err = p.expect(")"); err != nil {
		return nil, err
	}
	kind := "WeakFairness"
	if t.Text == "SF_" {
		kind = "StrongFairness"
	}
	return node(kind, "", p.location(t, start), subscript, action), nil
}

func (p *parser) caseExpression() (*Expression, error) {
	start := p.pos
	t := p.next()
	arms, other, err := p.caseBranches()
	if err != nil {
		return nil, err
	}
	var result *Expression
	if other != nil {
		result = node("Case", "", p.location(t, start), other)
	} else {
		result = node("Case", "", p.location(t, start))
	}
	result.Arms = arms
	return result, nil
}

func (p *parser) caseBranches() ([]CaseArm, *Expression, error) {
	var arms []CaseArm
	for {
		if p.accept("OTHER") {
			if err := p.expect("->"); err != nil {
				return nil, nil, err
			}
			other, err := p.expression(0)
			return arms, other, err
		}
		arm, err := p.caseArm()
		if err != nil {
			return nil, nil, err
		}
		arms = append(arms, arm)
		if !p.accept("[]") {
			return arms, nil, nil
		}
	}
}

func (p *parser) caseArm() (CaseArm, error) {
	condition, err := p.expression(0)
	if err != nil {
		return CaseArm{}, err
	}
	if err = p.expect("->"); err != nil {
		return CaseArm{}, err
	}
	body, err := p.expression(0)
	return CaseArm{condition, body}, err
}

func (p *parser) lambda() (*Expression, error) {
	start := p.pos
	t := p.next()
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
	result := node("Lambda", "", p.location(t, start), body)
	result.Names = names
	return result, nil
}
