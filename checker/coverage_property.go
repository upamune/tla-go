package checker

import (
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

// coverageNode mirrors the subset of TLC's OpApplNode cost tree used for
// state predicates. Numeric literals are not OpApplNodes in SANY.
type coverageNode struct {
	expr       *parser.Expression
	count      int
	cost       int
	boundCosts map[*parser.Expression]*coverageNode
	children   []*coverageNode
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
	calls       map[string]bool
	params      map[string]bool
	subsets     map[string]*coverageNode
	costs       map[*parser.Expression]*coverageNode
}

func (m *model) propertyCoverageNode(e *parser.Expression, vars, bound map[string]bool, seen *bool) (*coverageNode, bool) {
	b := &propertyBuilder{model: m, vars: vars, bound: bound, seen: seen, calls: map[string]bool{}, params: map[string]bool{}, subsets: map[string]*coverageNode{}, costs: map[*parser.Expression]*coverageNode{}}
	n, ok := b.node(e)
	if n != nil {
		n.boundCosts = b.costs
	}
	return n, ok
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
		return n, b.identifierNode(n)
	case "Apply":
		return n, b.applyNode(n)
	case "And", "Or":
		if !e.Bulleted && len(e.Args) != 2 {
			return nil, false
		}
		return n, b.children(n, e.Args)
	}
	return b.boundNode(n)
}

func (b *propertyBuilder) identifierNode(n *coverageNode) bool {
	name := n.expr.Text
	if d, ok := b.model.defs[name]; ok && !b.vars[name] && !b.bound[name] {
		return b.call(n, d)
	}
	return b.identifier(name)
}

func (b *propertyBuilder) applyNode(n *coverageNode) bool {
	if d, ok := b.model.defs[n.expr.Text]; ok {
		return b.call(n, d)
	}
	return b.apply(n)
}

// Each definition may occur once per property tree: observations identify
// expressions, but do not carry a call stack to disambiguate shared bodies.
// Literal arguments avoid attributing lazy argument evaluations to the wrong
// branch of the cost tree.
func (b *propertyBuilder) call(n *coverageNode, d parser.Definition) bool {
	if !b.callSupported(n, d) {
		return false
	}
	b.calls[d.Name] = true
	for _, p := range d.Params {
		b.bound[p.Name] = true
		b.params[p.Name] = true
	}
	outerSeen := b.seen
	localSeen := false
	b.seen = &localSeen
	ok := b.children(n, []*parser.Expression{d.Body})
	b.seen = outerSeen
	*outerSeen = *outerSeen || localSeen
	for _, p := range d.Params {
		delete(b.bound, p.Name)
		delete(b.params, p.Name)
	}
	// Constant nullary definitions may be cached outside this evaluation.
	return ok && (len(d.Params) > 0 || localSeen)
}

func (b *propertyBuilder) callSupported(n *coverageNode, d parser.Definition) bool {
	if b.calls[d.Name] || d.Kind != "Operator" || d.Recursive || d.InstanceSubstituted || b.model.trueConfigured[d.Name] || len(d.Params) != len(n.expr.Args) {
		return false
	}
	if _, overridden := b.model.constants[d.Name]; overridden {
		return false
	}
	return b.callArgumentsSupported(n, d)
}

func (b *propertyBuilder) callArgumentsSupported(n *coverageNode, d parser.Definition) bool {
	for i, p := range d.Params {
		if p.Arity != 0 || b.vars[p.Name] || b.bound[p.Name] || n.expr.Args[i].Kind != "Number" {
			return false
		}
	}
	return true
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
		return b.subsets[name] == nil
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
	if b.subsetRange(e) {
		return b.children(n, e.Args)
	}
	if len(e.Args) != 2 || !propertyOperator(e.Text) {
		return false
	}
	for _, arg := range e.Args {
		if !b.simpleOperand(e.Text, arg) {
			return false
		}
	}
	return b.children(n, e.Args)
}

func (b *propertyBuilder) subsetRange(e *parser.Expression) bool {
	return e.Text == "SUBSET" && len(e.Args) == 1 && b.literalRange(e.Args[0])
}

func (b *propertyBuilder) simpleOperand(op string, arg *parser.Expression) bool {
	return arg.Kind == "Number" || arg.Kind == "Identifier" || (op == "\\in" && b.literalRange(arg))
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
	if !b.quantifierBoundSupported(bound) {
		return false
	}
	subset, ok := b.quantifierDomain(bound)
	if !ok {
		return false
	}
	b.bound[bound.Names[0]] = true
	ok = b.children(n, e.Args)
	delete(b.bound, bound.Names[0])
	if subset != nil {
		delete(b.subsets, bound.Names[0])
		n.children = append(n.children, subset)
		return ok
	}
	return ok && b.quantifierDomainChild(n, bound.Set)
}

func (b *propertyBuilder) quantifierDomainChild(n *coverageNode, domain *parser.Expression) bool {
	if domain.Kind == "Identifier" && b.costs[domain] != nil {
		n.children = append(n.children, &coverageNode{expr: domain})
		return true
	}
	return b.children(n, []*parser.Expression{domain})
}

func (b *propertyBuilder) quantifierBoundSupported(bound parser.Bound) bool {
	return len(bound.Names) == 1 && !bound.Tuple && !b.vars[bound.Names[0]] && !b.bound[bound.Names[0]]
}

func (b *propertyBuilder) quantifierDomain(bound parser.Bound) (*coverageNode, bool) {
	domain := bound.Set
	if domain == nil {
		return nil, false
	}
	if domain.Kind == "Identifier" && b.subsets[domain.Text] != nil {
		b.costs[domain] = b.subsets[domain.Text]
		return nil, true
	}
	if b.literalRange(domain) {
		return nil, true
	}
	if domain.Kind != "Apply" || !b.subsetRange(domain) {
		return nil, false
	}
	subset, ok := b.node(domain)
	if ok {
		b.subsets[bound.Names[0]] = subset
	}
	return subset, ok
}

func (b *propertyBuilder) literalRange(e *parser.Expression) bool {
	return e != nil && e.Kind == "Apply" && e.Text == ".." && len(e.Args) == 2 && b.rangeEndpoint(e.Args[0]) && b.rangeEndpoint(e.Args[1])
}

func (b *propertyBuilder) rangeEndpoint(e *parser.Expression) bool {
	return e.Kind == "Number" || (e.Kind == "Identifier" && b.params[e.Text])
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
			out = append(out, CoverageExpression{Range: n.expr.Location.Range, Depth: depth, Count: n.count, Cost: n.cost})
		}
		return out
	}
	if count, ok := n.collapsedCount(counts); ok {
		cost := 0
		if n.count == count {
			cost = n.cost
		}
		return append(out, CoverageExpression{Range: n.expr.Location.Range, Depth: depth, Count: count, Cost: cost})
	}
	if n.count > 0 {
		out = append(out, CoverageExpression{Range: n.expr.Location.Range, Depth: depth, Count: n.count, Cost: n.cost})
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
	c := m.coverage
	root := c.properties[name]

	v, err := m.program.Evaluate(expr, evaluator.Context{Current: state, Constants: m.constants, Enabled: m.enabledCallback, OnEvaluate: func(expr *parser.Expression) { c.observeProperty(root, expr) }, OnBoundVisit: func(domain *parser.Expression, last bool) { c.observePropertyBound(root, domain, last) }})
	if err != nil {
		return false, err
	}
	if b, ok := v.Bool(); ok {
		return b, nil
	}
	return false, m.nonBooleanPredicateError(expr, v)
}

// TLC prefetches the next bound value before evaluating the current one.
func (n *coverageNode) observeBound(domain *parser.Expression, _ bool) {
	if target := n.boundCosts[domain]; target != nil {
		target.cost++
	}
}

// Callbacks acquire the lock only for their own update, never across evaluation.
func (c *coverageCollector) observeProperty(root *coverageNode, expr *parser.Expression) {
	c.mu.Lock()
	defer c.mu.Unlock()
	root.observe(expr)
}

func (c *coverageCollector) observePropertyBound(root *coverageNode, domain *parser.Expression, last bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	root.observeBound(domain, last)
}
