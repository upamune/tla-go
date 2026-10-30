package checker

import (
	"fmt"
	"maps"

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
	model   *model
	current evaluator.State
	initial bool
	enabled bool
	lexical *evaluator.LexicalScope
	source  *parser.Expression
	emit    func(evaluator.State, string, *parser.Expression) (bool, error)
}

func (g *generator) context(partial evaluator.State) (evaluator.State, evaluator.State) {
	if g.initial {
		return partial, nil
	}
	return g.current, partial
}

func (g *generator) solve(items []work, partial evaluator.State, action string) (bool, error) {
	if len(items) > 0 && items[0].path.hasLexicalScope() {
		previous := g.lexical
		g.lexical = items[0].path.scope
		more, err := g.solveInScope(items, partial, action)
		g.lexical = previous
		return more, err
	}
	return g.solveInScope(items, partial, action)
}

func (g *generator) solveInScope(items []work, partial evaluator.State, action string) (bool, error) {
	if err := g.model.ctx.Err(); err != nil {
		return false, err
	}
	if len(items) == 0 {
		return g.complete(partial, action)
	}
	if !g.enabled && (g.initial || len(partial) > 0) && g.isComplete(partial) {
		return g.finishAssigned(items, partial, action)
	}
	item, rest := items[0], items[1:]
	if handled, more, err := g.structural(item, rest, partial, action); handled {
		return more, err
	}
	return g.atomic(items, partial, action)
}

func (g *generator) atomic(items []work, partial evaluator.State, action string) (bool, error) {
	expr := items[0].expr
	rest := items[1:]
	if expr.Kind == "Unchanged" && !g.initial {
		return g.unchanged(expr, rest, partial, action)
	}
	if name, operator, ok := g.assignment(expr); ok {
		return g.assign(name, operator, expr.Args[1], items, partial, action)
	}
	return g.guard(items, partial, action)
}

func (g *generator) structural(item work, rest []work, partial evaluator.State, action string) (bool, bool, error) {
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

func (g *generator) scopedAction(item work, rest []work, partial evaluator.State, action string) (bool, bool, error) {
	switch item.expr.Kind {
	case "Identifier":
		return g.lazyEnabledAction(item, rest, partial, action)
	case "Apply":
		if item.expr.Text == "=>" {
			more, err := g.implication(item, rest, partial, action)
			return true, more, err
		}
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
	return false, false, nil
}

func (g *generator) otherScopedAction(item work, rest []work, partial evaluator.State, action string) (bool, bool, error) {
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

func prepend(item work, rest []work) []work {
	result := make([]work, 0, len(rest)+1)
	result = append(result, item)
	return append(result, rest...)
}

func (g *generator) expandWork(item work, expanded *parser.Expression, rest []work, partial evaluator.State, action string) (bool, error) {
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
	if err == nil && !more {
		g.addInitialExpandedSource(item.expr, expanded)
	}
	return more, err
}

func (g *generator) stateDependentActionSource(expr *parser.Expression) *parser.Expression {
	if expr.Kind != "Apply" {
		return nil
	}
	for _, arg := range expr.Args {
		if g.model.readsState(arg, map[string]bool{}) || g.model.hasAction(arg, map[string]bool{}) {
			return expr
		}
	}
	return nil
}

func (g *generator) isComplete(partial evaluator.State) bool {
	for _, name := range g.model.names {
		if _, ok := partial[name]; !ok {
			return false
		}
	}
	return true
}

func (g *generator) finishAssigned(items []work, partial evaluator.State, action string) (bool, error) {
	for _, item := range items {
		ok, err := g.holdsScopedWork(item, partial)
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

func (g *generator) complete(partial evaluator.State, action string) (bool, error) {
	if g.enabled {
		return g.emit(partial, action, g.source)
	}
	for _, name := range g.model.names {
		if _, ok := partial[name]; !ok {
			return false, &incompleteState{state: maps.Clone(partial), action: action}
		}
	}
	return g.emit(partial, action, g.source)
}

func (g *generator) conjunction(item work, rest []work, partial evaluator.State, action string) (bool, error) {
	items := make([]work, 0, len(item.expr.Args)+len(rest))
	for _, arg := range item.expr.Args {
		items = append(items, work{expr: arg, path: item.path, locals: item.locals})
	}
	items = append(items, rest...)
	more, err := g.solve(items, partial, action)
	if err != nil {
		addEnclosingActionSource(err, item.expr)
	} else if !more {
		g.addInitialFailureSource(item.expr, false)
	}
	return more, err
}

func (g *generator) disjunction(item work, rest []work, partial evaluator.State, action string) (bool, error) {
	for _, branch := range item.expr.Args {
		more, err := g.disjunctionBranch(item, branch, rest, partial, action)
		if err != nil || !more {
			g.addInitialFailureSource(item.expr, false)
			return more, err
		}
	}
	return true, nil
}

func (g *generator) disjunctionBranch(item work, branch *parser.Expression, rest []work, partial evaluator.State, action string) (bool, error) {
	previous := g.source
	if item.label && !g.initial && branch.Location.Range.Start.Line > 0 {
		g.source = branch
	}
	more, err := g.solve(prepend(work{expr: branch, path: item.path, label: item.label, locals: item.locals}, rest), maps.Clone(partial), action)
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

func (g *generator) assignment(expr *parser.Expression) (string, string, bool) {
	if !isAssignment(expr) {
		return "", "", false
	}
	target := expr.Args[0]
	if !g.initial {
		if target.Kind != "Prime" || len(target.Args) != 1 {
			return "", "", false
		}
		if name, ok := g.lexical.EnabledVariable(target); g.enabled && ok {
			return name, expr.Text, true
		}
		target = target.Args[0]
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

func (g *generator) assign(name, operator string, rhs *parser.Expression, items []work, partial evaluator.State, action string) (bool, error) {
	if _, ok := partial[name]; ok {
		return g.guard(items, partial, action)
	}
	current, next := g.context(partial)
	v, err := g.evalWithLocals(rhs, current, next, items[0].locals)
	if err != nil {
		evaluator.AddUnexpectedSource(err, items[0].expr)
		return false, err
	}
	var more bool
	if operator == "=" {
		more, err = g.assignOne(name, v, items[1:], partial, action)
	} else {
		more, err = g.assignMembers(name, v, items[1:], partial, action)
	}
	if err == nil && !more {
		g.addInitialFailureSource(items[0].expr, true)
	}
	return more, err
}

func (g *generator) assignOne(name string, v value.Value, rest []work, partial evaluator.State, action string) (bool, error) {
	next := maps.Clone(partial)
	next[name] = v
	return g.solve(rest, next, action)
}

func (g *generator) assignMembers(name string, set value.Value, rest []work, partial evaluator.State, action string) (bool, error) {
	more := true
	err := set.Each(func(element value.Value) (bool, error) {
		var e error
		more, e = g.assignOne(name, element, rest, partial, action)
		return more, e
	})
	return more, err
}

func (g *generator) guard(items []work, partial evaluator.State, action string) (bool, error) {
	current, next := g.context(partial)
	ok, err := g.holdsWithLocals(items[0].expr, current, next, items[0].locals)
	if err != nil {
		return false, err
	}
	if !ok {
		return true, nil
	}
	return g.solve(items[1:], partial, action)
}
