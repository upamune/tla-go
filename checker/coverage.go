package checker

import (
	"fmt"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

// CoverageAction contains counts for one initial predicate or next-state action.
// Distinct counts newly discovered in-model states; Generated includes repeats
// and states excluded by a state constraint.
type CoverageAction struct {
	Name                string
	Range               parser.Range
	Distinct, Generated int
	Property            bool
	Expressions         []CoverageExpression
}

// CoverageExpression is an expression cost at a source location. Depth is the
// nesting level in TLC's coverage report.
type CoverageExpression struct {
	Range parser.Range
	Depth int
	Count int
}

type coverageCollector struct {
	actions               []CoverageAction
	byName                map[string]int
	expressions           map[string]map[parser.Range]int
	initial               string
	bodies                map[string]*parser.Expression
	guards                map[string]map[parser.Range]int
	memberships           map[string]map[parser.Range]int
	assignmentMemberships map[string]map[parser.Range]int
	properties            map[string]*coverageNode
	existentialDomains    map[string]map[parser.Range]int
	hasExistential        bool
	evaluatedExistential  bool
}

func (m *model) newCoverage() (*coverageCollector, error) {
	c, ok := m.buildCoverage()
	if !ok {
		return nil, fmt.Errorf("-coverage does not yet support this model's action or expression forms")
	}
	return c, nil
}

func (m *model) buildCoverage() (*coverageCollector, bool) {
	if !m.coverageConfigSupported() {
		return nil, false
	}
	init, next, ok := m.coverageRoots()
	if !ok {
		return nil, false
	}
	c := &coverageCollector{byName: map[string]int{}, bodies: map[string]*parser.Expression{}, expressions: map[string]map[parser.Range]int{}, guards: map[string]map[parser.Range]int{}, memberships: map[string]map[parser.Range]int{}, assignmentMemberships: map[string]map[parser.Range]int{}, properties: map[string]*coverageNode{}, existentialDomains: map[string]map[parser.Range]int{}, initial: init.Name}
	if !m.coverageBodySupported(init.Body, true) || !c.add(init, true) {
		return nil, false
	}
	for _, branch := range next.Body.Args {
		if !m.addCoverageAction(c, branch) {
			return nil, false
		}
	}
	return c, m.addCoverageProperties(c)
}

func (m *model) addCoverageProperties(c *coverageCollector) bool {
	for _, name := range m.cfg.Invariants {
		if !m.addCoverageProperty(c, name) {
			return false
		}
	}
	return true
}

func (m *model) coverageRoots() (parser.Definition, parser.Definition, bool) {
	init, found := m.defs[m.cfg.Init]
	next, ok := m.defs[m.cfg.Next]
	return init, next, found && ok && next.Body.Kind == "Or"
}

func (m *model) coverageConfigSupported() bool {
	return !m.continueChecking && len(m.cfg.Properties)+len(m.cfg.ActionConstraints)+len(m.cfg.Postconditions)+len(m.cfg.Possible) == 0 && m.cfg.Symmetry == "" && m.cfg.View == ""
}

func (m *model) addCoverageAction(c *coverageCollector, branch *parser.Expression) bool {
	if branch.Kind != "Identifier" {
		return false
	}
	d, ok := m.defs[branch.Text]
	if !ok || len(d.Params) != 0 || d.InstanceSubstituted {
		return false
	}
	if _, exists := c.byName[d.Name]; exists {
		return false
	}
	return m.coverageBodySupported(d.Body, false) && c.add(d, false)
}

func (c *coverageCollector) add(d parser.Definition, initial bool) bool {
	c.bodies[d.Name] = d.Body
	r := d.Location.Range
	r.End = r.Start
	r.End.Column += len(d.Name) - 1
	c.byName[d.Name] = len(c.actions)
	c.expressions[d.Name] = map[parser.Range]int{}
	c.guards[d.Name] = map[parser.Range]int{}
	c.memberships[d.Name] = map[parser.Range]int{}
	c.assignmentMemberships[d.Name] = map[parser.Range]int{}
	c.existentialDomains[d.Name] = map[parser.Range]int{}
	c.actions = append(c.actions, CoverageAction{Name: d.Name, Range: r})
	return c.addExpression(d.Name, d.Body, initial)
}

func (c *coverageCollector) addExpression(name string, e *parser.Expression, initial bool) bool {
	if e.Kind == "Exists" {
		c.hasExistential = true
		if !c.addExpression(name, e.Args[0], initial) {
			return false
		}
		a := &c.actions[c.byName[name]]
		c.existentialDomains[name][e.Location.Range] = len(a.Expressions)
		a.Expressions = append(a.Expressions, CoverageExpression{Range: e.Bounds[0].Set.Location.Range})
		return true
	}
	if e.Kind != "And" {
		return c.addCoverageAtom(name, e, initial)
	}
	for _, arg := range e.Args {
		if !c.addExpression(name, arg, initial) {
			return false
		}
	}
	return true
}

func (c *coverageCollector) addCoverageAtom(name string, e *parser.Expression, initial bool) bool {
	if !coverageAtomSupported(e, initial) {
		return false
	}
	i := c.byName[name]
	a := &c.actions[i]
	c.expressions[name][e.Location.Range] = len(a.Expressions)
	a.Expressions = append(a.Expressions, CoverageExpression{Range: e.Location.Range})
	if coverageRangeMembership(e) {
		c.assignmentMemberships[name][e.Location.Range] = len(a.Expressions) - 1
		a.Expressions = append(a.Expressions, CoverageExpression{Range: e.Args[1].Location.Range, Depth: 1})
		return true
	}
	if !initial && coverageComparisonGuard(e) {
		c.guards[name][e.Location.Range] = len(a.Expressions) - 1
		a.Expressions = append(a.Expressions, CoverageExpression{Range: e.Args[0].Location.Range, Depth: 1})
		return true
	}
	if !initial && coverageNat(e) {
		c.memberships[name][e.Location.Range] = len(a.Expressions) - 1
		for _, child := range e.Args {
			a.Expressions = append(a.Expressions, CoverageExpression{Range: child.Location.Range, Depth: 1})
		}
	}
	return true
}

func coverageAtomSupported(e *parser.Expression, initial bool) bool {
	return coverageAtomic(e) && coverageMembershipSupported(e, initial)
}

func coverageComparisonGuard(e *parser.Expression) bool {
	return e.Kind == "Apply" && (e.Text == "<" || e.Text == "=") && e.Args[0].Kind == "Identifier" && e.Args[1].Kind == "Number"
}

func coverageRangeMembership(e *parser.Expression) bool {
	return e.Kind == "Apply" && e.Text == "\\in" && len(e.Args) == 2 && e.Args[1].Kind == "Apply" && e.Args[1].Text == ".."
}

func coverageMembershipSupported(e *parser.Expression, initial bool) bool {
	return initial || e.Kind != "Apply" || e.Text != "\\in" || coverageNat(e) || (e.Args[1].Kind == "Apply" && e.Args[1].Text == "..")
}

func coverageAtomic(e *parser.Expression) bool {
	if e.Kind == "Unchanged" {
		return len(e.Args) == 1 && coverageOperand(e.Args[0])
	}
	if e.Kind != "Apply" || len(e.Args) != 2 {
		return false
	}
	return coverageAtomicOperator(e.Text) && coverageOperand(e.Args[0]) && coverageOperand(e.Args[1])
}

func coverageAtomicOperator(op string) bool { return op == "=" || op == "<" || op == "\\in" }

func coverageNat(e *parser.Expression) bool {
	return e.Kind == "Apply" && e.Text == "\\in" && len(e.Args) == 2 && e.Args[1].Kind == "Identifier" && e.Args[1].Text == "Nat"
}

func (g *generator) cover(expr *parser.Expression, action string) {
	c := g.model.coverage
	if g.initial {
		action = c.initial
	}
	index, ok := c.byName[action]
	if !ok {
		return
	}
	a := &c.actions[index]
	i, ok := c.expressions[action][expr.Location.Range]
	if !ok {
		return
	}
	if _, guard := c.guards[action][expr.Location.Range]; guard {
		a.Expressions[i+1].Count++
		// TLC counts evaluation of equality at the parent as well as each
		// successful continuation; ordering comparisons count only the latter.
		if expr.Text == "=" {
			a.Expressions[i].Count++
		}
		return
	}
	a.Expressions[i].Count++
	if _, ok := c.assignmentMemberships[action][expr.Location.Range]; ok {
		a.Expressions[i+1].Count++
	}
	if _, ok := c.memberships[action][expr.Location.Range]; ok {
		a.Expressions[i+1].Count++
		a.Expressions[i+2].Count++
	}
}

// Initial memberships are charged for each member that completes a state.
// Action memberships are charged once per member with a successful continuation.
func (g *generator) coverAssignmentMember(expr *parser.Expression, partial evaluator.State, action string, last bool, count int) {
	if g.model.coverage == nil || (g.initial && (!last || !g.isComplete(partial))) {
		return
	}
	c := g.model.coverage
	if g.initial {
		action = c.initial
	}
	if i, ok := c.assignmentMemberships[action][expr.Location.Range]; ok {
		c.actions[c.byName[action]].Expressions[i].Count += count
	}
}

func (g *generator) coverAssignmentDomain(expr *parser.Expression, name string, partial evaluator.State, action string, last bool) {
	if g.model.coverage == nil || (g.initial && (!last || len(partial)+1 != len(g.model.names))) {
		return
	}
	if _, exists := partial[name]; exists {
		return
	}
	c := g.model.coverage
	if g.initial {
		action = c.initial
	}
	if i, ok := c.assignmentMemberships[action][expr.Location.Range]; ok {
		c.actions[c.byName[action]].Expressions[i].Count--
	}
}

func (g *generator) coverGuardSuccess(expr *parser.Expression, action string) {
	c := g.model.coverage
	if c == nil || g.initial {
		return
	}
	if i, ok := c.guards[action][expr.Location.Range]; ok {
		c.actions[c.byName[action]].Expressions[i].Count++
	}
	if i, ok := c.memberships[action][expr.Location.Range]; ok {
		c.actions[c.byName[action]].Expressions[i].Count++
	}
}

func (c *coverageCollector) generated(action string, initial bool) {
	if initial {
		action = c.initial
	}
	if i, ok := c.byName[action]; ok {
		c.actions[i].Generated++
	}
}

func (c *coverageCollector) distinct(action string, initial bool) {
	if initial {
		action = c.initial
	}
	if i, ok := c.byName[action]; ok {
		c.actions[i].Distinct++
	}
}

// Restrict reporting to the small expression subset whose TLC cost model is
// implemented. More complex operators need their own cost-model nodes.
func coverageOperand(e *parser.Expression) bool {
	switch e.Kind {
	case "Identifier", "Number":
		return true
	case "Prime":
		return len(e.Args) == 1 && e.Args[0].Kind == "Identifier"
	case "Tuple":
		return coverageOperandArgs(e, false)
	case "Apply":
		return coverageOperandApply(e)
	}
	return false
}

func coverageOperandApply(e *parser.Expression) bool {
	return (e.Text == "+" || e.Text == "..") && len(e.Args) == 2 && coverageOperandArgs(e, true)
}

func coverageOperandArgs(e *parser.Expression, numbers bool) bool {
	for _, arg := range e.Args {
		if arg.Kind != "Identifier" && !(numbers && arg.Kind == "Number") {
			return false
		}
	}
	return true
}

// Only permit flat conjunctions and simple aliases. In particular, evaluating
// a nested operator or guard expression requires a separate TLC cost model.
func (m *model) coverageBodySupported(body *parser.Expression, initial bool) bool {
	items := []*parser.Expression{body}
	if body.Kind == "And" {
		items = body.Args
	}
	g := generator{model: m, initial: initial}
	if !g.coverageItemsSupported(items) {
		return false
	}
	return initial || g.coverageActionOrderSupported(items)
}

func (g *generator) coverageItemsSupported(items []*parser.Expression) bool {
	for i, e := range items {
		if e.Kind == "Exists" {
			if g.initial || i != len(items)-1 || !g.coverageExistentialSupported(e, items[:i]) {
				return false
			}
		} else if !g.coverageItemSupported(e) {
			return false
		}
	}
	return true
}

func (g *generator) coverageActionOrderSupported(items []*parser.Expression) bool {
	state := coverageActionOrder{assigned: map[string]bool{}}
	for _, e := range items {
		if !state.acceptUnchangedOrder(e) {
			return false
		}
		if e.Kind == "Unchanged" {
			continue
		}
		if e.Kind != "Apply" || len(e.Args) != 2 {
			continue
		}
		name, op, assignment := g.assignment(e, nil)
		if !state.accept(e, name, op, assignment, len(g.model.names)) {
			return false
		}
	}
	return true
}

type coverageActionOrder struct {
	assigned                                 map[string]bool
	membership, deterministicAfterMembership bool
	deterministic                            bool
	unchanged                                bool
}

// TLC charges UNCHANGED for successful continuations. Subsequent guards or
// assignments can fail, including an empty range.
func (s *coverageActionOrder) acceptUnchangedOrder(e *parser.Expression) bool {
	if e.Kind == "Unchanged" {
		s.unchanged = true
		return true
	}
	return !s.unchanged
}

func (s *coverageActionOrder) accept(e *parser.Expression, name, op string, assignment bool, variables int) bool {
	if !assignment {
		// Deterministic assignment costs can depend on whether a later
		// guard produces a successor. Their child costs are not modeled yet.
		return s.guardSupported(e, variables)
	}
	s.assigned[name] = true
	s.deterministic = s.deterministic || op == "="
	s.deterministicAfterMembership = s.deterministicAfterMembership || (s.membership && op == "=")
	s.membership = s.membership || op == "\\in"
	return true
}

func (s *coverageActionOrder) guardSupported(e *parser.Expression, variables int) bool {
	if s.deterministic && !s.membership {
		return false
	}
	return !s.membership || len(s.assigned) != variables || (e.Text != "=" && !s.deterministicAfterMembership)
}

func (g *generator) coverageItemSupported(e *parser.Expression) bool {
	if e.Kind == "And" {
		return false
	}
	for _, arg := range e.Args {
		if !g.model.coverageAliasesSupported(arg, map[string]bool{}) {
			return false
		}
	}
	if e.Kind != "Apply" || len(e.Args) != 2 {
		return true
	}
	_, _, assignment := g.assignment(e, nil)
	return coverageOperationSupported(e, g.initial, assignment)
}

func coverageOperationSupported(e *parser.Expression, initial, assignment bool) bool {
	if e.Text == "\\in" {
		return coverageMembershipOperation(e, initial, assignment)
	}
	return assignment || (e.Args[0].Kind == "Identifier" && e.Args[1].Kind == "Number")
}

func coverageMembershipOperation(e *parser.Expression, initial, assignment bool) bool {
	if assignment {
		return coverageRangeMembership(e)
	}
	return !initial && e.Args[0].Kind == "Identifier" && coverageNat(e)
}

func (m *model) coverageAliasesSupported(e *parser.Expression, seen map[string]bool) bool {
	if !coverageOperand(e) {
		return false
	}
	if e.Kind == "Identifier" {
		return m.coverageAliasSupported(e.Text, seen)
	}
	for _, arg := range e.Args {
		if !m.coverageAliasesSupported(arg, seen) {
			return false
		}
	}
	return true
}

func (m *model) coverageAliasSupported(name string, seen map[string]bool) bool {
	d, ok := m.defs[name]
	if !ok {
		return true
	}
	if seen[name] || len(d.Params) > 0 || d.InstanceSubstituted {
		return false
	}
	seen[name] = true
	ok = m.coverageAliasesSupported(d.Body, seen)
	delete(seen, name)
	return ok
}

func (s *search) countGenerated(action string, initial bool) {
	s.result.GeneratedStates++
	if s.model.coverage != nil {
		s.model.coverage.generated(action, initial)
	}
}

func (s *search) countCoverageState(action string, parent int) {
	if s.model.coverage != nil {
		s.model.coverage.distinct(action, parent < 0)
	}
	s.countState(parent)
}

func validateCoverageMode(options Options) error {
	if options.Coverage && (options.Simulation != nil || options.DepthFirst != nil) {
		return fmt.Errorf("coverage is not yet supported in simulation or depth-first mode")
	}
	return nil
}

func (m *model) configureCoverage(options Options) error {
	if !options.Coverage {
		return nil
	}
	var err error
	m.coverage, err = m.newCoverage()
	return err
}

func (m *model) attachCoverage(result *Result) {
	if m.coverage != nil {
		m.coverage.finish()
		result.Coverage = m.coverage.actions
	}
}

// TLC collapses nodes whose nonzero descendants all have the same cost.
func (c *coverageCollector) finish() {
	for i := range c.actions {
		a := &c.actions[i]
		if a.Property {
			a.Expressions = c.properties[a.Name].report(nil, 0)
			continue
		}
		a.Expressions = c.finishAction(a)
		c.collapseAction(a)
	}
}

func (c *coverageCollector) collapseAction(a *CoverageAction) {
	body := c.bodies[a.Name]
	if body.Kind != "And" || !sameCoverageCounts(a.Expressions, a.Name == c.initial) {
		return
	}
	for _, e := range a.Expressions {
		if e.Count != 0 {
			a.Expressions = []CoverageExpression{{Range: body.Location.Range, Count: e.Count}}
			return
		}
	}
}

func (c *coverageCollector) finishAction(a *CoverageAction) []CoverageExpression {
	expressions := make([]CoverageExpression, 0, len(a.Expressions))
	for j := 0; j < len(a.Expressions); j++ {
		e := a.Expressions[j]
		if quantifier, ok := c.finishExistential(a, j); ok {
			expressions = append(expressions, quantifier)
			j++
			continue
		}
		if end := c.suppressedSubtree(a, j); end > j {
			j = end
			continue
		}
		if pair, ok := c.finishActionPair(a, j); ok {
			expressions = append(expressions, pair...)
			j++
			continue
		}
		if c.collapsedMembership(a, j) {
			j += 2
		}
		expressions = append(expressions, e)
	}
	return expressions
}

func (c *coverageCollector) finishActionPair(a *CoverageAction, index int) ([]CoverageExpression, bool) {
	e := a.Expressions[index]
	if _, ok := c.guards[a.Name][e.Range]; ok {
		return appendCoverageGuard(nil, e, a.Expressions[index+1]), true
	}
	if _, ok := c.assignmentMemberships[a.Name][e.Range]; ok {
		return c.appendMembership(nil, a.Name, e, a.Expressions[index+1]), true
	}
	return nil, false
}

// The quantified action's body and domain are sibling cost nodes in TLC.
func (c *coverageCollector) finishExistential(a *CoverageAction, index int) (CoverageExpression, bool) {
	for quantifier, domainIndex := range c.existentialDomains[a.Name] {
		if domainIndex != index+1 {
			continue
		}
		body, domain := a.Expressions[index], a.Expressions[domainIndex]
		if domain.Count == 0 {
			return body, true
		}
		if body.Count == domain.Count {
			return CoverageExpression{Range: quantifier, Count: body.Count}, true
		}
	}
	return CoverageExpression{}, false
}

func (c *coverageCollector) suppressedSubtree(a *CoverageAction, index int) int {
	_, assignment := c.assignmentMemberships[a.Name][a.Expressions[index].Range]
	if a.Name != c.initial && assignment {
		return index
	}
	return unvisitedCoverageSubtree(a.Expressions, index)
}

func (c *coverageCollector) collapsedMembership(a *CoverageAction, index int) bool {
	e := a.Expressions[index]
	_, ok := c.memberships[a.Name][e.Range]
	return ok && index+2 < len(a.Expressions) && a.Expressions[index+1].Count == e.Count && a.Expressions[index+2].Count == e.Count
}

func (c *coverageCollector) appendMembership(expressions []CoverageExpression, action string, parent, child CoverageExpression) []CoverageExpression {
	if action == c.initial {
		return appendCoverageGuard(expressions, parent, child)
	}
	if parent.Count == child.Count {
		return append(expressions, parent)
	}
	return append(expressions, parent, child)
}

func appendCoverageGuard(expressions []CoverageExpression, parent, child CoverageExpression) []CoverageExpression {
	if parent.Count == 0 {
		parent.Count = child.Count
	} else if parent.Count != child.Count {
		return append(expressions, parent, child)
	}
	return append(expressions, parent)
}

// A cost node with children that were all unvisited is suppressed.
func unvisitedCoverageSubtree(expressions []CoverageExpression, start int) int {
	e := expressions[start]
	end := start + 1
	if e.Count != 0 || end == len(expressions) || expressions[end].Depth <= e.Depth {
		return start
	}
	for end < len(expressions) && expressions[end].Depth > e.Depth {
		if expressions[end].Count != 0 {
			return start
		}
		end++
	}
	return end - 1
}

func sameCoverageCounts(expressions []CoverageExpression, ignoreZero bool) bool {
	count := 0
	for _, e := range expressions {
		if e.Count == 0 {
			if !ignoreZero {
				return false
			}
			continue
		}
		if count != 0 && e.Count != count {
			return false
		}
		count = e.Count
	}
	return count != 0
}

// coverageExistentialSupported models a final existential whose single witness
// assigns the sole state variable. Earlier conjuncts must be ordinary guards.
// Larger continuation trees require TLC's cost model for each branch.
func (g *generator) coverageExistentialSupported(e *parser.Expression, earlier []*parser.Expression) bool {
	if !g.coverageExistentialShape(e, earlier) {
		return false
	}
	b := e.Bounds[0]
	guard := earlier[0]
	if !coverageComparisonGuard(guard) || guard.Text != "<" || guard.Args[0].Text != g.model.names[0] {
		return false
	}
	return g.coverageExistentialAssignment(e.Args[0], b.Names[0])
}

func (g *generator) coverageExistentialShape(e *parser.Expression, earlier []*parser.Expression) bool {
	if len(earlier) != 1 || len(g.model.names) != 1 || len(e.Bounds) != 1 || len(e.Args) != 1 {
		return false
	}
	b := e.Bounds[0]
	return len(b.Names) == 1 && !b.Tuple && coverageLiteralInterval(b.Set)
}

func (g *generator) coverageExistentialAssignment(body *parser.Expression, bound string) bool {
	if body.Kind != "Apply" || body.Text != "=" || len(body.Args) != 2 || body.Args[1].Kind != "Identifier" || body.Args[1].Text != bound {
		return false
	}
	return bound != g.model.names[0] && coveragePrimeVariable(body.Args[0], g.model.names[0])
}

func coveragePrimeVariable(e *parser.Expression, name string) bool {
	return e.Kind == "Prime" && len(e.Args) == 1 && e.Args[0].Kind == "Identifier" && e.Args[0].Text == name
}

func coverageLiteralInterval(e *parser.Expression) bool {
	return e != nil && e.Kind == "Apply" && e.Text == ".." && len(e.Args) == 2 && e.Args[0].Kind == "Number" && e.Args[1].Kind == "Number"
}

func (g *generator) coverExistentialDomain(expr *parser.Expression, action string) {
	if g.model.coverage == nil {
		return
	}
	c := g.model.coverage
	if i, ok := c.existentialDomains[action][expr.Location.Range]; ok {
		c.evaluatedExistential = true
		c.actions[c.byName[action]].Expressions[i].Count++
	}
}
