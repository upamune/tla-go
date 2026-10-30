package parser

import "github.com/upamune/tla-go/syntax"

func (p *parser) byProof(start int, t syntax.Token, keywordStart int) (*Proof, error) {
	result := &Proof{Kind: "By", Keyword: p.span(keywordStart), Only: p.accept("ONLY")}
	var err error
	if p.factAhead() {
		result.Facts, err = p.facts()
		if err != nil {
			return nil, err
		}
	}
	if p.accept("DEF") || p.accept("DEFS") {
		result.Defs, err = p.facts()
		if err != nil {
			return nil, err
		}
		if err = p.validateDefs(result.Defs); err != nil {
			return nil, err
		}
	}
	result.Location = p.location(t, start)
	return result, nil
}

func (p *parser) factAhead() bool {
	t := p.peek()
	if t.Kind == "End" || t.Kind == "ModuleEnd" || t.Kind == "Separator" {
		return false
	}
	if t.Kind == "Keyword" {
		return proofFactKeywords[t.Text]
	}
	if t.Kind == "Identifier" && p.look(1).Text == "==" {
		return false
	}
	return true
}

var proofFactKeywords = map[string]bool{"CASE": true, "CHOOSE": true, "DOMAIN": true, "ENABLED": true, "IF": true, "LAMBDA": true, "LET": true, "SF_": true, "SUBSET": true, "UNCHANGED": true, "UNION": true, "WF_": true, "MODULE": true}

func (p *parser) facts() ([]*Expression, error) {
	var facts []*Expression
	for {
		fact, err := p.fact()
		if err != nil {
			return nil, err
		}
		facts = append(facts, fact)
		if !p.accept(",") {
			return facts, nil
		}
	}
}

func (p *parser) fact() (*Expression, error) {
	t := p.peek()
	start := p.pos
	if p.accept("MODULE") {
		name, err := p.name()
		if err != nil {
			return nil, err
		}
		return node("ModuleFact", name, p.location(t, start)), nil
	}
	_, ok := infix[t.Text]
	if t.Kind == "Operator" && ok && !isPrefixExpression(t.Text) {
		p.next()
		return node("OperatorReference", t.Text, p.location(t, start)), nil
	}
	return p.expression(0)
}

func isPrefixExpression(text string) bool {
	_, _, _, ok := prefixInfo(text)
	return ok || text == "/\\" || text == "\\/"
}

func (p *parser) validateDefs(defs []*Expression) error {
	for _, d := range defs {
		if d.Kind == "Identifier" && p.declared(d.Text) {
			return &syntax.SyntaxError{Reason: "DEF clause entry should describe a defined operator", Line: d.Location.Line, Column: d.Location.Column}
		}
	}
	return nil
}

func (p *parser) declared(name string) bool {
	for _, m := range p.modules {
		if hasDeclaration(m.Constants, name) || hasDeclaration(m.Variables, name) {
			return true
		}
	}
	return false
}

func hasDeclaration(declarations []Declaration, name string) bool {
	for _, d := range declarations {
		if d.Name == name {
			return true
		}
	}
	return false
}
