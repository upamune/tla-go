package checker

import (
	"errors"

	"github.com/upamune/tla-go/evaluator"
)

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
		return r.liveException("java.lang.ArrayIndexOutOfBoundsException", "Index 0 out of bounds for length 0", nil), nil
	}
	states := make([]evaluator.State, visited)
	for i, node := range r.search.nodes[:visited] {
		states[i] = node.state
	}
	for i := range r.live {
		violation, err := r.checkLiveSolution(&r.live[i], states)
		if recovery, ok := errors.AsType[*liveRecoveryError](err); ok {
			return r.liveException("tlc2.tool.EvalException", "", recovery), nil
		}
		if err != nil || violation != nil {
			return r.liveViolationResult(violation), err
		}
	}
	return nil, nil
}

// liveException reports an exception that TLC's LiveCheck1 throws in the
// simulation worker: TLC prints it, then the whole sampled behavior. A
// recovery failure follows the part of the counterexample printed before it.
func (r *simulationRun) liveException(exception, detail string, recovery *liveRecoveryError) *Result {
	heading := "TLC threw an unexpected exception.\nThis was probably caused by an error in the spec or model.\nSee the User Output or TLC Console for clues to what happened.\nThe exception was a " + exception + "\n:"
	result := r.search.result
	result.Status = EvaluationFailure
	result.Failure = &Failure{Message: heading + " " + detail, ToolCode: 1000, ExitCode: 255, Trace: r.search.trace(len(r.search.nodes) - 1)}
	if recovery != nil {
		f := result.Failure
		f.Message += recovery.message
		f.NestedToolCode, f.NestedMessage = recovery.code, recovery.message
		f.TemporalViolation, f.TemporalTrace = true, recovery.shown
	}
	return &result
}

func (r *simulationRun) checkLiveSolution(solution *liveSolution, states []evaluator.State) (*Violation, error) {
	graph, err := newLiveTraceGraph(r.model, solution, states)
	if err != nil {
		return nil, err
	}
	scc := newLiveSCC(graph)
	if !scc.search() {
		return nil, nil
	}
	return scc.violation(r.recoverLiveStep)
}

func (r *simulationRun) liveViolationResult(violation *Violation) *Result {
	if violation == nil {
		return nil
	}
	violation.Properties = r.liveViolationNames(violation)
	violation.Name = violation.Properties[0]
	result := r.search.result
	result.Status = LivenessViolation
	result.Violation = violation
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
