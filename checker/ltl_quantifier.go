package checker

import (
	"fmt"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func (c *ltlCompiler) compileQuantifier(expr *parser.Expression, negative bool, seen map[string]bool) (*ltlFormula, error) {
	if len(expr.Args) != 1 {
		return nil, fmt.Errorf("invalid temporal quantifier")
	}
	if err := c.validateBounds(expr.Bounds); err != nil {
		return nil, err
	}
	kind := ltlAnd
	if (expr.Kind == "Exists") != negative {
		kind = ltlOr
	}
	var result *ltlFormula
	bounds := append([]parser.Bound(nil), expr.Bounds...)
	for i := range bounds {
		bounds[i].Set = c.wrapLexical(bounds[i].Set)
	}
	err := c.model.expandBindings(bounds, c.locals, func(locals map[string]value.Value) error {
		body, err := c.compileWithLocals(expr.Args[0], negative, seen, locals)
		if err != nil {
			return err
		}
		if result == nil {
			result = body
		} else {
			result = c.intern(kind, result, body, nil, false)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		constant := ltlTrue
		if kind == ltlOr {
			constant = ltlFalse
		}
		result = c.intern(constant, nil, nil, nil, false)
	}
	return result, nil
}

func (c *ltlCompiler) validateBounds(bounds []parser.Bound) error {
	for _, bound := range bounds {
		if bound.Set == nil || c.model.hasAction(bound.Set, map[string]bool{}) || c.model.readsState(bound.Set, map[string]bool{}) {
			return fmt.Errorf("temporal quantifier domain must be a constant set")
		}
	}
	return nil
}

func (c *ltlCompiler) compileWithLocals(expr *parser.Expression, negative bool, seen map[string]bool, locals map[string]value.Value) (*ltlFormula, error) {
	previous, scope := c.locals, c.scope
	c.nextScope++
	c.locals, c.scope = locals, c.nextScope
	defer func() { c.locals, c.scope = previous, scope }()
	return c.compile(expr, negative, seen)
}
