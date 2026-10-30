package checker

import "github.com/upamune/tla-go/syntax/parser"

func (f *fairSearch) evaluateRequired(expr *parser.Expression, action, inverted bool) error {
	if expr == nil {
		return nil
	}
	if action {
		f.requiredAction = &temporalProperty{predicate: expr, locals: f.property.locals}
		return nil
	}
	f.required = make([]bool, len(f.search.nodes))
	for i, node := range f.search.nodes {
		if err := f.search.model.ctx.Err(); err != nil {
			return err
		}
		value, err := f.search.model.holdsBaseProperty(expr, node.state, f.property.locals)
		if err != nil {
			return err
		}
		f.required[i] = value != inverted
	}
	return nil
}

func (f *fairSearch) requiredActionWitness(component []int, members []bool) (cycleWitness, bool, error) {
	for _, node := range component {
		for _, edge := range f.search.edges[node] {
			if !members[edge.to] {
				continue
			}
			ok, err := f.search.actionPropertyForEdge(*f.requiredAction, node, edge)
			if err != nil || ok {
				return cycleWitness{node: node, edge: &edge}, ok, err
			}
		}
	}
	return cycleWitness{}, false, nil
}

func (f *fairSearch) acceptsRequiredAction(component []int, members []bool) (bool, error) {
	if f.requiredAction == nil {
		return true, nil
	}
	_, ok, err := f.requiredActionWitness(component, members)
	return ok, err
}

func (f *fairSearch) requiredWitness(component []int) cycleWitness {
	for _, node := range component {
		if f.required[node] {
			return cycleWitness{node: node}
		}
	}
	panic("required cycle witness was not found")
}
