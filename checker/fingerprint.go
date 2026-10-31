package checker

import (
	"slices"
	"strconv"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

// The reference switches to TLC FP64 if a model contains a wide string. Until
// SANY model-value token ordering is implemented, retain exact keys for models
// with model values, rather than using incorrect FP64 token numbers.
func useTLCFingerprint(module *parser.Module, cfg config.Config) bool {
	wide, hasModel := configFingerprintTraits(cfg.Constants)
	if hasModel || moduleCreatesModelValues(module) {
		return false
	}
	for _, def := range module.Definitions {
		if hasWideExpr(def.Body) || hasWideBounds(def.Bounds) {
			return true
		}
	}
	return wide
}

func moduleCreatesModelValues(module *parser.Module) bool {
	for _, def := range module.Definitions {
		if createsModelValue(def.Body) {
			return true
		}
		for _, bound := range def.Bounds {
			if createsModelValue(bound.Set) {
				return true
			}
		}
	}
	return false
}

func createsModelValue(expr *parser.Expression) bool {
	if expr == nil {
		return false
	}
	if expr.Text == "TLCModelValue" {
		return true
	}
	found := false
	walkChildren(expr, func(child *parser.Expression) {
		if !found {
			found = createsModelValue(child)
		}
	})
	return found
}

func configFingerprintTraits(assignments []config.Assignment) (bool, bool) {
	wide, model := false, false
	for _, assignment := range assignments {
		wide = wide || hasWideValue(assignment.Value)
		model = model || hasModelValue(assignment.Value)
		for _, arg := range assignment.Args {
			wide = wide || hasWideValue(arg)
			model = model || hasModelValue(arg)
		}
	}
	return wide, model
}

func hasWideValue(v config.Value) bool {
	if v.Kind == "String" && hasWideText(v.Text) {
		return true
	}
	return slices.ContainsFunc(v.Elements, hasWideValue)
}

func hasModelValue(v config.Value) bool {
	if v.Kind == "ModelValue" {
		return true
	}
	return slices.ContainsFunc(v.Elements, hasModelValue)
}

func hasWideText(s string) bool {
	for _, r := range s {
		if r > 255 {
			return true
		}
	}
	return false
}

func hasWideExpr(expr *parser.Expression) bool {
	if expr == nil {
		return false
	}
	if expr.Kind == "String" && hasWideText(expr.Text) {
		return true
	}
	return hasWideChildren(expr)
}

func hasWideChildren(expr *parser.Expression) bool {
	if slices.ContainsFunc(expr.Args, hasWideExpr) {
		return true
	}
	for _, field := range expr.Fields {
		if hasWideExpr(field.Value) {
			return true
		}
	}
	for _, arm := range expr.Arms {
		if hasWideExpr(arm.Condition) || hasWideExpr(arm.Body) {
			return true
		}
	}
	if hasWideBounds(expr.Bounds) {
		return true
	}
	return hasWideDefinitionsAndUpdates(expr)
}

func hasWideBounds(bounds []parser.Bound) bool {
	for _, bound := range bounds {
		if hasWideExpr(bound.Set) {
			return true
		}
	}
	return false
}

func hasWideDefinitionsAndUpdates(expr *parser.Expression) bool {
	for _, def := range expr.Definitions {
		if hasWideExpr(def.Body) || hasWideBounds(def.Bounds) {
			return true
		}
	}
	return hasWideUpdates(expr.Updates)
}

func hasWideUpdates(updates []parser.Update) bool {
	for _, update := range updates {
		if hasWideExpr(update.Value) {
			return true
		}
		for _, step := range update.Path {
			if hasWideArgs(step.Args) {
				return true
			}
		}
	}
	return false
}

func hasWideArgs(args []*parser.Expression) bool {
	return slices.ContainsFunc(args, hasWideExpr)
}

func (m *model) fingerprintKey(state evaluator.State) (string, error) {
	values := make([]value.Value, len(m.names))
	for i, name := range m.names {
		values[i] = state[name]
	}
	fingerprint, err := m.fingerprintAlgorithm.Fingerprint(values, nil, nil)
	if err != nil {
		return "", err
	}
	return strconv.FormatUint(fingerprint, 16), nil
}

// javaFingerprint follows TLCStateMut.fingerPrint: it fingerprints the
// symmetry representative, or its VIEW value, with variables in order.
func (m *model) javaFingerprint(state evaluator.State, order []string) (uint64, error) {
	reduced, err := m.reduceState(state)
	if err != nil {
		return 0, err
	}
	return m.reducedJavaFingerprint(reduced, order)
}

func (m *model) reducedJavaFingerprint(reduced reducedState, order []string) (uint64, error) {
	values := []value.Value{reduced.view}
	if !reduced.hasView {
		values = make([]value.Value, len(order))
		for i, name := range order {
			values[i] = reduced.state[name]
		}
	}
	rank := func(text string) (int, bool) { n, ok := m.valueRanks[text]; return n, ok }
	return m.fingerprintAlgorithm.Fingerprint(values, rank, m.compareOrderedValues)
}
