package checker

import (
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

// Java TLC 1.7.4's TBGraphNode collects only LNState predicates, not LNBool.
// Thus a FALSE inside an otherwise constant tableau does not constrain any
// particle. Its recurrence checks are separate and still evaluate constants.
// Restrict this compatibility path to pure constant formulas with one DNF
// branch, no recurrence checks, and no state reduction. State-dependent and
// effectful formulas keep their ordinary evaluation path. Java TLC 1.8.0
// checks LNBool in the tableau.
func (m *model) applyJava17ConstantTableau(p *temporalProperty) bool {
	if !m.java17ConstantCandidate(p) {
		return false
	}
	node, err := newLiveConverter(m).property(*p)
	if err != nil {
		return false
	}
	node = liveUnary(liveNeg, node).simplify().toDNF()
	if node.kind == liveDisj || node.kind == liveBool {
		return false
	}
	pem, err := classifyLiveDisjunct(node.flattenSingleJunctions())
	if err != nil || !constantTableauPEM(pem) {
		return false
	}
	p.mode = generalTemporal
	p.general = newLTLCompiler(m, p.locals).intern(ltlTrue, nil, nil, nil, false)
	p.eventualities = nil
	return true
}

func (m *model) java17ConstantCandidate(p *temporalProperty) bool {
	if m.tlcRuntime.simulation || m.hasReduction() || p.mode == stateAlways || p.mode == actionAlways || p.mode == initialOnly {
		return false
	}
	return (&search{model: m}).pureConstantTemporalSource(p.source, p.locals)
}

func constantTableauPEM(pem livePEMFormulas) bool {
	if len(pem.temporal) == 0 || len(pem.eaAction) != 0 || len(pem.aeAction) != 0 || len(pem.aeState) != 0 {
		return false
	}
	for _, formula := range pem.temporal {
		if !booleanTableauFormula(formula) {
			return false
		}
	}
	return containsFalseTableau(pem.temporal)
}

func (s *search) pureConstantTemporalSource(expr *parser.Expression, locals map[string]value.Value) bool {
	switch expr.Kind {
	case "Always", "Eventually", "And", "Or":
		return s.pureConstantTemporalArgs(expr.Args, locals)
	case "Apply":
		if expr.Text == "~" {
			return s.pureConstantTemporalArgs(expr.Args, locals)
		}
	}
	return s.pureConstantAtom(expr, locals) && s.scalarConstantOperands(expr, locals)
}

func (s *search) pureConstantTemporalArgs(args []*parser.Expression, locals map[string]value.Value) bool {
	for _, arg := range args {
		if !s.pureConstantTemporalSource(arg, locals) {
			return false
		}
	}
	return true
}

// Constant values can contain lazily evaluated functions. Reexamining those
// while classifying a property could execute their bodies before exploration.
func (s *search) scalarConstantOperands(expr *parser.Expression, locals map[string]value.Value) bool {
	if expr.Kind == "Identifier" && !s.scalarConstantIdentifier(expr.Text, locals) {
		return false
	}
	for _, arg := range expr.Args {
		if !s.scalarConstantOperands(arg, locals) {
			return false
		}
	}
	return true
}

func (s *search) scalarConstantIdentifier(name string, locals map[string]value.Value) bool {
	if name == "TRUE" || name == "FALSE" {
		return true
	}
	v, ok := locals[name]
	if !ok {
		v, ok = s.model.constants[name]
	}
	return ok && scalarConstant(v)
}

func scalarConstant(v value.Value) bool {
	switch v.Kind() {
	case value.IntegerKind, value.BooleanKind, value.StringKind, value.ModelKind:
		return true
	default:
		return false
	}
}

func booleanTableauFormula(n *liveNode) bool {
	switch n.kind {
	case liveBool:
		return true
	case liveAll, liveEven:
		return booleanTableauFormula(n.body)
	case liveConj, liveDisj:
		for _, item := range n.items {
			if !booleanTableauFormula(item) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func containsFalseTableau(formulas []*liveNode) bool {
	for _, n := range formulas {
		if n.kind == liveBool && !n.value || n.body != nil && containsFalseTableau([]*liveNode{n.body}) || containsFalseTableau(n.items) {
			return true
		}
	}
	return false
}
