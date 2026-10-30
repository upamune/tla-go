package checker

import (
	"fmt"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/value"
)

func (m *model) holdsLTLAtom(formula *ltlFormula, current, next evaluator.State, locals map[string]value.Value) (bool, error) {
	if formula.kind == ltlAtom {
		return m.holdsProperty(formula.atom, current, locals)
	}
	context := evaluator.Context{Current: current, Next: next, Constants: m.constants, Locals: locals, Enabled: m.enabledCallback}
	v, err := m.program.Evaluate(formula.atom, context)
	if err != nil {
		return false, err
	}
	result, ok := v.Bool()
	if !ok {
		return false, fmt.Errorf("temporal action predicate must be boolean, got %s", v)
	}
	return result, nil
}
