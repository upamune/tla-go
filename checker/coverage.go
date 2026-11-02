package checker

import (
	"cmp"
	"fmt"
	"slices"
	"sync"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

// CoverageAction contains counts for one initial predicate or next-state action.
// For a next action, Distinct counts newly discovered in-model states. For
// Init, it counts states satisfying the initial predicate before constraints,
// including repeated candidates. Generated includes repeats and rejected
// initial candidates.
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
	Cost  int
}

// The schema is immutable after construction. mu protects every runtime update
// and the entire report, including property trees. No evaluator or user callback
// runs while mu is held.
type coverageCollector struct {
	mu                    sync.Mutex
	model                 *model
	actions               []CoverageAction
	byName                map[string]int
	expressions           map[string]map[parser.Range]int
	initial               string
	bodies                map[string]*parser.Expression
	guards                map[string]map[parser.Range]int
	memberships           map[string]map[parser.Range]int
	assignmentMemberships map[string]map[parser.Range]int
	assignmentOperands    map[string]map[parser.Range]int
	properties            map[string]*coverageNode
	existentialDomains    map[string]map[parser.Range]int
	parameterCalls        map[*parser.Expression]*coverageParameterizedRoot
	hasExistential        bool
	evaluatedExistential  bool
	unreliableReplay      bool
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
	// The implicit empty model has neither an initial action nor a next
	// action to instrument. An explicit specification can still contribute
	// coverage actions even when it produces no states.
	if m.emptyCoverageModel() {
		return &coverageCollector{}, true
	}
	init, next, ok := m.coverageRoots()
	if !ok {
		return nil, false
	}
	c := &coverageCollector{model: m, byName: map[string]int{}, bodies: map[string]*parser.Expression{}, expressions: map[string]map[parser.Range]int{}, guards: map[string]map[parser.Range]int{}, memberships: map[string]map[parser.Range]int{}, assignmentMemberships: map[string]map[parser.Range]int{}, assignmentOperands: map[string]map[parser.Range]int{}, properties: map[string]*coverageNode{}, existentialDomains: map[string]map[parser.Range]int{}, initial: init.Name}
	if !m.coverageBodySupported(init.Body, true) || !m.coverageInitialSetContext(init.Body) || !c.add(init, true) {
		return nil, false
	}
	if !m.addCoverageNext(c, next) {
		return nil, false
	}
	return c, m.addCoverageProperties(c)
}

func (m *model) emptyCoverageModel() bool {
	return m.noStateSearch() && !m.fromSpecification && len(m.cfg.Invariants)+len(m.cfg.Constraints) == 0
}

func (m *model) addCoverageProperties(c *coverageCollector) bool {
	for _, name := range m.cfg.Invariants {
		if m.coverageLiteralTrueInvariant(name) && m.coverageTotalUnchangedSafe() {
			continue
		}
		if !m.addCoverageProperty(c, name) {
			return false
		}
	}
	return true
}

// TLC omits constant TRUE invariants from the property coverage report.
func (m *model) coverageLiteralTrueInvariant(name string) bool {
	d, ok := m.defs[name]
	return ok && m.trueConfigured[name] && coverageDirectDefinition(d) && d.Body != nil && d.Body.Kind == "Identifier" && d.Body.Text == "TRUE"
}

func (m *model) coverageRoots() (parser.Definition, parser.Definition, bool) {
	init, found := m.defs[m.cfg.Init]
	next, ok := m.defs[m.cfg.Next]
	return init, next, found && ok && init.Body != nil && next.Body != nil
}

func (m *model) addCoverageNext(c *coverageCollector, next parser.Definition) bool {
	if next.Body.Kind != "Or" {
		return m.addCoverageUnchangedNext(c, next)
	}
	for _, branch := range next.Body.Args {
		if branch.Kind == "Apply" {
			return m.addCoverageParameterizedCalls(c, next.Body.Args)
		}
	}
	for _, branch := range next.Body.Args {
		if !m.addCoverageAction(c, branch) {
			return false
		}
	}
	return true
}

func (m *model) coverageConfigSupported() bool {
	return !m.continueChecking && len(m.cfg.Properties)+len(m.cfg.ActionConstraints)+len(m.cfg.Postconditions)+len(m.cfg.Possible) == 0 && m.cfg.Symmetry == "" && m.cfg.View == "" && !m.coverageConstraintUsesRuntimeRead()
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
		if !c.addCoverageAtom(name, e.Args[0], initial, true) {
			return false
		}
		a := &c.actions[c.byName[name]]
		c.existentialDomains[name][e.Location.Range] = len(a.Expressions)
		a.Expressions = append(a.Expressions, CoverageExpression{Range: e.Bounds[0].Set.Location.Range})
		return true
	}
	if e.Kind != "And" {
		return c.addCoverageAtom(name, e, initial, false)
	}
	for _, arg := range e.Args {
		if !c.addExpression(name, arg, initial) {
			return false
		}
	}
	return true
}

func (c *coverageCollector) addCoverageAtom(name string, e *parser.Expression, initial, existential bool) bool {
	if !coverageAtomSupported(e, initial) {
		return false
	}
	i := c.byName[name]
	a := &c.actions[i]
	c.expressions[name][e.Location.Range] = len(a.Expressions)
	a.Expressions = append(a.Expressions, CoverageExpression{Range: e.Location.Range})
	if c.addAssignmentCoverageNode(a, name, e, initial, existential) {
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

func (c *coverageCollector) addAssignmentCoverageNode(a *CoverageAction, name string, e *parser.Expression, initial, existential bool) bool {
	if existential {
		return false
	}
	var indices map[parser.Range]int
	if coverageAssignmentMembership(e, initial) {
		indices = c.assignmentMemberships[name]
	} else if !initial && c.coverageAssignmentOperand(name, e) {
		indices = c.assignmentOperands[name]
		if indices == nil {
			indices = make(map[parser.Range]int)
			c.assignmentOperands[name] = indices
		}
	} else {
		return false
	}
	indices[e.Location.Range] = len(a.Expressions) - 1
	a.Expressions = append(a.Expressions, CoverageExpression{Range: e.Args[1].Location.Range, Depth: 1})
	return true
}

// Arithmetic and identifiers on the right of an assignment have their own
// TLC cost node, independent of continuation success.
func (c *coverageCollector) coverageAssignmentOperand(name string, e *parser.Expression) bool {
	if len(c.model.defs[name].Params) != 0 {
		return false
	}
	g := generator{model: c.model}
	if _, operator, ok := g.assignment(e, nil); !ok || operator != "=" {
		return false
	}
	rhs := e.Args[1]
	return rhs.Kind == "Apply" && rhs.Text == "+" || rhs.Kind == "Identifier"
}

func coverageAtomSupported(e *parser.Expression, initial bool) bool {
	return (coverageAtomic(e, initial) || (!initial && coverageBooleanToggleSyntax(e))) && coverageMembershipSupported(e, initial)
}

func coverageComparisonGuard(e *parser.Expression) bool {
	return e.Kind == "Apply" && (e.Text == "<" || e.Text == "=") && e.Args[0].Kind == "Identifier" && e.Args[1].Kind == "Number"
}

func coverageRangeMembership(e *parser.Expression) bool {
	return e.Kind == "Apply" && e.Text == "\\in" && len(e.Args) == 2 && e.Args[1].Kind == "Apply" && e.Args[1].Text == ".."
}

func coverageAssignmentMembership(e *parser.Expression, initial bool) bool {
	return coverageRangeMembership(e) || initial && e.Kind == "Apply" && e.Text == "\\in" && len(e.Args) == 2 && coverageInitialSet(e.Args[1])
}

func coverageMembershipSupported(e *parser.Expression, initial bool) bool {
	return initial || e.Kind != "Apply" || e.Text != "\\in" || coverageNat(e) || (e.Args[1].Kind == "Apply" && e.Args[1].Text == "..")
}

func coverageAtomic(e *parser.Expression, initial bool) bool {
	if e.Kind == "Unchanged" {
		return len(e.Args) == 1 && coverageOperand(e.Args[0])
	}
	if e.Kind != "Apply" || len(e.Args) != 2 {
		return false
	}
	rhs := coverageOperand(e.Args[1]) || coverageInitialSetAssignment(e, initial)
	return coverageAtomicOperator(e.Text) && coverageOperand(e.Args[0]) && rhs
}

func coverageInitialSetAssignment(e *parser.Expression, initial bool) bool {
	return initial && e.Kind == "Apply" && (e.Text == "\\in" || e.Text == "=") && len(e.Args) == 2 && coverageInitialSet(e.Args[1])
}

func coverageAtomicOperator(op string) bool { return op == "=" || op == "<" || op == "\\in" }

func coverageNat(e *parser.Expression) bool {
	return e.Kind == "Apply" && e.Text == "\\in" && len(e.Args) == 2 && e.Args[1].Kind == "Identifier" && e.Args[1].Text == "Nat"
}

func (g *generator) cover(expr *parser.Expression, action string) {
	c := g.model.coverage
	c.mu.Lock()
	defer c.mu.Unlock()
	if g.enabled {
		if g.coverageProperty != nil {
			g.coverageProperty.observe(expr)
		}
		return
	}
	if !g.initial && g.coverageRoot != nil {
		a := &c.actions[g.coverageRoot.action]
		if len(a.Expressions) == 1 {
			a.Expressions[0].Count++
		} else {
			c.coverExpression(expr, action)
		}
		return
	}
	if g.initial {
		action = c.initial
	}
	c.coverExpression(expr, action)
}

func (c *coverageCollector) coverExpression(expr *parser.Expression, action string) {
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
	if i, ok := c.assignmentOperands[action][expr.Location.Range]; ok {
		a.Expressions[i+1].Count++
	}
	a.Expressions[i].Count++
	c.coverMembershipExpression(a, action, expr, i)
}

func (c *coverageCollector) coverMembershipExpression(a *CoverageAction, action string, expr *parser.Expression, i int) {
	if _, ok := c.assignmentMemberships[action][expr.Location.Range]; ok {
		a.Expressions[i+1].Count++
		if isInitialSetMembership(c, action, expr) {
			a.Expressions[i+1].Cost++
		}
	}
	if _, ok := c.memberships[action][expr.Location.Range]; ok {
		a.Expressions[i+1].Count++
		a.Expressions[i+2].Count++
	}
}

func isInitialSetMembership(c *coverageCollector, action string, expr *parser.Expression) bool {
	return action == c.initial && len(expr.Args) == 2 && expr.Args[1].Kind == "SetEnumeration"
}

// coverageAssignmentAttempt records evaluation of a deterministic assignment
// independently of whether its continuation ultimately emits a successor.
func (g *generator) coverageAssignmentAttempt(expr *parser.Expression, action string) {
	c := g.model.coverage
	c.mu.Lock()
	defer c.mu.Unlock()
	if i, ok := c.assignmentOperands[action][expr.Location.Range]; ok {
		c.actions[c.byName[action]].Expressions[i+1].Count++
	}
}

func (g *generator) coverageAssignmentSuccess(expr *parser.Expression, action string) {
	c := g.model.coverage
	c.mu.Lock()
	defer c.mu.Unlock()
	if i, ok := c.expressions[action][expr.Location.Range]; ok {
		c.actions[c.byName[action]].Expressions[i].Count++
	}
}

// Initial memberships are charged for each member that completes a state.
// Action memberships are charged once per member with a successful continuation.
func (g *generator) coverAssignmentMember(expr *parser.Expression, partial evaluator.State, action string, last bool, count int) {
	if g.model.coverage == nil || (g.initial && (!last || !g.isComplete(partial))) {
		return
	}
	c := g.model.coverage
	c.mu.Lock()
	defer c.mu.Unlock()
	if g.initial {
		action = c.initial
	}
	if i, ok := c.assignmentMemberships[action][expr.Location.Range]; ok {
		c.actions[c.byName[action]].Expressions[i].Count += count
		g.coverSetMemberCost(c, action, expr, i, count)
	}
}

func (g *generator) coverSetMemberCost(c *coverageCollector, action string, expr *parser.Expression, i, count int) {
	if g.initial && expr.Args[1].Kind == "SetEnumeration" {
		c.actions[c.byName[action]].Expressions[i+1].Cost += count
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
	c.mu.Lock()
	defer c.mu.Unlock()
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
	c.mu.Lock()
	defer c.mu.Unlock()
	if i, ok := c.guards[action][expr.Location.Range]; ok {
		c.actions[c.byName[action]].Expressions[i].Count++
	}
	if i, ok := c.memberships[action][expr.Location.Range]; ok {
		c.actions[c.byName[action]].Expressions[i].Count++
	}
}

func (c *coverageCollector) generated(action string, initial bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if initial {
		action = c.initial
	}
	if i, ok := c.byName[action]; ok {
		c.actions[i].Generated++
	}
}

func (c *coverageCollector) distinct(action string, initial bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
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
	if !initial && coverageBooleanToggleSyntax(body) && m.coverageBooleanToggleProgram() {
		return true
	}
	items := coverageConjuncts(body)
	g := generator{model: m, initial: initial}
	if !g.coverageItemsSupported(items) {
		return false
	}
	return initial || g.coverageActionOrderSupported(items)
}

// Set assignments are modeled only when every initial conjunct directly
// assigns a complete variable and no state constraint affects the count.
func (m *model) coverageInitialSetContext(body *parser.Expression) bool {
	items := coverageConjuncts(body)
	if !coverageHasSetOperand(items) {
		return true
	}
	if len(m.cfg.Constraints) != 0 || !m.coverageDirectInitial(body) {
		return false
	}
	return !coverageHasSetEquality(items) || m.coverageTotalUnchangedSafe()
}

func coverageHasSetEquality(items []*parser.Expression) bool {
	for _, e := range items {
		if e.Kind == "Apply" && e.Text == "=" && len(e.Args) == 2 && e.Args[1].Kind == "SetEnumeration" {
			return true
		}
	}
	return false
}

// Newly supported constant-TRUE invariants and literal set equalities need a
// total direct UNCHANGED check. Other checks can replay Init and Next while
// reporting a violation, changing costs.
func (m *model) coverageTotalUnchangedSafe() bool {
	if len(m.module.Assumptions) != 0 || m.graph != nil {
		return false
	}
	for _, name := range m.cfg.Invariants {
		if !m.coverageLiteralTrueInvariant(name) {
			return false
		}
	}
	return m.coverageTotalUnchangedNextSafe()
}

func (m *model) coverageTotalUnchangedNextSafe() bool {
	next, ok := m.defs[m.cfg.Next]
	return ok && coverageDirectDefinition(next) && next.Body != nil && !m.trueConfigured[next.Name] && m.coverageUnchangedSequence(next.Body)
}

func coverageConjuncts(body *parser.Expression) []*parser.Expression {
	if body.Kind == "And" {
		return body.Args
	}
	return []*parser.Expression{body}
}

func coverageHasSetOperand(items []*parser.Expression) bool {
	for _, e := range items {
		if e.Kind == "Apply" && len(e.Args) == 2 && e.Args[1].Kind == "SetEnumeration" {
			return true
		}
	}
	return false
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
	init := g.model.defs[g.model.cfg.Init]
	state := coverageActionOrder{assigned: map[string]bool{}, initialSet: coverageHasSetOperand(coverageConjuncts(init.Body))}
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
	initialSet                               bool
}

// TLC charges UNCHANGED for successful continuations. Subsequent guards or
// assignments can fail, including an empty range.
func (s *coverageActionOrder) acceptUnchangedOrder(e *parser.Expression) bool {
	if e.Kind == "Unchanged" {
		s.unchanged = true
		// A previous assignment may make UNCHANGED fail, changing how TLC
		// charges it. Do not extend this unmodeled case to set initializers.
		return !s.initialSet || len(s.assigned) == 0
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
	for i, arg := range e.Args {
		if i == 1 && coverageInitialSetAssignment(e, g.initial) {
			continue
		}
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
	if e.Args[1].Kind == "SetEnumeration" && !coverageSetAssignmentAllowed(e.Text, initial, assignment) {
		return false
	}
	if e.Text == "\\in" {
		return coverageMembershipOperation(e, initial, assignment)
	}
	return assignment || (e.Args[0].Kind == "Identifier" && e.Args[1].Kind == "Number")
}

func coverageSetAssignmentAllowed(op string, initial, assignment bool) bool {
	return initial && assignment && (op == "\\in" || op == "=")
}

func coverageMembershipOperation(e *parser.Expression, initial, assignment bool) bool {
	if assignment {
		return coverageAssignmentMembership(e, initial)
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
	s.publishProgress()
	if s.model.coverage != nil {
		s.model.coverage.generated(action, initial)
		if initial {
			s.model.coverage.distinct(action, true)
		}
	}
}

func (s *search) countCoverageState(action string, parent int) {
	if s.model.coverage != nil && parent >= 0 {
		s.model.coverage.distinct(action, false)
	}
	s.countState(parent)
}

func validateCoverageMode(options Options) error {
	if options.LiveProgress != nil && !options.Coverage {
		return fmt.Errorf("live progress requires coverage")
	}
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
		result.Coverage = m.coverage.report()
	}
}

// report leaves the collector's counters and update indices intact. Callers
// may run concurrently with updates. Each report reflects one instant between updates.
func (c *coverageCollector) report() []CoverageAction {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.actions == nil {
		return nil
	}
	actions := make([]CoverageAction, len(c.actions))
	copy(actions, c.actions)
	c.sortCoverageActions(actions)
	for i := range actions {
		a := &actions[i]
		if a.Property {
			a.Expressions = c.properties[a.Name].report(nil, 0)
			continue
		}
		a.Expressions = c.finishAction(a)
		c.collapseAction(a)
	}
	return actions
}

// CostModelCreator.report orders next-state actions by predicate location.
// Different modules use SANY's interned-name IDs rather than lexical ordering,
// and identical locations may share a cost model. Until those cases are
// modeled, preserve the previous order for them.
func (c *coverageCollector) sortCoverageActions(actions []CoverageAction) {
	if len(actions) < 3 {
		return
	}
	end := 1
	for end < len(actions) && !actions[end].Property {
		end++
	}
	if !c.coverageActionLocationsOrderable(actions[1:end]) {
		return
	}
	slices.SortStableFunc(actions[1:end], func(a, b CoverageAction) int {
		x, y := c.bodies[a.Name].Location.Range, c.bodies[b.Name].Location.Range
		return cmp.Or(cmp.Compare(x.Module, y.Module), cmp.Compare(x.Start.Line, y.Start.Line), cmp.Compare(x.Start.Column, y.Start.Column), cmp.Compare(x.End.Line, y.End.Line), cmp.Compare(x.End.Column, y.End.Column))
	})
}

func (c *coverageCollector) coverageActionLocationsOrderable(actions []CoverageAction) bool {
	var small [16]parser.Range
	ranges := small[:0]
	if len(actions) > len(small) {
		ranges = make([]parser.Range, 0, len(actions))
	}
	for _, action := range actions {
		body := c.bodies[action.Name]
		if body == nil {
			return false
		}
		ranges = append(ranges, body.Location.Range)
	}
	return coverageSameModuleDistinct(ranges)
}

func coverageSameModuleDistinct(ranges []parser.Range) bool {
	seen := make(map[parser.Range]bool, len(ranges))
	for _, r := range ranges {
		if !coverageLocated(r) || r.Module != ranges[0].Module || seen[r] {
			return false
		}
		seen[r] = true
	}
	return true
}

func coverageLocated(r parser.Range) bool {
	return r.Module != "" && r.Start.Line > 0 && r.Start.Column > 0 && r.End.Line > 0 && r.End.Column > 0
}

// TLC collapses nodes whose nonzero descendants all have the same cost.
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
	if _, ok := c.assignmentOperands[a.Name][e.Range]; ok {
		child := a.Expressions[index+1]
		if e.Count != child.Count {
			return []CoverageExpression{e, child}, true
		}
		return []CoverageExpression{e}, true
	}
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
	if _, ok := c.assignmentOperands[a.Name][a.Expressions[index].Range]; ok {
		return index
	}
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
	c.mu.Lock()
	defer c.mu.Unlock()
	if i, ok := c.existentialDomains[action][expr.Location.Range]; ok {
		c.evaluatedExistential = true
		c.actions[c.byName[action]].Expressions[i].Count++
	}
}

func (c *coverageCollector) existentialEvaluated() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.evaluatedExistential
}
