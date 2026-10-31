package checker

import (
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

// instanceActionSource is the source of a definition that SANY wraps in
// instance substitutions. TLC's Tool.getActions does not split such a
// definition, and it reports the action at the INSTANCE statement.
func (m *model) instanceActionSource(d parser.Definition) *parser.Expression {
	if !d.InstanceSubstituted {
		return nil
	}
	if source, ok := m.instanceActions[d.Name]; ok {
		return source
	}
	if m.instanceActions == nil {
		m.instanceActions = map[string]*parser.Expression{}
	}
	source := &parser.Expression{Kind: "Instance", Text: d.Name, Location: parser.Location{Range: d.InstanceRange}}
	m.instanceActions[d.Name] = source
	return source
}

// solveNext solves the next-state relation split into TLC's actions.
func (g *generator) solveNext(next *parser.Expression) (bool, error) {
	item := work{expr: next, label: true}
	if d, ok := g.model.defs[g.model.cfg.Next]; ok && d.Body == next {
		if source := g.model.instanceActionSource(d); source != nil {
			item.label, g.source = false, source
		}
	}
	return g.solve([]work{item}, evaluator.State{}, g.model.cfg.Next)
}
