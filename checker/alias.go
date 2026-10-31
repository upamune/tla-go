package checker

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func (m *model) applyAlias(result *Result) {
	if result.Violation != nil && !(result.Status == Deadlock && len(result.Violation.Trace) == 1 && !result.Violation.AfterInitial) {
		result.Violation.Trace = m.aliasTrace(result.Violation.Trace, result.Violation.LoopStart)
	}
	if result.Failure != nil {
		result.Failure.Trace = m.aliasTrace(result.Failure.Trace, nil)
	}
}

func (m *model) aliasTrace(trace []TraceStep, loop *int) []TraceStep {
	if len(trace) == 0 {
		return trace
	}
	result := make([]TraceStep, len(trace))
	runtime := m.tlcRuntime
	oldLevel, oldOverride, oldAliasing := runtime.level, runtime.traceOverride, runtime.aliasEvaluating
	defer func() {
		runtime.level, runtime.traceOverride, runtime.aliasEvaluating = oldLevel, oldOverride, oldAliasing
	}()
	runtime.aliasEvaluating = true
	states := traceStates(trace)
	for i, step := range trace {
		runtime.level = i + 1
		runtime.traceOverride = states[:i+1]
		next := step.State
		if i+1 < len(trace) {
			next = trace[i+1].State
		} else if loop != nil && *loop >= 0 && *loop < len(trace) {
			next = trace[*loop].State
		}
		result[i] = step
		result[i].State, result[i].StateOrder = m.aliasState(step.State, next)
	}
	return result
}

func traceStates(trace []TraceStep) []evaluator.State {
	states := make([]evaluator.State, len(trace))
	for i, step := range trace {
		states[i] = step.State
	}
	return states
}

func (m *model) aliasState(current, next evaluator.State) (evaluator.State, []string) {
	definition, err := m.aliasDefinition()
	if err != nil {
		return aliasError(current, err.Error())
	}
	result, err := m.eval(definition.Body, current, next)
	if err != nil {
		if failure, ok := errors.AsType[*evaluator.RuntimeError](err); ok {
			return aliasError(current, failure.Message)
		}
		return aliasError(current, err.Error())
	}
	return aliasRecord(result, current, definition.Body)
}

func (m *model) aliasDefinition() (parser.Definition, error) {
	name := m.cfg.Alias
	definition, ok := m.defs[name]
	if !ok {
		return parser.Definition{}, fmt.Errorf("The alias %s specified in the configuration file\nis not defined in the specification.", name)
	}
	if definition.Kind == "Operator" && len(definition.Params) > 0 {
		return parser.Definition{}, fmt.Errorf("TLC requires alias not to take any argument, but one was given: %s", name)
	}
	if !m.readsState(definition.Body, map[string]bool{}) && !m.hasAction(definition.Body, map[string]bool{}) && !m.hasTemporal(definition.Body, map[string]bool{}) {
		return parser.Definition{}, fmt.Errorf("The alias %s cannot be a constant.", name)
	}
	return definition, nil
}

func aliasError(current evaluator.State, message string) (evaluator.State, []string) {
	result := maps.Clone(current)
	result["_ALIASEvalError"] = value.String(message)
	order := slices.Sorted(maps.Keys(current))
	return result, append(order, "_ALIASEvalError")
}

func aliasRecord(v value.Value, fallback evaluator.State, body *parser.Expression) (evaluator.State, []string) {
	if v.Kind() != value.FunctionKind && v.Kind() != value.LazyFunctionKind {
		return fallback, nil
	}
	pairs, err := v.Pairs()
	if err != nil {
		return aliasError(fallback, err.Error())
	}
	result := make(evaluator.State, len(pairs))
	for _, pair := range pairs {
		key, ok := pair.Key.Text()
		if !ok || pair.Key.Kind() != value.StringKind {
			return fallback, nil
		}
		result[key] = pair.Value
	}
	return result, aliasFieldOrder(result, body)
}

func aliasFieldOrder(state evaluator.State, body *parser.Expression) []string {
	order := slices.Sorted(maps.Keys(state))
	if body.Kind != "Record" {
		return order
	}
	order = order[:0]
	for _, field := range body.Fields {
		if _, ok := state[field.Name]; ok {
			order = append(order, field.Name)
		}
	}
	return order
}
