package parser

import (
	"fmt"
	"strings"

	"github.com/upamune/tla-go/syntax"
)

func (p *parser) identifier() (*Expression, error) {
	start := p.pos
	t := p.next()
	return p.identifierTail(start, t)
}

func (p *parser) identifierTail(start int, t syntax.Token) (*Expression, error) {
	args, hasArgs, err := p.selectorArgs()
	if err != nil {
		return nil, err
	}
	if p.accept("::") {
		return p.label(start, t, args)
	}
	selectors := []Selector{{Kind: "Name", Name: t.Text, Args: args}}
	for p.selectorAhead() {
		p.next()
		selector, err := p.selector()
		if err != nil {
			return nil, err
		}
		selectors = append(selectors, selector)
	}
	if len(selectors) > 1 {
		if err := p.checkInstanceArity(t, len(args)); err != nil {
			return nil, err
		}
	}
	return p.reference(start, t, selectors, hasArgs), nil
}

func (p *parser) label(start int, t syntax.Token, args []*Expression) (*Expression, error) {
	var names []string
	for _, arg := range args {
		if arg.Kind != "Identifier" {
			return nil, p.error("Expected a label parameter")
		}
		names = append(names, arg.Text)
	}
	body, err := p.expression(0)
	if err != nil {
		return nil, err
	}
	result := node("Label", t.Text, p.location(t, start), body)
	result.Names = names
	return result, nil
}

func (p *parser) selectorAhead() bool {
	if !p.at("!") {
		return false
	}
	t := p.look(1)
	if t.Kind == "Identifier" || t.Kind == "Number" {
		return true
	}
	if strings.Contains(" << >> : @ ( ", " "+t.Text+" ") {
		return true
	}
	return selectorOperator(t)
}

func selectorOperator(t syntax.Token) bool {
	_, ok := infix[t.Text]
	return t.Kind == "Operator" && (ok || postfixOperator(t.Text) || t.Text == "~" || t.Text == "-.")
}

func (p *parser) selectorArgs() ([]*Expression, bool, error) {
	if !p.accept("(") {
		return nil, false, nil
	}
	args, err := p.arguments()
	return args, true, err
}

func (p *parser) arguments() ([]*Expression, error) {
	previous := p.offside
	p.offside = 0
	defer func() { p.offside = previous }()
	var args []*Expression
	for {
		arg, err := p.argument()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
		if !p.accept(",") {
			break
		}
	}
	return args, p.expect(")")
}

func (p *parser) selector() (Selector, error) {
	t := p.next()
	if t.Kind == "Number" || t.Text == "<<" || t.Text == ">>" {
		return Selector{Kind: "Operand", Operand: t.Text}, nil
	}
	if t.Text == ":" {
		return Selector{Kind: "Colon"}, nil
	}
	if t.Text == "@" {
		return Selector{Kind: "At"}, nil
	}
	if t.Text == "(" {
		args, err := p.arguments()
		return Selector{Kind: "Bound", Args: args}, err
	}
	args, _, err := p.selectorArgs()
	return Selector{Kind: "Name", Name: t.Text, Args: args}, err
}

func (p *parser) reference(start int, t syntax.Token, selectors []Selector, hasArgs bool) *Expression {
	for _, selector := range selectors {
		if selector.Kind != "Name" {
			result := node("Subexpression", "", p.location(t, start))
			result.Selectors = selectors
			return result
		}
	}
	var names []string
	var args []*Expression
	var counts []int
	for _, selector := range selectors {
		names = append(names, selector.Name)
		args = append(args, selector.Args...)
		counts = append(counts, len(selector.Args))
	}
	kind := "Identifier"
	if len(args) > 0 || len(selectors) == 1 && hasArgs {
		kind = "Apply"
	}
	result := node(kind, strings.Join(names, "!"), p.location(t, start), args...)
	result.ArgumentCounts = qualifiedCounts(counts)
	return result
}

func qualifiedCounts(counts []int) []int {
	for _, count := range counts[:len(counts)-1] {
		if count > 0 {
			return counts
		}
	}
	return nil
}

func (p *parser) checkInstanceArity(t syntax.Token, arity int) error {
	for _, module := range p.modules {
		for _, instance := range module.Instances {
			if instance.Name == t.Text {
				if len(instance.Params) != arity {
					return &syntax.SyntaxError{Reason: fmt.Sprintf("The operator %s requires %d arguments", t.Text, len(instance.Params)), Line: t.Line, Column: t.Column}
				}
				return nil
			}
		}
	}
	return nil
}
