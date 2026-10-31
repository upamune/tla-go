package checker

import "github.com/upamune/tla-go/syntax/parser"

// Preserve the defining module's action range after modules are linked.
func (m *model) applyActionRanges(result *Result) {
	if result.Violation != nil {
		m.applyStepActionRanges(result.Violation.Trace)
		result.Violation.LoopAction, result.Violation.LoopActionRange = m.traceAction(result.Violation.LoopAction, result.Violation.LoopActionRange)
	}
	if result.Failure != nil {
		m.applyStepActionRanges(result.Failure.Trace)
		m.applyStepActionRanges(result.Failure.TemporalTrace)
	}
}

func actionSourceRange(source *parser.Expression) parser.Range {
	if source == nil {
		return parser.Range{}
	}
	return source.Location.Range
}

func (m *model) applyStepActionRanges(trace []TraceStep) {
	for i := range trace {
		trace[i].Action, trace[i].ActionRange = m.traceAction(trace[i].Action, trace[i].ActionRange)
	}
}

func (m *model) traceAction(name string, location parser.Range) (string, parser.Range) {
	if name == "!spec!next" {
		name = "Action"
		if expr, err := m.definition(m.cfg.Next); err == nil && location.Start.Line == 0 {
			location = expr.Location.Range
		}
	}
	if location.Start.Line == 0 {
		location = m.actionRange(name)
	}
	return name, location
}

func (m *model) actionRange(name string) parser.Range {
	if name == "" || name == "Stuttering" {
		return parser.Range{}
	}
	definition, ok := m.defs[name]
	if !ok || definition.BodyRange.Start.Line == 0 {
		return parser.Range{}
	}
	return definitionActionRange(definition)
}

func definitionActionRange(definition parser.Definition) parser.Range {
	body := definition.Body
	for body != nil && body.Kind == "Let" && len(body.Args) > 0 {
		body = body.Args[0]
	}
	if body != nil && body.Location.Range.Start.Line > 0 {
		return body.Location.Range
	}
	return definition.BodyRange
}
