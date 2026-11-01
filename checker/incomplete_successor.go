package checker

import (
	"errors"
	"fmt"
	"strings"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

// incompleteState keeps a generated partial state until TLC can report it.
type incompleteState struct {
	state    evaluator.State
	action   string
	location parser.Range
	split    bool
}

func (e *incompleteState) Error() string { return "state is not completely specified" }

func (s *search) incompleteInitialFailure(incomplete *incompleteState) Result {
	s.model.addUnassigned(incomplete.state)
	s.result.Status = EvaluationFailure
	s.result.Failure = &Failure{Message: "current state is not a legal state", InitialState: incomplete.state, ExitCode: 255, SuppressStatistics: true, IncompleteInitialState: true}
	return s.result
}

func (s *search) incompleteSuccessorFailure(incomplete *incompleteState, index int) (Result, error) {
	trace, err := s.recoverFailureTrace(index)
	if err != nil {
		return Result{}, err
	}
	step, message := s.model.incompleteSuccessor(incomplete)
	s.result.GeneratedStates++
	s.result.Status = EvaluationFailure
	s.result.Failure = &Failure{Message: message, Trace: append(trace, step), ExitCode: 75, ToolCode: 2109}
	return s.result, nil
}

// incompleteSuccessor returns the trace step and message TLC reports for a
// successor that leaves variables unassigned.
func (m *model) incompleteSuccessor(incomplete *incompleteState) (TraceStep, string) {
	missing := m.addUnassigned(incomplete.state)
	step := TraceStep{Action: incomplete.action, State: incomplete.state, ActionRange: incomplete.location}
	if step.Action == "!spec!next" {
		step.Action = "Action"
		if expr, err := m.definition(m.cfg.Next); err == nil && step.ActionRange.Start.Line == 0 {
			step.ActionRange = expr.Location.Range
		}
	}
	diagnosticAction := step.Action
	if incomplete.action == "!spec!next" {
		diagnosticAction = "UnnamedAction"
	}
	return step, m.incompleteMessage(diagnosticAction, missing, incomplete.split)
}

func (m *model) addUnassigned(state evaluator.State) []string {
	var missing []string
	for _, name := range m.symmetryVariableOrder() {
		if _, ok := state[name]; !ok {
			missing = append(missing, name)
			state[name] = value.Value{}
		}
	}
	return missing
}

func (m *model) incompleteMessage(action string, missing []string, split bool) string {
	which := "the next-state action"
	if expr, err := m.definition(m.cfg.Next); split || err == nil && m.hasSplitAction(expr, map[string]bool{}) {
		which = fmt.Sprintf("action %s of the next-state relation", action)
	}
	variable, verb := "variable", "is"
	if len(missing) != 1 {
		variable, verb = "variables", "are"
	}
	return fmt.Sprintf("Successor state is not completely specified by %s. The following %s %s not assigned: %s.", which, variable, verb, strings.Join(missing, ", "))
}

// Record action splitting only while unwinding an incomplete state; normal
// exploration does not evaluate or enumerate a domain a second time.
func (g *generator) incompleteAction(err error, action string, location parser.Range, split bool) {
	incomplete, ok := errors.AsType[*incompleteState](err)
	if !ok || g.initial {
		return
	}
	incomplete.split = incomplete.split || split
	if incomplete.action == action && incomplete.location.Start.Line == 0 {
		incomplete.location = location
	}
}

func (m *model) hasSplitAction(expr *parser.Expression, seen map[string]bool) bool {
	if expr.Kind == "Or" && len(expr.Args) > 1 {
		return true
	}
	if expr.Kind != "Identifier" && expr.Kind != "Apply" {
		return false
	}
	definition, ok := m.defs[expr.Text]
	if !ok || seen[expr.Text] {
		return false
	}
	seen[expr.Text] = true
	return m.hasSplitAction(definition.Body, seen)
}
