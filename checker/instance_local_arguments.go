package checker

import (
	"maps"

	"github.com/upamune/tla-go/syntax/parser"
)

func (c *stateInstanceCheck) localChildren(expr *parser.Expression, scope map[string]bool) error {
	local := *c
	local.requirements = maps.Clone(c.requirements)
	local.localNames = maps.Clone(c.localNames)
	if local.localNames == nil {
		local.localNames = map[string]bool{}
	}
	for _, definition := range expr.Definitions {
		local.requirements[definition.Name] = make([]bool, len(definition.Params))
		local.localNames[definition.Name] = true
	}
	if err := local.inferScoped(expr.Definitions, scope); err != nil {
		return err
	}
	return local.expressionChildren(expr, scope)
}
