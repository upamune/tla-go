package checker

import (
	"fmt"
	"maps"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

type ltlKind uint8

const (
	ltlAtom ltlKind = iota
	ltlAnd
	ltlOr
	ltlAlways
	ltlEventually
	ltlAction
	ltlTrue
	ltlFalse
)

type ltlFormula struct {
	id          int
	kind        ltlKind
	left        *ltlFormula
	right       *ltlFormula
	atom        *parser.Expression
	negative    bool
	locals      map[string]value.Value
	fairness    bool
	fairEnabled bool
	baseLevel   bool
	// leaf identifies an atom regardless of its polarity, like the shared
	// LiveExprNode of a TLC IF guard.
	leaf int
}

type ltlKey struct {
	kind        ltlKind
	left, right int
	atom        *parser.Expression
	negative    bool
	scope       int
}

type ltlCompiler struct {
	model       *model
	formulas    map[ltlKey]*ltlFormula
	leaves      map[ltlKey]int
	eventuality []*ltlFormula
	nextID      int
	locals      map[string]value.Value
	scope       int
	nextScope   int
	renamer     alphaRenamer
	lexical     [][]parser.Definition
	ownsDefs    bool
}

func newLTLCompiler(m *model, locals map[string]value.Value) *ltlCompiler {
	return &ltlCompiler{model: m, formulas: map[ltlKey]*ltlFormula{}, leaves: map[ltlKey]int{}, locals: maps.Clone(locals), renamer: alphaRenamer{prefix: "ltl", sequential: true}}
}

func (c *ltlCompiler) intern(kind ltlKind, left, right *ltlFormula, atom *parser.Expression, negative bool) *ltlFormula {
	if formula := simplifyLTLJunction(kind, left, right); formula != nil {
		return formula
	}
	key := c.formulaKey(kind, left, right, atom, negative)
	if existing := c.formulas[key]; existing != nil {
		return existing
	}
	return c.newFormula(key, left, right)
}

func simplifyLTLJunction(kind ltlKind, left, right *ltlFormula) *ltlFormula {
	if !isLTLJunction(kind) || left == nil || right == nil {
		return nil
	}
	if left.kind == ltlTrue || left.kind == ltlFalse {
		return simplifyLTLConstant(kind, left, right)
	}
	if right.kind == ltlTrue || right.kind == ltlFalse {
		return simplifyLTLConstant(kind, right, left)
	}
	return nil
}

func isLTLJunction(kind ltlKind) bool { return kind == ltlAnd || kind == ltlOr }

func simplifyLTLConstant(kind ltlKind, constant, other *ltlFormula) *ltlFormula {
	if (kind == ltlAnd) == (constant.kind == ltlFalse) {
		return constant
	}
	return other
}

func (c *ltlCompiler) formulaKey(kind ltlKind, left, right *ltlFormula, atom *parser.Expression, negative bool) ltlKey {
	key := ltlKey{kind: kind, atom: atom, negative: negative}
	if kind == ltlAtom || kind == ltlAction {
		key.scope = c.scope
	}
	if left != nil {
		key.left = left.id
	}
	if right != nil {
		key.right = right.id
	}
	return key
}

func (c *ltlCompiler) newFormula(key ltlKey, left, right *ltlFormula) *ltlFormula {
	c.nextID++
	formula := &ltlFormula{id: c.nextID, kind: key.kind, left: left, right: right, atom: key.atom, negative: key.negative}
	if key.kind == ltlAtom || key.kind == ltlAction {
		formula.locals = maps.Clone(c.locals)
		formula.atom = c.wrapLexical(formula.atom)
		formula.leaf = c.leafID(key)
	}
	c.formulas[key] = formula
	if key.kind == ltlEventually {
		c.eventuality = append(c.eventuality, formula)
	}
	return formula
}

func (c *ltlCompiler) leafID(key ltlKey) int {
	key.negative = false
	if id, ok := c.leaves[key]; ok {
		return id
	}
	id := len(c.leaves) + 1
	c.leaves[key] = id
	return id
}

func (c *ltlCompiler) compile(expr *parser.Expression, negative bool, seen map[string]bool) (*ltlFormula, error) {
	if literal := c.compileLiteral(expr, negative); literal != nil {
		return literal, nil
	}
	if expanded := c.inlinePureLocal(expr); expanded != nil {
		return c.compile(expanded, negative, seen)
	}
	if !c.model.hasTemporal(expr, map[string]bool{}) {
		if c.model.hasAction(expr, map[string]bool{}) {
			return nil, fmt.Errorf("temporal formula contains an action outside a temporal action operator")
		}
		return c.intern(ltlAtom, nil, nil, expr, negative), nil
	}
	if expr.Kind == "Identifier" || expr.Kind == "Apply" {
		if def, ok := c.model.defs[expr.Text]; ok {
			return c.compileDefinition(expr, def, negative, seen)
		}
	}
	return c.compileTemporal(expr, negative, seen)
}

func (c *ltlCompiler) compileLiteral(expr *parser.Expression, negative bool) *ltlFormula {
	if !isBooleanLiteral(expr, "TRUE") && !isBooleanLiteral(expr, "FALSE") {
		return nil
	}
	kind := ltlFalse
	if (expr.Text == "TRUE") != negative {
		kind = ltlTrue
	}
	return c.intern(kind, nil, nil, nil, false)
}

func (c *ltlCompiler) compileDefinition(expr *parser.Expression, def parser.Definition, negative bool, seen map[string]bool) (*ltlFormula, error) {
	if seen[expr.Text] || def.Kind != "Operator" || len(def.Params) != len(expr.Args) {
		return nil, fmt.Errorf("recursive or invalid temporal operator %s", expr.Text)
	}
	expanded, err := substituteActionCall(def, expr)
	if err != nil {
		return nil, err
	}
	seen[expr.Text] = true
	defer delete(seen, expr.Text)
	return c.compile(expanded, negative, seen)
}
