package parser

func (p *parser) pushRecursiveScope()          { p.recursive = append(p.recursive, map[string]bool{}) }
func (p *parser) popRecursiveScope()           { p.recursive = p.recursive[:len(p.recursive)-1] }
func (p *parser) isRecursive(name string) bool { return p.recursive[len(p.recursive)-1][name] }
func (p *parser) declareRecursive() ([]Declaration, error) {
	decls, err := p.declarations(true)
	if err != nil {
		return nil, err
	}
	scope := p.recursive[len(p.recursive)-1]
	for _, d := range decls {
		scope[d.Name] = true
	}
	return decls, nil
}

func (p *parser) isOperatorDeclaration() bool {
	return p.at("_") && p.look(1).Kind == "Operator" || p.peek().Kind == "Operator" && p.look(1).Text == "_"
}

func (p *parser) operatorDeclaration() (Declaration, error) {
	t := p.peek()
	arity := 1
	name := ""
	if p.accept("_") {
		name = p.next().Text
		if p.accept("_") {
			arity = 2
		}
	} else {
		name = p.next().Text
		if name == "-" {
			name = "-."
		}
		if err := p.expect("_"); err != nil {
			return Declaration{}, err
		}
	}
	return Declaration{Name: name, Arity: arity, Location: Location{Position: position(t)}}, nil
}
