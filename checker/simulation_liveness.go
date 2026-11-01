package checker

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

// SimulationBehaviorError is a separate error raised while printing an
// already-reported simulation failure's sampled behavior.
type SimulationBehaviorError struct {
	Failure *Failure
	Cause   error
}

func (e *SimulationBehaviorError) Error() string { return e.Cause.Error() }
func (e *SimulationBehaviorError) Unwrap() error { return e.Cause }

// prepareSimulationLiveness builds LiveCheck1's solutions once per run.
func (r *simulationRun) prepareSimulationLiveness() error {
	solutions, err := r.model.liveSolutions()
	r.live = solutions
	if len(solutions) > 0 {
		r.model.emitSearchEvent(SearchEvent{Kind: SimulationLiveness, Branches: len(solutions)})
	}
	return err
}

// checkTraceLiveness is LiveCheck1.checkTrace: TLC checks each solution on
// the behavior graph of the sampled trace, in which every state can stutter.
func (r *simulationRun) checkTraceLiveness(visited int) (*Result, error) {
	if len(r.live) == 0 {
		return nil, nil
	}
	if visited == 0 {
		// TLC's LiveCheck1 reads the first state of the empty trace.
		return r.liveException(&Failure{Message: unexpectedExceptionMessage("java.lang.ArrayIndexOutOfBoundsException", "Index 0 out of bounds for length 0"), ToolCode: 1000}, nil), nil
	}
	states := make([]evaluator.State, visited)
	for i, node := range r.search.nodes[:visited] {
		states[i] = node.state
	}
	trace, err := newLiveTrace(r.model, states, r.uncachedLiveChecks())
	if err != nil {
		return r.liveFailure(err)
	}
	for i := range r.live {
		violation, err := r.checkLiveSolution(&r.live[i], trace)
		if err != nil {
			return r.liveFailure(err)
		}
		if violation != nil {
			return r.liveViolationResult(violation), nil
		}
	}
	return nil, nil
}

func (r *simulationRun) uncachedLiveChecks() bool {
	return r.options.OmitLassoPropertyNames && r.toolLivenessCompatible()
}

// liveFailure reports an exception that TLC's LiveCheck1 throws, as TLC's
// simulator does. An exception while TLC prints the counterexample follows
// the part printed before it.
func (r *simulationRun) liveFailure(err error) (*Result, error) {
	var failure *Failure
	if recovery, ok := errors.AsType[*liveRecoveryError](err); ok {
		failure = liveEvalException(recovery.code, recovery.message)
	} else if runtime, ok := asRuntimeError(err); ok {
		failure = liveEvaluationFailure(runtime)
	} else {
		return nil, err
	}
	printing, _ := errors.AsType[*livePrintError](err)
	result := r.liveException(failure, printing)
	if printing != nil && r.uncachedLiveChecks() && r.options.OnBehaviorStart != nil && r.options.OnBehaviorState != nil {
		if err := r.streamFailedBehavior(failure); err != nil {
			return nil, behaviorRecoveryError(err)
		}
		result.SimulationBehaviorStreamed = true
	}
	return result, nil
}

func behaviorRecoveryError(err error) error {
	if runtime, ok := asRuntimeError(err); ok {
		return &SimulationBehaviorError{Failure: liveEvaluationFailure(runtime), Cause: err}
	}
	return err
}

// Simulator.printBehavior recovers each action before deciding whether to
// suppress a stutter. Unlike live recovery, getState compares full states.
func (r *simulationRun) streamFailedBehavior(failure *Failure) error {
	r.options.OnBehaviorStart(failure)
	path := r.search.trace(len(r.search.nodes) - 1)
	runtime := r.model.tlcRuntime
	oldLevel := runtime.level
	defer func() { runtime.level = oldLevel }()
	return r.streamBehaviorPath(path)
}

func (r *simulationRun) streamBehaviorPath(path []TraceStep) error {
	count := 0
	for i, sampled := range path {
		step, show, err := r.behaviorPathStep(path, i, sampled)
		if err != nil {
			return err
		}
		if !show {
			continue
		}
		count++
		v := &Violation{Trace: []TraceStep{step}}
		result := Result{Violation: v}
		r.model.presentTraces(&result)
		r.options.OnBehaviorState(result.Violation.Trace[0], count)
	}
	return nil
}

func (r *simulationRun) behaviorPathStep(path []TraceStep, index int, sampled TraceStep) (TraceStep, bool, error) {
	if err := r.model.ctx.Err(); err != nil {
		return TraceStep{}, false, err
	}
	if index == 0 {
		return sampled, true, nil
	}
	r.model.tlcRuntime.level = r.search.nodes[index-1].depth
	return r.recoverBehaviorStep(path[index-1].State, sampled.State)
}

func (r *simulationRun) recoverBehaviorStep(previous, sampled evaluator.State) (TraceStep, bool, error) {
	step, found, err := r.findBehaviorAction(previous, sampled)
	if err != nil {
		return TraceStep{}, false, err
	}
	cur, err := r.model.fingerprintKey(sampled)
	if err != nil {
		return TraceStep{}, false, err
	}
	prev, err := r.model.fingerprintKey(previous)
	if err != nil {
		return TraceStep{}, false, err
	}
	if cur == prev {
		return TraceStep{}, false, nil
	}
	if !found {
		return TraceStep{}, false, fmt.Errorf("failed to recover sampled state action")
	}
	return step, true, nil
}

func (r *simulationRun) findBehaviorAction(previous, sampled evaluator.State) (TraceStep, bool, error) {
	for _, action := range r.worker.actions {
		states, err := r.worker.actionStates(action, previous)
		if err != nil {
			return TraceStep{}, false, err
		}
		for _, state := range states {
			if r.model.tlcRuntime.sameSimulationState(sampled, state) {
				return TraceStep{State: state, Action: action.name, ActionRange: actionSourceRange(action.expr)}, true, nil
			}
		}
	}
	return TraceStep{}, false, nil
}

// liveEvaluationFailure follows Simulator: TLC prints an EvalException, which
// standard modules throw, as an unexpected exception and then the statistics.
// It prints other TLC errors with their code and general errors as runtime
// exceptions, followed only by the final progress.
func liveEvaluationFailure(failure *evaluator.RuntimeError) *Failure {
	switch {
	case failure.Unexpected != nil || failure.ToolCode == 0 || failure.ToolCode == 1000:
		return &Failure{Message: unexpectedExceptionMessage(runtimeExceptionClass, failure.Message), ToolCode: 1000, SuppressSummary: true}
	case failure.Standard:
		return liveEvalException(failure.ToolCode, failure.Message)
	}
	return &Failure{Message: failure.Message, ToolCode: failure.ToolCode, SuppressSummary: true}
}

// liveEvalException is TLC's message for an EvalException, which nests the
// TLC error.
func liveEvalException(code int, message string) *Failure {
	return &Failure{Message: unexpectedExceptionMessage(evalExceptionClass, message), ToolCode: 1000, NestedToolCode: code, NestedMessage: message}
}

// liveException completes the report of an exception in the simulation
// worker: TLC prints it, then the whole sampled behavior.
func (r *simulationRun) liveException(failure *Failure, printing *livePrintError) *Result {
	failure.ExitCode, failure.Trace = 255, r.search.trace(len(r.search.nodes)-1)
	if printing != nil {
		failure.TemporalViolation, failure.TemporalTrace = true, printing.shown
	}
	result := r.search.result
	result.Status = EvaluationFailure
	result.Failure = failure
	result.SimulationCounterexampleStarted = printing != nil && r.uncachedLiveChecks() && r.options.OnCounterexampleStart != nil
	result.SimulationCounterexampleStates = printing != nil && r.uncachedLiveChecks() && r.options.OnCounterexampleStates != nil
	return &result
}

func (r *simulationRun) checkLiveSolution(solution *liveSolution, trace *liveTrace) (*Violation, error) {
	graph, err := newLiveTraceGraph(trace, solution)
	if err != nil {
		return nil, err
	}
	scc := newLiveSCC(graph)
	if !scc.search() {
		return nil, nil
	}
	if !r.uncachedLiveChecks() {
		return scc.violation(r.recoverLiveStep, nil, nil)
	}
	var onStates func([]TraceStep, int)
	if r.options.OnCounterexampleStates != nil {
		onStates = r.counterexamplePresenter()
	}
	return scc.violation(r.recoverLiveStep, r.options.OnCounterexampleStart, onStates)
}

func (r *simulationRun) counterexamplePresenter() func([]TraceStep, int) {
	var presented []TraceStep
	return func(trace []TraceStep, from int) {
		newSteps := slices.Clone(trace[from:])
		result := Result{Violation: &Violation{Trace: newSteps}}
		r.model.presentTraces(&result)
		presented = append(presented, newSteps...)
		// Previously delivered read-only prefixes remain unchanged on append.
		r.options.OnCounterexampleStates(presented[:len(presented):len(presented)], from)
	}
}

// ALIAS and effectful VIEW presentation need further Java parity work.
func (r *simulationRun) toolLivenessCompatible() bool {
	if r.model.cfg.Alias != "" {
		return false
	}
	if r.model.cfg.View == "" {
		return true
	}
	view, err := r.model.definition(r.model.cfg.View)
	return err == nil && !r.model.unsafeIncrementalExpression(view) && !r.model.viewMayUseRuntimeOperator(view, map[string]bool{})
}

// A higher-order operator can hide a dynamic read or an effect, including a
// builtin passed as an identifier rather than used in an application.
func (m *model) viewMayUseRuntimeOperator(expr *parser.Expression, seen map[string]bool) bool {
	if expr == nil {
		return false
	}
	name := tlcOperatorName(expr)
	if i := strings.LastIndexByte(name, '!'); i >= 0 {
		name = name[i+1:]
	}
	if viewRuntimeOperator(name) || m.viewOperatorConstant(expr.Text) || viewHasOperatorParameter(expr.Definitions) {
		return true
	}
	if m.viewDefinitionMayUseRuntimeOperator(expr.Text, seen) {
		return true
	}
	found := false
	walkChildren(expr, func(child *parser.Expression) { found = found || m.viewMayUseRuntimeOperator(child, seen) })
	return found
}

func (m *model) viewDefinitionMayUseRuntimeOperator(name string, seen map[string]bool) bool {
	def, ok := m.defs[name]
	if !ok || seen[name] {
		return false
	}
	if viewHasOperatorParameter([]parser.Definition{def}) {
		return true
	}
	seen[name] = true
	defer delete(seen, name)
	return m.viewMayUseRuntimeOperator(def.Body, seen)
}

func viewRuntimeOperator(name string) bool {
	switch name {
	case "TLCGet", "TLCSet", "TLCGetAndSet", "Print", "PrintT", "AssertEq", "JavaTime":
		return true
	}
	return isRandomName(name)
}

func (m *model) viewOperatorConstant(name string) bool {
	for _, c := range m.module.Constants {
		if c.Name == name && c.Arity > 0 {
			return true
		}
	}
	return false
}

func viewHasOperatorParameter(definitions []parser.Definition) bool {
	for _, definition := range definitions {
		for _, param := range definition.Params {
			if param.Arity > 0 {
				return true
			}
		}
	}
	return false
}

func (r *simulationRun) liveViolationResult(violation *Violation) *Result {
	if violation == nil {
		return nil
	}
	if !r.options.OmitLassoPropertyNames || !r.toolLivenessCompatible() || violation.LoopStart == nil {
		violation.Properties = r.liveViolationNames(violation)
		if len(violation.Properties) > 0 {
			violation.Name = violation.Properties[0]
		}
	} else {
		violation.Name, violation.Properties = "", nil
	}
	result := r.search.result
	result.Status = LivenessViolation
	result.Violation = violation
	result.SimulationCounterexampleStarted = r.uncachedLiveChecks() && r.options.OnCounterexampleStart != nil
	result.SimulationCounterexampleStates = r.uncachedLiveChecks() && r.options.OnCounterexampleStates != nil
	return &result
}

// TLC prints no property names; name those that the lasso violates, or all
// of them when LiveCheck1's evaluation differs from a replay of the lasso.
func (r *simulationRun) liveViolationNames(violation *Violation) []string {
	names, err := r.search.violatedLassoNames(violation)
	if err == nil && len(names) > 0 {
		return names
	}
	seen := map[string]bool{}
	names = nil
	for _, property := range r.model.livenessOrder {
		if !seen[property.name] {
			seen[property.name] = true
			names = append(names, property.name)
		}
	}
	return names
}

// recoverLiveStep is Tool.getState: the first initial state or, in TLC's
// action order, the first successor whose fingerprint matches. Like TLC, it
// generates the states again. TLC makes the predecessor the current state, so
// TLCGet("level") is its position in the counterexample. Initial states are
// generated with the last state that the worker sampled as the current state.
func (r *simulationRun) recoverLiveStep(printed []TraceStep, key string) (TraceStep, error) {
	runtime := r.model.tlcRuntime
	oldLevel := runtime.level
	defer func() { runtime.level = oldLevel }()
	if len(printed) == 0 {
		runtime.level = r.search.nodes[len(r.search.nodes)-1].depth
		return r.recoverLiveInitial(key)
	}
	runtime.level = len(printed)
	previous := printed[len(printed)-1].State
	for _, action := range r.worker.actions {
		states, err := r.worker.actionStates(action, previous)
		if err != nil {
			return TraceStep{}, err
		}
		state, err := r.matchingState(states, key)
		if err != nil || state != nil {
			return TraceStep{Action: action.name, State: state, ActionRange: actionSourceRange(action.expr)}, err
		}
	}
	return TraceStep{}, &liveRecoveryError{code: 2117, message: "Failed to recover the next state from its fingerprint."}
}

func (r *simulationRun) recoverLiveInitial(key string) (TraceStep, error) {
	states, err := r.generateInitialStates()
	if err != nil {
		return TraceStep{}, err
	}
	state, err := r.matchingState(states, key)
	if err == nil && state == nil {
		err = &liveRecoveryError{code: 2123, message: "Failed to recover the initial state from its fingerprint."}
	}
	return TraceStep{State: state}, err
}

func (r *simulationRun) matchingState(states []evaluator.State, key string) (evaluator.State, error) {
	for _, state := range states {
		candidate, err := r.model.stateKey(state)
		if err != nil || candidate == key {
			return state, err
		}
	}
	return nil, nil
}
