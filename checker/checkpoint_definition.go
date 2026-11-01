package checker

import "github.com/upamune/tla-go/syntax/parser"

// Validate definitions before binding: binding may evaluate even unused
// operators, so resolving a name must not admit effects or recursive calls.
type checkpointExpressionValidator struct {
	model  *model
	counts map[string]int
	states map[string]uint8
}

const (
	checkpointVisiting uint8 = iota + 1
	checkpointPure
	checkpointImpure
)

func newCheckpointExpressionValidator(m *model) *checkpointExpressionValidator {
	v := &checkpointExpressionValidator{model: m, counts: map[string]int{}, states: map[string]uint8{}}
	for _, d := range m.module.Definitions {
		v.counts[d.Name]++
	}
	return v
}

func checkpointParameters(d parser.Definition) (map[string]bool, bool) {
	if d.Kind != "Operator" || d.Recursive || len(d.Bounds) != 0 {
		return nil, false
	}
	params := make(map[string]bool, len(d.Params))
	for _, param := range d.Params {
		if param.Arity != 0 || params[param.Name] {
			return nil, false
		}
		params[param.Name] = true
	}
	return params, true
}

func (v *checkpointExpressionValidator) preparationDefinition(d parser.Definition) bool {
	params, ok := checkpointParameters(d)
	if !ok || v.counts[d.Name] != 1 {
		return false
	}
	return v.definition(d.Name) || v.preparationExpression(d.Body, params)
}

func (v *checkpointExpressionValidator) definition(name string) bool {
	if v.states[name] != 0 {
		return v.states[name] == checkpointPure
	}
	d, ok := v.model.defs[name]
	params, valid := checkpointParameters(d)
	if !ok || !valid || v.counts[name] != 1 {
		return false
	}
	v.states[name] = checkpointVisiting
	if !v.expression(d.Body, params) {
		v.states[name] = checkpointImpure
		return false
	}
	v.states[name] = checkpointPure
	return true
}

func (v *checkpointExpressionValidator) expression(e *parser.Expression, params map[string]bool) bool {
	if e == nil || parallelExtraChildren(e) {
		return false
	}
	switch e.Kind {
	case "Identifier":
		return len(e.Args) == 0 && v.identifier(e, params)
	case "Apply":
		return v.apply(e, params)
	case "String":
		return len(e.Args) == 0
	default:
		return v.model.parallelExpressionKind(e) && v.arguments(e.Args, params)
	}
}

func (v *checkpointExpressionValidator) apply(e *parser.Expression, params map[string]bool) bool {
	if checkpointMembership(e) {
		return v.expression(e.Args[0], params) && v.arguments(e.Args[1].Args, params)
	}
	if d, ok := v.model.defs[e.Text]; ok && !params[e.Text] {
		return len(e.Args) == len(d.Params) && v.arguments(e.Args, params) && v.definition(e.Text)
	}
	return v.model.parallelExpressionKind(e) && v.arguments(e.Args, params)
}

func (v *checkpointExpressionValidator) identifier(e *parser.Expression, params map[string]bool) bool {
	if params[e.Text] || v.model.variables[e.Text] || e.Text == "TRUE" || e.Text == "FALSE" {
		return true
	}
	if d, ok := v.model.defs[e.Text]; ok {
		return len(d.Params) == 0 && v.definition(e.Text)
	}
	return v.model.parallelIdentifier(e) || v.model.checkpointConstantIdentifier(e)
}

func (v *checkpointExpressionValidator) arguments(args []*parser.Expression, params map[string]bool) bool {
	for _, arg := range args {
		if !v.expression(arg, params) {
			return false
		}
	}
	return true
}
