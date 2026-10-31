package checker

import (
	"fmt"
	"maps"
	"slices"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

type work struct {
	expr   *parser.Expression
	path   *workPath
	label  bool
	locals map[string]value.Value
}
type generator struct {
	model           *model
	current         evaluator.State
	initial         bool
	enabled         bool
	allowIncomplete bool
	lexical         *evaluator.LexicalScope
	source          *parser.Expression
	emit            func(evaluator.State, string, *parser.Expression) (bool, error)
	// undo records UNCHANGED writes to the shared partial state.
	undo []stateUndo
}

// partialState has room for every variable, which most generated states assign.
func (m *model) partialState() evaluator.State {
	return make(evaluator.State, len(m.names))
}

// takePartial lends a partial state, which a generator empties again before
// its solve returns. Nested generators get their own.
func (m *model) takePartial() evaluator.State {
	partial := m.sharedPartial
	m.sharedPartial = nil
	if partial == nil {
		partial = m.partialState()
	}
	return partial
}

func (m *model) returnPartial(partial evaluator.State) {
	if len(partial) == 0 {
		m.sharedPartial = partial
	}
}

func (g *generator) context(partial evaluator.State) (evaluator.State, evaluator.State) {
	if g.initial {
		return partial, nil
	}
	return g.current, partial
}

func (g *generator) solve(items *workList, partial evaluator.State, action string) (bool, error) {
	if items != nil && items.item.path.hasLexicalScope() {
		previous := g.lexical
		g.lexical = items.item.path.scope
		more, err := g.solveInScope(items, partial, action)
		g.lexical = previous
		return more, err
	}
	return g.solveInScope(items, partial, action)
}

func (g *generator) solveInScope(items *workList, partial evaluator.State, action string) (bool, error) {
	if err := g.model.ctx.Err(); err != nil {
		return false, err
	}
	if items == nil {
		return g.complete(partial, action)
	}
	if !g.enabled && (g.initial || len(partial) > 0) && g.isComplete(partial) {
		return g.finishAssigned(items, partial, action)
	}
	item, rest := items.item, items.next
	if handled, more, err := g.structural(item, rest, partial, action); handled {
		return more, err
	}
	return g.atomic(items, partial, action)
}

func (g *generator) atomic(items *workList, partial evaluator.State, action string) (bool, error) {
	expr := items.item.expr
	rest := items.next
	if expr.Kind == "Unchanged" && !g.initial {
		return g.unchanged(expr, rest, partial, action)
	}
	if name, operator, ok := g.assignment(expr, items.item.locals); ok {
		return g.assign(name, operator, expr.Args[1], items, partial, action)
	}
	return g.guard(items, partial, action)
}

func (g *generator) structural(item work, rest *workList, partial evaluator.State, action string) (bool, bool, error) {
	if item.expr.Kind == "And" {
		more, err := g.conjunction(item, rest, partial, action)
		return true, more, err
	}
	if item.expr.Kind == "Or" {
		more, err := g.disjunction(item, rest, partial, action)
		return true, more, err
	}
	if handled, more, err := g.scopedAction(item, rest, partial, action); handled {
		return true, more, err
	}

	if g.isActionCall(item.expr) {
		more, err := g.expandCall(item, rest, partial, action)
		return true, more, err
	}
	if item.expr.Kind == "Identifier" && g.hasLocalValue(item.expr.Text, item.locals) {
		return false, false, nil
	}
	if expanded, ok := g.expand(item.expr); ok {
		more, err := g.expandWork(item, expanded, rest, partial, action)
		return true, more, err
	}
	return false, false, nil
}

func (g *generator) scopedAction(item work, rest *workList, partial evaluator.State, action string) (bool, bool, error) {
	switch item.expr.Kind {
	case "Identifier":
		return g.lazyEnabledAction(item, rest, partial, action)
	case "Apply":
		if item.expr.Text == "=>" {
			more, err := g.implication(item, rest, partial, action)
			return true, more, err
		}
		return g.lazyEnabledOperatorAction(item, rest, partial, action)
	case "Let":
		more, err := g.letAction(item, rest, partial, action)
		return true, more, err
	case "Exists":
		more, err := g.existential(item, rest, partial, action)
		return true, more, err
	case "Forall":
		more, err := g.universal(item, rest, partial, action)
		return true, more, err
	case "If":
		more, err := g.conditional(item, rest, partial, action)
		return true, more, err
	default:
		return g.otherScopedAction(item, rest, partial, action)
	}
}

func (g *generator) otherScopedAction(item work, rest *workList, partial evaluator.State, action string) (bool, bool, error) {
	switch item.expr.Kind {
	case "FunctionApplication":
		return g.functionAction(item, rest, partial, action)
	case "Case":
		more, err := g.caseAction(item, rest, partial, action)
		return true, more, err
	case "Label":
		item.expr = item.expr.Args[0]
		more, err := g.solve(prepend(item, rest), partial, action)
		return true, more, err
	case "AngleAction", "BoxAction":
		more, err := g.stutteringAction(item, rest, partial, action)
		return true, more, err
	}
	return false, false, nil
}

func (g *generator) isActionCall(expr *parser.Expression) bool {
	if expr.Kind != "Apply" {
		return false
	}
	_, ok := g.definition(expr.Text)
	return ok
}

// workList is an immutable list of pending conjuncts; prepending shares the rest.
type workList struct {
	item work
	next *workList
}

func prepend(item work, rest *workList) *workList {
	return &workList{item: item, next: rest}
}

// prependMany returns n blank items, which the caller fills, followed by rest.
func prependMany(n int, rest *workList) (*workList, []workList) {
	if n == 0 {
		return rest, nil
	}
	nodes := make([]workList, n)
	for i := range n - 1 {
		nodes[i].next = &nodes[i+1]
	}
	nodes[n-1].next = rest
	return &nodes[0], nodes
}

// workArena lends nodes for the conjuncts of one solve call. Generators nest
// only through calls, and pending work is never retained after solve returns,
// so nodes are released in reverse order of their loans. Growing the arena
// copies its nodes, but existing nodes stay valid while they are referenced.
type workArena struct{ nodes []workList }

func (a *workArena) prepend(n int, rest *workList) (*workList, []workList, int) {
	mark := len(a.nodes)
	if n == 0 {
		return rest, nil, mark
	}
	a.nodes = slices.Grow(a.nodes, n)[:mark+n]
	nodes := a.nodes[mark:]
	for i := range n - 1 {
		nodes[i].next = &nodes[i+1]
	}
	nodes[n-1].next = rest
	return &nodes[0], nodes, mark
}

func (a *workArena) release(mark int) {
	clear(a.nodes[mark:])
	a.nodes = a.nodes[:mark]
}

// scopedTail copies items and gives those without a lexical boundary the
// current one, so they keep it after a LET or operator argument.
func (g *generator) scopedTail(items *workList) *workList {
	n := 0
	for current := items; current != nil; current = current.next {
		n++
	}
	head, nodes := prependMany(n, nil)
	for i := range nodes {
		nodes[i].item = items.item
		if !items.item.path.hasLexicalScope() {
			nodes[i].item.path = items.item.path.withScope(g.lexical)
		}
		items = items.next
	}
	return head
}

func (g *generator) expandWork(item work, expanded *parser.Expression, rest *workList, partial evaluator.State, action string) (bool, error) {
	if item.path.contains(item.expr.Text) {
		return false, fmt.Errorf("recursive action expansion: %s", item.expr.Text)
	}
	path := item.path.withName(item.expr.Text)
	previous := g.source
	if !g.initial && item.label {
		action = item.expr.Text
		g.source = g.stateDependentActionSource(item.expr)
	}
	more, err := g.solve(prepend(work{expr: expanded, path: path, label: item.label && g.source == nil, locals: item.locals}, rest), partial, action)
	g.source = previous
	if err != nil {
		addUnboundActionCallSource(err, item.expr, expanded)
	} else if !more {
		g.addInitialExpandedSource(item.expr, expanded)
	}
	return more, err
}

func (g *generator) stateDependentActionSource(expr *parser.Expression) *parser.Expression {
	if expr.Kind != "Apply" || !g.model.stateDependentCall(expr) {
		return nil
	}
	return expr
}

// stateDependentCall reports whether a call passes a state or action
// argument. That depends only on the call and the module's definitions.
func (m *model) stateDependentCall(expr *parser.Expression) bool {
	if dependent, ok := m.stateDependentCalls[expr]; ok {
		return dependent
	}
	dependent := slices.ContainsFunc(expr.Args, func(arg *parser.Expression) bool {
		return m.readsState(arg, map[string]bool{}) || m.hasAction(arg, map[string]bool{})
	})
	if len(m.stateDependentCalls) < 4096 {
		if m.stateDependentCalls == nil {
			m.stateDependentCalls = make(map[*parser.Expression]bool)
		}
		m.stateDependentCalls[expr] = dependent
	}
	return dependent
}

func (g *generator) isComplete(partial evaluator.State) bool {
	for _, name := range g.model.names {
		if _, ok := partial[name]; !ok {
			return false
		}
	}
	return true
}

func (g *generator) finishAssigned(items *workList, partial evaluator.State, action string) (bool, error) {
	for ; items != nil; items = items.next {
		ok, err := g.holdsScopedWork(items.item, partial)
		if err != nil {
			return false, err
		}
		if !ok {
			return true, nil
		}
	}
	return g.complete(partial, action)
}

func (g *generator) holdsScopedWork(item work, partial evaluator.State) (bool, error) {
	current, next := g.context(partial)
	if item.path.hasLexicalScope() {
		previous := g.lexical
		g.lexical = item.path.scope
		ok, err := g.holdsWithLocals(item.expr, current, next, item.locals)
		g.lexical = previous
		return ok, err
	}
	return g.holdsWithLocals(item.expr, current, next, item.locals)
}

// A generator run shares one partial state and restores it after each branch,
// so emitted states are copies. ENABLED only needs to know a successor exists.
func (g *generator) complete(partial evaluator.State, action string) (bool, error) {
	if g.enabled {
		return g.emit(partial, action, g.source)
	}
	if !g.allowIncomplete {
		for _, name := range g.model.names {
			if _, ok := partial[name]; !ok {
				return false, &incompleteState{state: maps.Clone(partial), action: action}
			}
		}
	}
	return g.emit(maps.Clone(partial), action, g.source)
}

func (g *generator) conjunction(item work, rest *workList, partial evaluator.State, action string) (bool, error) {
	items, nodes, mark := g.model.work.prepend(len(item.expr.Args), rest)
	for i, arg := range item.expr.Args {
		nodes[i].item = work{expr: arg, path: item.path, locals: item.locals}
	}
	more, err := g.solve(items, partial, action)
	g.model.work.release(mark)
	if err != nil {
		addEnclosingActionSource(err, item.expr)
	} else if !more {
		g.addInitialFailureSource(item.expr, false)
	}
	return more, err
}

func (g *generator) disjunction(item work, rest *workList, partial evaluator.State, action string) (bool, error) {
	for _, branch := range item.expr.Args {
		more, err := g.disjunctionBranch(item, branch, rest, partial, action)
		if err != nil || !more {
			g.addInitialFailureSource(item.expr, false)
			return more, err
		}
	}
	return true, nil
}

func (g *generator) disjunctionBranch(item work, branch *parser.Expression, rest *workList, partial evaluator.State, action string) (bool, error) {
	previous := g.source
	if item.label && !g.initial && branch.Location.Range.Start.Line > 0 {
		g.source = branch
	}
	more, err := g.solve(prepend(work{expr: branch, path: item.path, label: item.label, locals: item.locals}, rest), partial, action)
	g.source = previous
	if err != nil && item.label {
		g.incompleteAction(err, action, branch.Location.Range, len(item.expr.Args) > 1)
	}
	return more, err
}

func (g *generator) expand(expr *parser.Expression) (*parser.Expression, bool) {
	if expr.Kind != "Identifier" {
		return nil, false
	}
	if _, ok := g.model.constants[expr.Text]; ok {
		return nil, false
	}
	d, ok := g.definition(expr.Text)
	return d.Body, ok && d.Kind == "Operator" && len(d.Params) == 0
}

func isAssignment(expr *parser.Expression) bool {
	return expr.Kind == "Apply" && len(expr.Args) == 2 && (expr.Text == "=" || expr.Text == "\\in")
}

func (g *generator) assignment(expr *parser.Expression, locals map[string]value.Value) (string, string, bool) {
	if !isAssignment(expr) {
		return "", "", false
	}
	target := expr.Args[0]
	if !g.initial {
		name, ok := g.actionAssignmentTarget(target, locals)
		return name, expr.Text, ok
	}
	name, ok := g.assignable(target, map[string]bool{})
	return name, expr.Text, ok
}

func (g *generator) assignable(expr *parser.Expression, seen map[string]bool) (string, bool) {
	if expr.Kind != "Identifier" {
		return "", false
	}
	name := expr.Text
	if target, ok := g.lexical.PrimedArgumentVariable(name); ok {
		name = target
	}
	if g.model.variables[name] {
		return name, true
	}
	if seen[name] {
		return "", false
	}
	def, ok := g.definition(name)
	if !ok || def.Kind != "Operator" || len(def.Params) > 0 {
		return "", false
	}
	seen[name] = true
	return g.assignable(def.Body, seen)
}

func (g *generator) assign(name, operator string, rhs *parser.Expression, items *workList, partial evaluator.State, action string) (bool, error) {
	if _, ok := partial[name]; ok {
		return g.guard(items, partial, action)
	}
	current, next := g.context(partial)
	v, err := g.evalWithLocals(rhs, current, next, items.item.locals)
	if err != nil {
		evaluator.AddUnexpectedSource(err, items.item.expr)
		return false, err
	}
	var more bool
	if operator == "=" {
		more, err = g.assignOne(name, v, items.next, partial, action)
	} else {
		more, err = g.assignMembers(name, v, items.next, partial, action)
	}
	if g.initial && err != nil {
		addInitialAssignmentErrorSource(err, items.item.expr)
	}
	if err == nil && !more {
		g.addInitialFailureSource(items.item.expr, true)
	}
	return more, err
}

// assign only reaches assignOne for a variable missing from partial.
func (g *generator) assignOne(name string, v value.Value, rest *workList, partial evaluator.State, action string) (bool, error) {
	partial[name] = v
	more, err := g.solve(rest, partial, action)
	delete(partial, name)
	return more, err
}

func (g *generator) assignMembers(name string, set value.Value, rest *workList, partial evaluator.State, action string) (bool, error) {
	more := true
	err := set.Each(func(element value.Value) (bool, error) {
		var e error
		more, e = g.assignOne(name, element, rest, partial, action)
		return more, e
	})
	return more, err
}

func (g *generator) guard(items *workList, partial evaluator.State, action string) (bool, error) {
	current, next := g.context(partial)
	ok, err := g.holdsWithLocals(items.item.expr, current, next, items.item.locals)
	if err != nil {
		return false, err
	}
	if !ok {
		return true, nil
	}
	return g.solve(items.next, partial, action)
}
