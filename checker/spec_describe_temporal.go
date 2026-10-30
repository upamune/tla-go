package checker

import (
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func (d *specDescription) collectSpec(expr *parser.Expression, seen map[string]bool) error {
	if expr.Kind == "And" {
		return d.collectSpecConjunction(expr.Args, seen)
	}
	if definition, ok := d.specDefinition(expr, seen); ok {
		return d.collectSpecDefinition(expr, definition, seen)
	}
	if isAlwaysBox(expr) {
		return d.collectActions(expr.Args[0].Args[0], actionDescriptionScope{}, nil, map[string]bool{})
	}
	category := "inits"
	if d.model.hasTemporal(expr, map[string]bool{}) {
		category = "temporals"
	}
	return d.add(category, describedAction{name: "UnnamedAction", location: expr.Location.Range})
}

func (d *specDescription) collectSpecConjunction(args []*parser.Expression, seen map[string]bool) error {
	for _, arg := range args {
		if err := d.collectSpec(arg, seen); err != nil {
			return err
		}
	}
	return nil
}

func (d *specDescription) collectSpecDefinition(expr *parser.Expression, definition parser.Definition, seen map[string]bool) error {
	if d.isConstantDefinition(definition) {
		return nil
	}
	if len(definition.Params) == 0 && !d.model.hasTemporal(definition.Body, map[string]bool{}) && !d.model.hasAction(definition.Body, map[string]bool{}) {
		return d.add("inits", describedAction{name: definition.Name, location: definition.Body.Location.Range})
	}
	body, err := substituteActionCall(definition, expr)
	if err != nil {
		return err
	}
	seen[definition.Name] = true
	defer delete(seen, definition.Name)
	return d.collectSpec(body, seen)
}

func (d *specDescription) specDefinition(expr *parser.Expression, seen map[string]bool) (parser.Definition, bool) {
	if (expr.Kind != "Identifier" && expr.Kind != "Apply") || seen[expr.Text] {
		return parser.Definition{}, false
	}
	definition, ok := d.model.defs[expr.Text]
	return definition, ok && definition.Kind == "Operator" && len(definition.Params) == len(expr.Args)
}

func (d *specDescription) collectProperties(invariants, properties []string) error {
	if err := d.collectInvariantDefinitions(invariants); err != nil {
		return err
	}
	for _, name := range properties {
		definition, ok := d.model.defs[name]
		if !ok || d.isConstantDefinition(definition) {
			continue
		}
		if err := d.collectProperty(name, definition.Body, nil, map[string]bool{}); err != nil {
			return err
		}
	}
	return nil
}

func (d *specDescription) collectInvariantDefinitions(names []string) error {
	for _, name := range names {
		definition, ok := d.model.defs[name]
		if !ok || d.isConstantDefinition(definition) {
			continue
		}
		if err := d.add("invariants", describedAction{name: name, location: definition.Body.Location.Range}); err != nil {
			return err
		}
	}
	return nil
}

func (d *specDescription) collectProperty(name string, expr *parser.Expression, locals map[string]value.Value, seen map[string]bool) error {
	if expr.Kind == "And" {
		return d.collectPropertyConjunction(name, expr.Args, locals, seen)
	}
	if expr.Kind == "Forall" && len(expr.Args) == 1 {
		return d.collectQuantifiedProperty(name, expr, locals, seen)
	}
	if definition, ok := d.specDefinition(expr, seen); ok {
		return d.collectPropertyDefinition(name, expr, definition, locals, seen)
	}
	category, target := d.propertyCategory(expr)
	return d.add(category, describedAction{name: name, location: target.Location.Range})
}

func (d *specDescription) collectPropertyConjunction(name string, args []*parser.Expression, locals map[string]value.Value, seen map[string]bool) error {
	for _, arg := range args {
		if err := d.collectProperty(name, arg, locals, seen); err != nil {
			return err
		}
	}
	return nil
}

func (d *specDescription) collectPropertyDefinition(name string, expr *parser.Expression, definition parser.Definition, locals map[string]value.Value, seen map[string]bool) error {
	if d.isConstantDefinition(definition) {
		return nil
	}
	body, err := substituteActionCall(definition, expr)
	if err != nil {
		return err
	}
	seen[definition.Name] = true
	defer delete(seen, definition.Name)
	return d.collectProperty(name, body, locals, seen)
}

func (d *specDescription) collectQuantifiedProperty(name string, expr *parser.Expression, locals map[string]value.Value, seen map[string]bool) error {
	return d.model.expandBindings(expr.Bounds, locals, func(bound map[string]value.Value) error {
		return d.collectProperty(name, expr.Args[0], bound, seen)
	})
}

func (d *specDescription) propertyCategory(expr *parser.Expression) (string, *parser.Expression) {
	if expr.Kind == "Always" && len(expr.Args) == 1 {
		inner := expr.Args[0]
		if inner.Kind == "BoxAction" {
			return "impliedactions", inner
		}
		if !d.model.hasAction(inner, map[string]bool{}) && !d.model.hasTemporal(inner, map[string]bool{}) {
			return "invariants", inner
		}
		return "impliedtemporals", expr
	}
	if d.model.hasTemporal(expr, map[string]bool{}) {
		return "impliedtemporals", expr
	}
	return "impliedinits", expr
}
