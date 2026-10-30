package checker

import "github.com/upamune/tla-go/syntax/parser"

func (a *alphaRenamer) expressionChildren(result, source *parser.Expression, scope map[string]string) {
	result.Args = a.expressions(source.Args, scope)
	result.Selectors = append([]parser.Selector(nil), source.Selectors...)
	for i := range result.Selectors {
		result.Selectors[i].Args = a.expressions(result.Selectors[i].Args, scope)
	}
	if len(result.Selectors) > 0 && result.Selectors[0].Kind == "Name" {
		if name, ok := scope[result.Selectors[0].Name]; ok {
			result.Selectors[0].Name = name
		}
	}
	result.Fields = append([]parser.Field(nil), source.Fields...)
	for i := range result.Fields {
		result.Fields[i].Value = a.expression(result.Fields[i].Value, scope)
	}
	result.Arms = append([]parser.CaseArm(nil), source.Arms...)
	for i := range result.Arms {
		result.Arms[i].Condition = a.expression(result.Arms[i].Condition, scope)
		result.Arms[i].Body = a.expression(result.Arms[i].Body, scope)
	}
	result.Updates = a.updates(source.Updates, scope)
}

func (a *alphaRenamer) expressions(exprs []*parser.Expression, scope map[string]string) []*parser.Expression {
	if exprs == nil {
		return nil
	}
	result := make([]*parser.Expression, len(exprs))
	for i, expr := range exprs {
		result[i] = a.expression(expr, scope)
	}
	return result
}

func (a *alphaRenamer) updates(updates []parser.Update, scope map[string]string) []parser.Update {
	result := append([]parser.Update(nil), updates...)
	for i := range result {
		result[i].Value = a.expression(result[i].Value, scope)
		result[i].Path = append([]parser.PathStep(nil), result[i].Path...)
		for j := range result[i].Path {
			result[i].Path[j].Args = a.expressions(result[i].Path[j].Args, scope)
		}
	}
	return result
}
