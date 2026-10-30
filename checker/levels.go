package checker

import (
	"errors"
	"fmt"
	"slices"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

// validatePredicates rejects action and temporal expressions in state predicates
// even when there are no reachable states to evaluate.
func (m *model) validatePredicates() error {
	names := append([]string{m.cfg.Init}, m.cfg.Invariants...)
	names = append(names, m.cfg.Constraints...)
	for _, name := range names {
		expr, _ := m.definition(name)
		if m.hasAction(expr, map[string]bool{}) {
			return fmt.Errorf("%s is not a state predicate", name)
		}
	}
	return nil
}

func (m *model) hasAction(expr *parser.Expression, seen map[string]bool) bool {
	if m.isTLCDefer(expr) {
		return false
	}
	if actionKind(expr) {
		return true
	}
	if expr.Kind == "Enabled" {
		return false
	}
	if expr.Kind == "Let" {
		return m.letHasAction(expr, seen)
	}
	if action, ok := m.actionCallLevel(expr, seen); ok {
		return action
	}
	if m.definitionHasAction(expr, seen) {
		return true
	}
	return m.childrenHaveAction(expr, seen)
}

func actionKind(expr *parser.Expression) bool {
	if expr.Kind == "Apply" && expr.Text == "~>" {
		return true
	}
	switch expr.Kind {
	case "Prime", "BoxAction", "AngleAction", "Always", "Eventually", "WeakFairness", "StrongFairness":
		return true
	}
	return false
}

func (m *model) definitionHasAction(expr *parser.Expression, seen map[string]bool) bool {
	name := ""
	if expr.Kind == "Identifier" || expr.Kind == "Apply" {
		name = expr.Text
	}
	d, ok := m.defs[name]
	if !ok || seen[name] {
		return false
	}
	seen[name] = true
	defer delete(seen, name)
	return m.hasAction(d.Body, seen)
}

func (m *model) childrenHaveAction(expr *parser.Expression, seen map[string]bool) bool {
	return expressionChildrenMatch(m, expr, seen, actionAnalysis)
}

func (m *model) validateStateDependence() error {
	if len(m.names) == 0 {
		return nil
	}
	for _, name := range []string{m.cfg.Init, m.cfg.Next} {
		expr, _ := m.definition(name)
		if !m.readsState(expr, map[string]bool{}) && !m.constantFalseNext(name, expr) {
			return fmt.Errorf("%s cannot be a constant", name)
		}
	}
	return nil
}

func (m *model) constantFalseNext(name string, expr *parser.Expression) bool {
	if !m.fromSpecification || name != m.cfg.Next || m.hasAction(expr, map[string]bool{}) || m.expressionHasEffects(expr, map[string]bool{}) {
		return false
	}
	holds, err := m.holdsWithoutOutput(expr)
	return err == nil && !holds
}

func (m *model) readsState(expr *parser.Expression, seen map[string]bool) bool {
	if m.isTLCDefer(expr) {
		return false
	}
	if isTLCRuntimeRead(expr) {
		return true
	}
	if expr.Kind == "Enabled" {
		return true
	}
	if expr.Kind == "Identifier" && (m.variables[expr.Text] || m.isTLCExtState(expr)) {
		return true
	}
	if m.definitionReadsState(expr, seen) {
		return true
	}
	return m.childrenReadState(expr, seen)
}

func (m *model) definitionReadsState(expr *parser.Expression, seen map[string]bool) bool {
	if expr.Kind != "Identifier" && expr.Kind != "Apply" {
		return false
	}
	name := expr.Text
	d, ok := m.defs[name]
	if !ok || seen[name] {
		return false
	}
	seen[name] = true
	defer delete(seen, name)
	return m.readsState(d.Body, seen)
}

func (m *model) childrenReadState(expr *parser.Expression, seen map[string]bool) bool {
	return expressionChildrenMatch(m, expr, seen, stateAnalysis)
}

func (m *model) holdsWithoutOutput(expr *parser.Expression) (bool, error) {
	v, err := m.program.Evaluate(expr, evaluator.Context{Constants: m.constants, Enabled: m.enabledCallback, OnPrint: func(string) {}})
	return predicateBoolean(v, err)
}

func (m *model) validateConstantPredicates() error {
	names := append([]string(nil), m.cfg.Invariants...)
	names = append(names, m.cfg.Constraints...)
	names = append(names, m.cfg.ActionConstraints...)
	for _, name := range names {
		if err := m.validateConstantPredicate(name); err != nil {
			return err
		}
	}
	return nil
}

func (m *model) validateConstantPredicate(name string) error {
	expr, _ := m.definition(name)
	if m.readsState(expr, map[string]bool{}) || m.hasAction(expr, map[string]bool{}) {
		return nil
	}
	ok, err := m.holdsWithoutOutput(expr)
	if err != nil {
		if _, runtime := errors.AsType[*evaluator.RuntimeError](err); runtime && slices.Contains(m.cfg.Invariants, name) {
			return nil
		}
		return err
	}
	if !ok {
		return fmt.Errorf("configured predicate %s is false", name)
	}
	return nil
}

func (m *model) isTLCDefer(expr *parser.Expression) bool {
	if expr.Kind != "Apply" || expr.Text != "TLCDefer" || !slices.Contains(m.module.Extends, "TLCExt") {
		return false
	}
	_, shadowed := m.defs[expr.Text]
	return !shadowed
}

func isTLCRuntimeRead(expr *parser.Expression) bool {
	name := tlcOperatorName(expr)
	return expr.Kind == "Apply" && (name == "TLCGet" || name == "TLCSet" || name == "TLCGetOrDefault" || name == "TLCGetAndSet")
}

func (m *model) isTLCExtState(expr *parser.Expression) bool {
	if expr.Kind != "Identifier" || (expr.Text != "Trace" && expr.Text != "CounterExample") || !slices.Contains(m.module.Extends, "TLCExt") {
		return false
	}
	_, shadowed := m.defs[expr.Text]
	return !shadowed
}
