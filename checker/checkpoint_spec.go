package checker

import "github.com/upamune/tla-go/syntax/parser"

// Accept only a conjunction of pure initial predicates and one stuttering
// action. This check runs before binding, which can evaluate definitions.
func (m *model) checkpointSpecification(name string) bool {
	d, ok := m.defs[name]
	if !ok || d.Kind != "Operator" || len(d.Params) != 0 {
		return false
	}
	if !m.checkpointUniqueSpec(name) {
		return false
	}
	initial, next := 0, 0
	return m.checkpointSpecParts(d.Body, &initial, &next) && initial > 0 && next == 1
}

func (m *model) checkpointUniqueSpec(name string) bool {
	count := 0
	for _, definition := range m.module.Definitions {
		if definition.Name == name {
			count++
		}
	}
	return count == 1
}

func (m *model) checkpointSpecParts(e *parser.Expression, initial, next *int) bool {
	if e == nil || parallelExtraChildren(e) {
		return false
	}
	if e.Kind == "And" {
		return m.checkpointSpecConjunction(e.Args, initial, next)
	}
	if e.Kind == "Always" {
		return m.checkpointSpecAction(e, next)
	}
	if !m.checkpointExpression(e) || m.hasAction(e, map[string]bool{}) {
		return false
	}
	*initial++
	return true
}

func (m *model) checkpointSpecConjunction(args []*parser.Expression, initial, next *int) bool {
	for _, arg := range args {
		if !m.checkpointSpecParts(arg, initial, next) {
			return false
		}
	}
	return true
}

func (m *model) checkpointSpecAction(e *parser.Expression, next *int) bool {
	if !checkpointSpecBox(e) {
		return false
	}
	box := e.Args[0]
	if box.Args[1] == nil || !validSubscript(m.module, box.Args[1]) || !m.checkpointExpression(box.Args[0]) {
		return false
	}
	*next++
	return true
}

func checkpointSpecBox(e *parser.Expression) bool {
	return len(e.Args) == 1 && e.Args[0] != nil && e.Args[0].Kind == "BoxAction" && !parallelExtraChildren(e.Args[0]) && len(e.Args[0].Args) == 2
}
