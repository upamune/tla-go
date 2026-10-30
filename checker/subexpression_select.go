package checker

import (
	"slices"
	"strconv"
	"strings"

	"github.com/upamune/tla-go/syntax/parser"
)

type subexpressionSelection struct {
	resolver     *subexpressionResolver
	scope        map[string]parser.Definition
	selectors    []parser.Selector
	pool         []*parser.Expression
	location     parser.Location
	params       []parser.Declaration
	args         []*parser.Expression
	lets         [][]parser.Definition
	current      *parser.Expression
	index        int
	canSelectLet bool
}

func (r *subexpressionResolver) selectExpression(selectors []parser.Selector, pool []*parser.Expression, scope map[string]parser.Definition, location parser.Location) (*parser.Expression, error) {
	s := subexpressionSelection{resolver: r, scope: scope, selectors: selectors, pool: append([]*parser.Expression(nil), pool...), location: location}
	definition, args, err := s.findDefinition(scope)
	if err != nil {
		return nil, err
	}
	if err = s.enter(definition, args); err != nil {
		return nil, err
	}
	for s.index < len(s.selectors) {
		selector := s.selectors[s.index]
		if selector.Kind == "Name" {
			err = s.selectName(selector)
		} else {
			err = s.selectOperand(selector)
			s.canSelectLet = true
		}
		if err != nil {
			return nil, err
		}
		s.index++
	}
	if len(s.pool) != 0 {
		return nil, selectionError(location, "Too many arguments for subexpression")
	}
	return s.application()
}

func (s *subexpressionSelection) findDefinition(scope map[string]parser.Definition) (parser.Definition, []*parser.Expression, error) {
	var parts []string
	var args []*parser.Expression
	for s.index < len(s.selectors) {
		selector := s.selectors[s.index]
		if selector.Kind != "Name" {
			break
		}
		parts = append(parts, selector.Name)
		args = append(args, selector.Args...)
		if definition, ok := scope[strings.Join(parts, "!")]; ok {
			s.index++
			return definition, s.takeArguments(args, len(definition.Params)), nil
		}
		s.index++
	}
	return parser.Definition{}, nil, selectionError(s.location, "Unknown operator %s", strings.Join(parts, "!"))
}

func (s *subexpressionSelection) takeArguments(args []*parser.Expression, count int) []*parser.Expression {
	if len(args) != 0 || len(s.pool) == 0 {
		return args
	}
	if count > len(s.pool) {
		count = len(s.pool)
	}
	args, s.pool = s.pool[:count], s.pool[count:]
	return args
}

func (s *subexpressionSelection) enter(definition parser.Definition, args []*parser.Expression) error {
	if definition.Kind != "Operator" {
		return selectionError(s.location, "Cannot select a subexpression of %s", definition.Name)
	}
	if len(args) != 0 && len(args) != len(definition.Params) {
		return selectionError(s.location, "The operator %s requires %d arguments", definition.Name, len(definition.Params))
	}
	s.params = append(s.params, definition.Params...)
	s.args = append(s.args, args...)
	s.current = definition.Body
	s.canSelectLet = false
	return nil
}

func (s *subexpressionSelection) selectName(selector parser.Selector) error {
	if s.canSelectLet {
		return s.selectLetName(selector)
	}
	found, lets := findSubexpressionLabel(s.current, selector.Name)
	if found != nil {
		s.lets = append(s.lets, lets...)
		for _, name := range found.Names {
			s.params = append(s.params, parser.Declaration{Name: name})
		}
		s.args = append(s.args, s.takeArguments(selector.Args, len(found.Names))...)
		s.current = found.Args[0]
		return nil
	}
	return selectionError(s.location, "Cannot find label %s", selector.Name)
}

func (s *subexpressionSelection) selectLetName(selector parser.Selector) error {
	for s.current != nil && s.current.Kind == "Label" {
		s.current = s.current.Args[0]
	}
	if s.current == nil || s.current.Kind != "Let" {
		return selectionError(s.location, "A name selector here must be from a LET clause")
	}
	s.lets = append(s.lets, s.current.Definitions)
	for _, definition := range s.current.Definitions {
		if definition.Name == selector.Name {
			return s.enter(definition, s.takeArguments(selector.Args, len(definition.Params)))
		}
	}
	return selectionError(s.location, "Unknown operator %s", selector.Name)
}

func findSubexpressionLabel(expr *parser.Expression, name string) (*parser.Expression, [][]parser.Definition) {
	if expr == nil {
		return nil, nil
	}
	if expr.Kind == "Label" {
		if expr.Text == name {
			return expr, nil
		}
		return nil, nil
	}
	if expr.Kind == "Lambda" {
		return nil, nil
	}
	if expr.Kind == "Let" {
		label, lets := findSubexpressionLabel(expr.Args[0], name)
		if label != nil {
			return label, append([][]parser.Definition{expr.Definitions}, lets...)
		}
		return nil, nil
	}
	var label *parser.Expression
	var lets [][]parser.Definition
	walkChildren(expr, func(child *parser.Expression) {
		if label == nil {
			label, lets = findSubexpressionLabel(child, name)
		}
	})
	return label, lets
}

func (s *subexpressionSelection) application() (*parser.Expression, error) {
	if s.current == nil {
		return nil, selectionError(s.location, "Subexpression selection failed")
	}
	body := s.current
	for _, v := range slices.Backward(s.lets) {
		body = &parser.Expression{Kind: "Let", Definitions: v, Args: []*parser.Expression{body}, Location: s.location}
	}
	if len(s.params) == 0 {
		return body, nil
	}
	if len(s.args) == 0 {
		return s.lambda(body)
	}
	if len(s.args) != len(s.params) {
		return nil, selectionError(s.location, "The subexpression requires %d arguments but received %d", len(s.params), len(s.args))
	}
	s.resolver.next++
	name := "!subexpression!" + strconv.Itoa(s.resolver.next)
	d := parser.Definition{Kind: "Operator", Name: name, Params: s.params, Body: body, Local: true, Location: s.location}
	d = (&alphaRenamer{prefix: "selection"}).definition(d, nil)
	call := &parser.Expression{Kind: "Apply", Text: name, Args: s.args, Location: s.location}
	return &parser.Expression{Kind: "Let", Definitions: []parser.Definition{d}, Args: []*parser.Expression{call}, Location: s.location}, nil
}

func (s *subexpressionSelection) lambda(body *parser.Expression) (*parser.Expression, error) {
	var names []string
	for _, param := range s.params {
		if param.Arity != 0 {
			return nil, selectionError(s.location, "Selected an operator with operator parameters as an argument")
		}
		names = append(names, param.Name)
	}
	return &parser.Expression{Kind: "Lambda", Names: names, Args: []*parser.Expression{body}, Location: s.location}, nil
}
