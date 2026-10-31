package checker

import (
	"fmt"
	"slices"
	"strings"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

// presentTraces prepares reported traces for printing: ALIAS, variable
// order, action locations, and -view.
func (m *model) presentTraces(result *Result) {
	if m.cfg.Alias != "" {
		m.applyAlias(result)
	}
	m.applyTraceOrder(result)
	m.applyActionRanges(result)
	m.applyViewStates(result)
}

// applyViewStates follows TLCStateMut.toString under TLC's -view option,
// which prints trace states through the VIEW expression.
func (m *model) applyViewStates(result *Result) {
	if !m.viewStates || m.cfg.View == "" {
		return
	}
	if result.Violation != nil {
		m.viewTrace(result.Violation.Trace)
	}
	if result.Failure != nil {
		m.viewTrace(result.Failure.Trace)
	}
}

func (m *model) viewTrace(trace []TraceStep) {
	for i := range trace {
		trace[i].View = m.stateView(trace[i].State)
	}
}

// stateView returns the empty string when the view cannot be evaluated, so
// that the state itself is printed.
func (m *model) stateView(state evaluator.State) string {
	definition, ok := m.defs[m.cfg.View]
	if !ok {
		return ""
	}
	v, err := m.eval(definition.Body, state, nil)
	if err != nil {
		return ""
	}
	if text, ok := m.viewTupleText(definition.Body, v); ok {
		return text
	}
	return value.PrettyPrint(v.String())
}

// viewTupleText follows SANY's OpApplNode.toString(IValue): a tuple with one
// element per constant and variable of the expression is printed as a
// conjunction labeled in variable order. Constants share TLC's variable
// location -1, so only one of them can label an element.
func (m *model) viewTupleText(body *parser.Expression, v value.Value) (string, bool) {
	if body.Kind == "Let" || !v.IsTuple() {
		return "", false
	}
	pairs, err := v.Pairs()
	constants, variables := m.viewParameters(body)
	if err != nil || len(constants)+len(variables) != len(pairs) {
		return "", false
	}
	labels := variables
	if len(constants) > 0 {
		labels = append([]string{constants[0]}, variables...)
	}
	var text strings.Builder
	for i, label := range labels {
		fmt.Fprintf(&text, "/\\ %s = %s\n", label, value.PrettyPrint(pairs[i].Value.String()))
	}
	return text.String(), true
}

// viewParameters returns the declared constants and variables an expression
// refers to, including through definitions, in declaration order.
func (m *model) viewParameters(expr *parser.Expression) ([]string, []string) {
	names := map[string]bool{}
	m.collectIdentifiers(expr, names, map[string]bool{})
	var constants []string
	for _, declaration := range m.module.Constants {
		if names[declaration.Name] {
			constants = append(constants, declaration.Name)
		}
	}
	variables := slices.DeleteFunc(slices.Clone(m.symmetryVariableOrder()), func(name string) bool { return !names[name] })
	return constants, variables
}

func (m *model) collectIdentifiers(expr *parser.Expression, names, seen map[string]bool) {
	if expr == nil {
		return
	}
	if expr.Kind == "Identifier" || expr.Kind == "Apply" {
		names[expr.Text] = true
		if definition, ok := m.defs[expr.Text]; ok && !seen[expr.Text] {
			seen[expr.Text] = true
			m.collectIdentifiers(definition.Body, names, seen)
		}
	}
	walkChildren(expr, func(child *parser.Expression) { m.collectIdentifiers(child, names, seen) })
}
