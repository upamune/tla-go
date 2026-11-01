package checker

import "github.com/upamune/tla-go/syntax/parser"

// coverageBooleanToggleSyntax identifies an assignment whose right-hand side
// is a unary operator call. The model-level check below validates the call.
func coverageBooleanToggleSyntax(e *parser.Expression) bool {
	if e == nil || e.Kind != "Apply" || e.Text != "=" || len(e.Args) != 2 {
		return false
	}
	left, right := e.Args[0], e.Args[1]
	return coverageBooleanTarget(left) && coverageBooleanCall(right)
}

func coverageBooleanTarget(e *parser.Expression) bool {
	return e.Kind == "Prime" && len(e.Args) == 1 && e.Args[0].Kind == "Identifier"
}

func coverageBooleanCall(e *parser.Expression) bool {
	return e.Kind == "Apply" && len(e.Args) == 1 && e.Args[0].Kind == "Identifier"
}

// TLC's B coverage test has no nested cost entry for a simple Boolean
// operator call in an assignment. Restrict this path to a closed Boolean
// transition system: its initial state is Boolean and every action toggles
// that same variable. This excludes evaluation errors and unmodeled replay.
func (m *model) coverageBooleanToggleProgram() bool {
	if !m.coverageBooleanToggleConfig() {
		return false
	}
	init, next, ok := m.coverageRoots()
	if !ok || len(init.Params) != 0 || !coverageBooleanInitial(init.Body, m.names[0]) {
		return false
	}
	return m.coverageBooleanActions(next.Body.Args, m.names[0])
}

func (m *model) coverageBooleanActions(branches []*parser.Expression, name string) bool {
	var ranges []parser.Range
	for _, branch := range branches {
		if branch.Kind != "Identifier" {
			return false
		}
		d, found := m.defs[branch.Text]
		if !found || len(d.Params) != 0 || d.InstanceSubstituted || !m.coverageBooleanToggleAction(d.Body, name) {
			return false
		}
		ranges = append(ranges, d.Body.Location.Range)
	}
	return coverageSameModuleDistinct(ranges)
}

func (m *model) coverageBooleanToggleConfig() bool {
	return !m.fromSpecification && len(m.names) == 1 &&
		len(m.cfg.Invariants)+len(m.cfg.Constraints)+len(m.cfg.Constants)+len(m.cfg.Substitutions) == 0
}

func coverageBooleanInitial(e *parser.Expression, name string) bool {
	if e == nil || e.Kind != "Apply" || e.Text != "=" || len(e.Args) != 2 {
		return false
	}
	return e.Args[0].Kind == "Identifier" && e.Args[0].Text == name && coverageBooleanLiteral(e.Args[1])
}

func coverageBooleanLiteral(e *parser.Expression) bool {
	return e.Kind == "Identifier" && (e.Text == "TRUE" || e.Text == "FALSE")
}

func (m *model) coverageBooleanToggleAction(e *parser.Expression, name string) bool {
	if !coverageBooleanToggleSyntax(e) || e.Args[0].Args[0].Text != name || e.Args[1].Args[0].Text != name {
		return false
	}
	d, ok := m.defs[e.Args[1].Text]
	if !ok || !coverageBooleanOperator(d) {
		return false
	}
	body := d.Body
	return coverageBooleanNegation(body, d.Params[0].Name)
}

func coverageBooleanOperator(d parser.Definition) bool {
	return d.Kind == "Operator" && !d.Recursive && !d.InstanceSubstituted && !d.ConfigConstant &&
		d.ConfigSubstitutionTarget == "" && len(d.Params) == 1 && d.Params[0].Arity == 0
}

func coverageBooleanNegation(body *parser.Expression, name string) bool {
	return body != nil && body.Kind == "Apply" && body.Text == "~" && len(body.Args) == 1 &&
		body.Args[0].Kind == "Identifier" && body.Args[0].Text == name
}
