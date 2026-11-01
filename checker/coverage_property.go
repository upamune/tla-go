package checker

import (
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

// coverageNode mirrors the subset of TLC's OpApplNode cost tree used for
// state predicates. Numeric literals are not OpApplNodes in SANY.
type coverageNode struct {
	expr     *parser.Expression
	count    int
	children []*coverageNode
}

func (m *model) addCoverageProperty(c *coverageCollector, name string) bool {
	d, ok := m.defs[name]
	if !ok || !m.coveragePropertyDefinitionSupported(d) {
		return false
	}
	if _, exists := c.byName[name]; exists {
		return false
	}
	vars := make(map[string]bool, len(m.names))
	for _, n := range m.names {
		vars[n] = true
	}
	seenState := false
	root, ok := m.propertyCoverageNode(d.Body, vars, map[string]bool{}, &seenState)
	if !ok || root == nil || !seenState {
		return false
	}
	r := d.Location.Range
	r.End = r.Start
	r.End.Column += len(name) - 1
	c.byName[name] = len(c.actions)
	c.actions = append(c.actions, CoverageAction{Name: name, Range: r, Property: true})
	c.properties[name] = root
	return true
}

func (m *model) coveragePropertyDefinitionSupported(d parser.Definition) bool {
	return len(d.Params) == 0 && !d.InstanceSubstituted && !m.trueConfigured[d.Name]
}

type propertyBuilder struct {
	model       *model
	vars, bound map[string]bool
	seen        *bool
}

func (m *model) propertyCoverageNode(e *parser.Expression, vars, bound map[string]bool, seen *bool) (*coverageNode, bool) {
	return (&propertyBuilder{model: m, vars: vars, bound: bound, seen: seen}).node(e)
}

func (b *propertyBuilder) node(e *parser.Expression) (*coverageNode, bool) {
	if e == nil {
		return nil, false
	}
	if e.Kind == "Number" {
		return nil, true
	}
	n := &coverageNode{expr: e}
	switch e.Kind {
	case "Identifier":
		return n, b.identifier(e.Text)
	case "Apply":
		return n, b.apply(n)
	case "And", "Or":
		if !e.Bulleted && len(e.Args) != 2 {
			return nil, false
		}
		return n, b.children(n, e.Args)
	}
	return b.boundNode(n)
}

func (b *propertyBuilder) boundNode(n *coverageNode) (*coverageNode, bool) {
	if n.expr.Kind == "Forall" || n.expr.Kind == "Exists" {
		return n, b.quantifier(n)
	}
	return nil, false
}

func (b *propertyBuilder) identifier(name string) bool {
	if b.vars[name] {
		*b.seen = true
		return true
	}
	if b.bound[name] {
		return true
	}
	if name != "Nat" {
		return false
	}
	_, defined := b.model.defs[name]
	_, constant := b.model.constants[name]
	return !defined && !constant
}

func (b *propertyBuilder) children(n *coverageNode, exprs []*parser.Expression) bool {
	for _, expr := range exprs {
		child, ok := b.node(expr)
		if !ok {
			return false
		}
		if child != nil {
			n.children = append(n.children, child)
		}
	}
	return true
}

func (b *propertyBuilder) apply(n *coverageNode) bool {
	e := n.expr
	if len(e.Args) != 2 || !propertyOperator(e.Text) {
		return false
	}
	for _, arg := range e.Args {
		if arg.Kind != "Number" && arg.Kind != "Identifier" {
			return false
		}
	}
	return b.children(n, e.Args)
}

func propertyOperator(op string) bool {
	return op == "=" || op == "<" || op == "\\in" || op == ".."
}

func (b *propertyBuilder) quantifier(n *coverageNode) bool {
	e := n.expr
	if len(e.Bounds) != 1 || len(e.Args) != 1 {
		return false
	}
	bound := e.Bounds[0]
	if !b.boundSupported(bound) {
		return false
	}
	b.bound[bound.Names[0]] = true
	ok := b.children(n, e.Args)
	delete(b.bound, bound.Names[0])
	return ok && b.children(n, []*parser.Expression{bound.Set})
}

func (b *propertyBuilder) boundSupported(bound parser.Bound) bool {
	return len(bound.Names) == 1 && !bound.Tuple && b.literalRange(bound.Set) && !b.vars[bound.Names[0]] && !b.bound[bound.Names[0]]
}

func (b *propertyBuilder) literalRange(e *parser.Expression) bool {
	return e != nil && e.Kind == "Apply" && e.Text == ".." && len(e.Args) == 2 && e.Args[0].Kind == "Number" && e.Args[1].Kind == "Number"
}

func (n *coverageNode) observe(expr *parser.Expression) {
	if n.expr == expr {
		n.count++
		return
	}
	for _, child := range n.children {
		child.observe(expr)
	}
}

// TLC suppresses zero-only subtrees and collapses descendants if their
// nonzero invocation counts agree with the parent.
func (n *coverageNode) counts(counts map[int]bool) {
	if n.count > 0 {
		counts[n.count] = true
	}
	for _, child := range n.children {
		child.counts(counts)
	}
}

func (n *coverageNode) report(out []CoverageExpression, depth int) []CoverageExpression {
	counts := map[int]bool{}
	for _, child := range n.children {
		child.counts(counts)
	}
	if len(counts) == 0 {
		if n.count > 0 {
			out = append(out, CoverageExpression{Range: n.expr.Location.Range, Depth: depth, Count: n.count})
		}
		return out
	}
	if count, ok := n.collapsedCount(counts); ok {
		return append(out, CoverageExpression{Range: n.expr.Location.Range, Depth: depth, Count: count})
	}
	if n.count > 0 {
		out = append(out, CoverageExpression{Range: n.expr.Location.Range, Depth: depth, Count: n.count})
		depth++
	}
	for _, child := range n.children {
		out = child.report(out, depth)
	}
	return out
}

func (n *coverageNode) collapsedCount(counts map[int]bool) (int, bool) {
	if len(counts) == 1 {
		for count := range counts {
			if n.count == 0 || n.count == count {
				return count, true
			}
		}
	}
	return 0, false
}

func (m *model) holdsInvariant(name string, expr *parser.Expression, state evaluator.State) (bool, error) {
	if m.coverage == nil {
		return m.holds(expr, state, nil)
	}
	root := m.coverage.properties[name]
	v, err := m.program.Evaluate(expr, evaluator.Context{Current: state, Constants: m.constants, Enabled: m.enabledCallback, OnEvaluate: root.observe})
	if err != nil {
		return false, err
	}
	if b, ok := v.Bool(); ok {
		return b, nil
	}
	return false, m.nonBooleanPredicateError(expr, v)
}
