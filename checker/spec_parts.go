package checker

import (
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

type fairnessCondition struct {
	strong           bool
	recurrence       bool
	recurrenceAction bool
	subscript        *parser.Expression
	action           *parser.Expression
	bounds           []parser.Bound
	locals           map[string]value.Value
}

type specificationParts struct {
	module   *parser.Module
	init     string
	next     string
	fairness []fairnessCondition
	bounds   []parser.Bound
}

func (p *specificationParts) collect(expr *parser.Expression, seen map[string]bool) bool {
	switch expr.Kind {
	case "And":
		return p.conjunction(expr.Args, seen)
	case "Or":
		return p.initialDisjunction(expr)
	case "Identifier":
		return p.identifier(expr.Text, seen)
	case "Apply":
		return p.operatorCall(expr, seen)
	case "Always":
		return p.collectAlways(expr)
	case "WeakFairness", "StrongFairness":
		return p.fairAction(expr)
	case "Forall":
		return p.quantifiedFairness(expr, seen)
	default:
		return false
	}
}

func (p *specificationParts) initialDisjunction(expr *parser.Expression) bool {
	if len(p.bounds) != 0 {
		return false
	}
	analysis := model{module: p.module, defs: make(map[string]parser.Definition, len(p.module.Definitions))}
	for _, definition := range p.module.Definitions {
		analysis.defs[definition.Name] = definition
	}
	if analysis.hasAction(expr, map[string]bool{}) {
		return false
	}
	p.addInitialExpression(expr)
	return true
}

func (p *specificationParts) operatorCall(expr *parser.Expression, seen map[string]bool) bool {
	definition, ok := p.operator(expr.Text, len(expr.Args))
	if ok && p.containsSpecAction(definition.Body, map[string]bool{expr.Text: true}) {
		if seen[expr.Text] {
			return false
		}
		expanded, err := substituteActionCall(definition, expr)
		if err != nil {
			return false
		}
		seen[expr.Text] = true
		defer delete(seen, expr.Text)
		return p.collect(expanded, seen)
	}
	if len(p.bounds) > 0 {
		return false
	}
	p.addInitialExpression(expr)
	return true
}

func (p *specificationParts) operator(name string, arity int) (parser.Definition, bool) {
	for _, definition := range p.module.Definitions {
		if definition.Name == name && definition.Kind == "Operator" && len(definition.Params) == arity {
			return definition, true
		}
	}
	return parser.Definition{}, false
}

func (p *specificationParts) addWrapper(kind string, expr *parser.Expression) string {
	name := "!spec!" + kind
	p.module.Definitions = append(p.module.Definitions, parser.Definition{Kind: "Operator", Name: name, Body: expr, Location: expr.Location})
	return name
}

func (p *specificationParts) conjunction(args []*parser.Expression, seen map[string]bool) bool {
	for _, arg := range args {
		if !p.collect(arg, seen) {
			return false
		}
	}
	return true
}

func (p *specificationParts) identifier(name string, seen map[string]bool) bool {
	if len(p.bounds) > 0 {
		return false
	}
	body := findSpecification(p.module, name)
	if body == nil || seen[name] {
		return false
	}
	if p.containsSpecAction(body, map[string]bool{name: true}) {
		seen[name] = true
		defer delete(seen, name)
		return p.collect(body, seen)
	}
	p.addInitialExpression(&parser.Expression{Kind: "Identifier", Text: name, Location: body.Location})
	return true
}

func (p *specificationParts) addInitialExpression(expr *parser.Expression) {
	if p.init == "" && expr.Kind == "Identifier" && findSpecification(p.module, expr.Text) != nil {
		p.init = expr.Text
		return
	}
	if p.init == "" {
		p.init = p.addWrapper("init", expr)
		return
	}
	p.combineInitial(expr)
}

func (p *specificationParts) combineInitial(expr *parser.Expression) {
	if p.init == "!spec!init" || p.init == "!spec!combined-init" {
		body := findSpecification(p.module, p.init)
		if body.Kind == "And" {
			body.Args = append(body.Args, expr)
			return
		}
	}
	previous := &parser.Expression{Kind: "Identifier", Text: p.init}
	body := &parser.Expression{Kind: "And", Args: []*parser.Expression{previous, expr}}
	p.init = p.addWrapper("combined-init", body)
}

func (p *specificationParts) containsSpecAction(expr *parser.Expression, seen map[string]bool) bool {
	switch expr.Kind {
	case "Always", "WeakFairness", "StrongFairness":
		return true
	case "And":
		return p.conjunctionHasSpecAction(expr.Args, seen)
	case "Forall":
		return len(expr.Args) == 1 && p.containsSpecAction(expr.Args[0], seen)
	case "Identifier":
		return p.identifierHasSpecAction(expr.Text, seen)
	case "Apply":
		return p.callHasSpecAction(expr, seen)
	}
	return false
}

func (p *specificationParts) conjunctionHasSpecAction(args []*parser.Expression, seen map[string]bool) bool {
	for _, arg := range args {
		if p.containsSpecAction(arg, seen) {
			return true
		}
	}
	return false
}

func (p *specificationParts) callHasSpecAction(expr *parser.Expression, seen map[string]bool) bool {
	if seen[expr.Text] {
		return false
	}
	definition, ok := p.operator(expr.Text, len(expr.Args))
	if !ok {
		return false
	}
	seen[expr.Text] = true
	defer delete(seen, expr.Text)
	return p.containsSpecAction(definition.Body, seen)
}

func (p *specificationParts) identifierHasSpecAction(name string, seen map[string]bool) bool {
	if seen[name] {
		return false
	}
	body := findSpecification(p.module, name)
	if body == nil {
		return false
	}
	seen[name] = true
	defer delete(seen, name)
	return p.containsSpecAction(body, seen)
}

func (p *specificationParts) nextAction(expr *parser.Expression) bool {
	if len(p.bounds) > 0 {
		return false
	}
	if p.next != "" || len(expr.Args) != 1 || expr.Args[0].Kind != "BoxAction" || len(expr.Args[0].Args) != 2 {
		return false
	}
	box := expr.Args[0]
	if !validSubscript(p.module, box.Args[1]) {
		return false
	}
	p.next = p.nextOperator(box.Args[0])
	return true
}

func (p *specificationParts) nextOperator(expr *parser.Expression) string {
	if expr.Kind == "Identifier" && findSpecification(p.module, expr.Text) != nil {
		return expr.Text
	}
	return p.addWrapper("next", expr)
}

func (p *specificationParts) fairAction(expr *parser.Expression) bool {
	if len(expr.Args) != 2 || !validSubscript(p.module, expr.Args[0]) {
		return false
	}
	p.fairness = append(p.fairness, fairnessCondition{strong: expr.Kind == "StrongFairness", subscript: expr.Args[0], action: expr.Args[1], bounds: append([]parser.Bound(nil), p.bounds...)})
	return true
}

func (p *specificationParts) quantifiedFairness(expr *parser.Expression, seen map[string]bool) bool {
	if len(expr.Args) != 1 || len(expr.Bounds) == 0 {
		return false
	}
	previous := p.bounds
	p.bounds = append(append([]parser.Bound(nil), previous...), expr.Bounds...)
	defer func() { p.bounds = previous }()
	return p.collect(expr.Args[0], seen)
}
