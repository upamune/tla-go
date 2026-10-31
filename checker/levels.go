package checker

import (
	"fmt"
	"slices"
	"strings"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

// validatePredicates rejects action-level expressions in state predicates.
// Temporal expressions are evaluated lazily by TLC, including in constraints.
func (m *model) validatePredicates() error {
	names := append([]string{m.cfg.Init}, m.cfg.Invariants...)
	names = append(names, m.cfg.Constraints...)
	for _, name := range names {
		expr, _ := m.definition(name)
		if m.isRawActionLevel(expr) {
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
	if expr.Kind == "Apply" && (expr.Text == "~>" || expr.Text == "-+->") {
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
	for _, item := range []struct{ kind, name string }{{"initial predicate", m.cfg.Init}, {"next state action", m.cfg.Next}} {
		if item.name == "" || m.dependsOnState(item.name) {
			continue
		}
		if !m.fromSpecification && !strings.HasPrefix(item.name, "!spec!") {
			// Java TLC replaces constant definitions by their values, so a
			// configured INIT or NEXT that reads no variable is a constant,
			// even in a model without variables.
			return &ConfigurationError{Message: fmt.Sprintf("The %s %s cannot be a constant.", item.kind, item.name), Code: 2233}
		}
		if len(m.names) > 0 {
			return fmt.Errorf("%s cannot be a constant", item.name)
		}
	}
	return nil
}

func (m *model) dependsOnState(name string) bool {
	expr, _ := m.definition(name)
	return m.readsState(expr, map[string]bool{}) || m.hasTemporal(expr, map[string]bool{}) || m.constantFalseNext(name, expr)
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
	// TLC evaluates the configured constants before reporting the first FALSE.
	type checked struct {
		kind, name string
		truth      bool
	}
	var results []checked
	for _, group := range []struct {
		kind  string
		names []string
	}{{"invariant", m.cfg.Invariants}, {"property", m.cfg.Properties}, {"constraint", m.cfg.Constraints}, {"action constraint", m.cfg.ActionConstraints}} {
		for _, name := range group.names {
			if truth, valid := m.evaluateConstantPredicate(name); valid {
				results = append(results, checked{group.kind, name, truth})
			}
		}
	}
	for _, result := range results {
		if !result.truth {
			return &ConfigurationError{Message: fmt.Sprintf("The %s of %s is equal to FALSE", result.kind, result.name), Code: 2230}
		}
		m.rememberTrueConfigured(result.name)
	}
	return nil
}

func (m *model) rememberTrueConfigured(name string) {
	if m.trueConfigured == nil {
		m.trueConfigured = make(map[string]bool)
	}
	m.trueConfigured[name] = true
}

func (m *model) evaluateConstantPredicate(name string) (bool, bool) {
	if m.startupFailures[name] {
		return false, false
	}
	expr, _ := m.definition(name)
	if m.readsState(expr, map[string]bool{}) || m.hasAction(expr, map[string]bool{}) {
		return false, false
	}
	v, err := m.eval(&parser.Expression{Kind: "Identifier", Text: name}, nil, nil)
	if err != nil {
		return false, false
	}
	return v.Bool()
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
