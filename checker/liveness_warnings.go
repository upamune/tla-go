package checker

import "github.com/upamune/tla-go/syntax/parser"

// Warning is a Java TLC warning.
type Warning struct {
	// Code is the Java TLC message code.
	Code    int
	Message string
}

const (
	livenessSymmetryWarning    = "Declaring symmetry during liveness checking is dangerous. It might cause TLC to miss violations of the stated liveness properties. Please check liveness without symmetry defined."
	livenessConstraintsWarning = "Declaring state or action constraints during liveness checking is dangerous: Please read section 14.3.5 on page 247 of Specifying Systems (https://lamport.azurewebsites.net/tla/book.html) and optionally the discussion at https://discuss.tlapl.us/msg00994.html for more details."
)

// emitLivenessWarnings follows Java TLC's AbstractChecker, which warns before
// checking assumptions when a model checks temporal properties.
func (m *model) emitLivenessWarnings() {
	if m.livenessWarning == nil || !m.checksImpliedTemporals() {
		return
	}
	if m.cfg.Symmetry != "" {
		m.livenessWarning(Warning{Code: 2279, Message: livenessSymmetryWarning})
	}
	if m.hasEffectiveConstraints() {
		m.livenessWarning(Warning{Code: 2284, Message: livenessConstraintsWarning})
	}
}

func (m *model) hasEffectiveConstraints() bool {
	for _, name := range append(append([]string(nil), m.cfg.Constraints...), m.cfg.ActionConstraints...) {
		if !m.trueConfigured[name] {
			return true
		}
	}
	return false
}

// checksImpliedTemporals reports whether a configured property is one of
// SpecProcessor's implied temporals rather than an initial predicate,
// invariant, or action property.
func (m *model) checksImpliedTemporals() bool {
	for _, name := range m.cfg.Properties {
		if m.trueConfigured[name] {
			continue
		}
		expr, err := m.definition(name)
		if err == nil && m.impliedTemporal(expr, map[string]bool{}) {
			return true
		}
	}
	return false
}

func (m *model) impliedTemporal(expr *parser.Expression, seen map[string]bool) bool {
	switch expr.Kind {
	case "Identifier":
		return m.impliedTemporalDefinition(expr, seen)
	case "And":
		for _, arg := range expr.Args {
			if m.impliedTemporal(arg, seen) {
				return true
			}
		}
		return false
	case "Always":
		if len(expr.Args) == 1 && expr.Args[0].Kind != "BoxAction" {
			return m.hasAction(expr.Args[0], map[string]bool{})
		}
		return false
	}
	return m.hasTemporal(expr, map[string]bool{})
}

func (m *model) impliedTemporalDefinition(expr *parser.Expression, seen map[string]bool) bool {
	definition, ok := m.defs[expr.Text]
	if !ok || definition.Kind != "Operator" || len(definition.Params) != 0 || seen[expr.Text] {
		return m.hasTemporal(expr, map[string]bool{})
	}
	seen[expr.Text] = true
	defer delete(seen, expr.Text)
	return m.impliedTemporal(definition.Body, seen)
}
