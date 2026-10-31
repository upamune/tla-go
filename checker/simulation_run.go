package checker

import (
	"errors"
	"fmt"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

type simulationRun struct {
	model      *model
	search     search
	worker     simulationWorker
	depth      int
	current    evaluator.State
	inModel    bool
	deadlocked bool
	options    SimulationOptions
	traceIndex int
	traceOrder []string
	properties []temporalProperty
}

// simulate checks bounded traces with one worker and selected liveness properties.
func (m *model) simulate(traces, depth int, seed int64) (Result, error) {
	return m.simulateWithOptions(SimulationOptions{Traces: traces, Depth: depth}, seed)
}

func (m *model) simulateWithOptions(options SimulationOptions, seed int64) (Result, error) {
	if err := m.validateSimulation(options.Depth); err != nil {
		return Result{}, err
	}
	actions, err := m.simulationActions()
	if err != nil {
		return Result{}, err
	}
	run := simulationRun{model: m, search: newSearch(m), depth: simulationDepth(options.Depth), options: options}
	master := evaluator.NewJavaRandom(seed)
	if options.Aril > 0 {
		master.AdvanceDoubles(uint64(options.Aril))
	}
	run.worker = newSimulationWorker(m, actions, master)
	run.search.result.DistinctStates = -1
	run.search.result.QueuedStates = -1
	m.tlcRuntime.started = true
	m.tlcRuntime.search = &run.search
	if result, failed, err := m.checkAssumptions(); failed || err != nil {
		return result, err
	}
	return run.explore(simulationTraces(options))
}

func (m *model) validateSimulation(depth int) error {
	if !m.supportsSimulationLiveness() {
		return fmt.Errorf("simulation of temporal properties is not yet supported")
	}
	if m.graph != nil || m.debug != nil {
		return fmt.Errorf("graph and liveness debug output are not supported in simulation mode")
	}
	if depth < -1 {
		return fmt.Errorf("simulation depth must be -1 or nonnegative")
	}
	return nil
}

func simulationDepth(depth int) int {
	if depth == -1 {
		return int(^uint(0) >> 1)
	}
	return depth
}

func simulationTraces(options SimulationOptions) int {
	if options.UnlimitedTraces {
		return int(^uint(0) >> 1)
	}
	return max(1, options.Traces)
}

func (r *simulationRun) explore(traces int) (Result, error) {
	states, result, err := r.initialStates()
	if result != nil {
		return *result, err
	}
	if err != nil {
		return r.initialGenerationFailure(err)
	}
	return r.exploreTraces(states, traces)
}

func (r *simulationRun) initialGenerationFailure(err error) (Result, error) {
	failure, ok := errors.AsType[*evaluator.RuntimeError](err)
	if !ok || failure.ToolCode != 2247 {
		return Result{}, err
	}
	result := r.search.result
	result.Status = EvaluationFailure
	result.Failure = &Failure{Message: failure.Message, ToolCode: 2247, ExitCode: 255, SuppressStatistics: true}
	return result, nil
}

func (r *simulationRun) exploreTraces(states []evaluator.State, traces int) (Result, error) {
	for index := range traces {
		if r.model.tlcRuntime.stopped {
			break
		}
		r.traceIndex = index
		result, err := r.trace(states)
		if r.continueTrace(result, err) {
			continue
		}
		if result != nil {
			return *result, err
		}
		if err != nil {
			return Result{}, err
		}
	}
	if r.search.result.ContinuedViolations > 0 {
		r.search.result.Status = ContinuedViolation
	}
	return r.search.result, nil
}

func (r *simulationRun) continueTrace(result *Result, err error) bool {
	return result != nil && err == nil && r.continueViolation(result)
}

func (r *simulationRun) continueViolation(result *Result) bool {
	if !r.options.Continue || result.Failure != nil || !simulationContinuable(result) {
		return false
	}
	r.search.result.ContinuedViolations++
	if r.options.OnViolation != nil {
		r.model.finishSimulationResult(result)
		r.options.OnViolation(*result)
	}
	r.search.result.Status = Success
	r.search.result.Violation = nil
	return true
}

func simulationContinuable(result *Result) bool {
	return result.Status == InvariantViolation || result.Status == PropertyViolation || result.Status == Deadlock ||
		result.Status == LivenessViolation && result.Violation != nil && result.Violation.LoopStart == nil
}

func (r *simulationRun) initialStates() ([]evaluator.State, *Result, error) {
	init, err := r.model.definition(r.model.cfg.Init)
	if err != nil {
		return nil, nil, err
	}
	var states []evaluator.State
	gen := generator{model: r.model, initial: true, allowIncomplete: true, emit: func(state evaluator.State, _ string, _ *parser.Expression) (bool, error) {
		states = append(states, state)
		return true, nil
	}}
	_, err = gen.solve([]work{{expr: init}}, evaluator.State{}, "")
	if err != nil {
		return nil, nil, err
	}
	r.search.result.GeneratedStates = len(states)
	r.model.tlcRuntime.initial = len(states)
	r.model.emitSearchEvent(SearchEvent{Kind: SimulationInitialComplete, GeneratedStates: len(states), DistinctStates: -1})
	for _, state := range states {
		if !r.complete(state) {
			result := r.search.incompleteInitialFailure(&incompleteState{state: state})
			result.Failure.ToolCode = 2106
			return nil, &result, nil
		}
		// Java Simulator checks configured invariants here; unlike the BFS
		// checker it does not evaluate implied-initial properties.
		more, err := r.search.checkViolation(state, -1, "")
		if err != nil {
			return nil, nil, err
		}
		if !more {
			r.adjustFailure(state, true)
			return nil, &r.search.result, nil
		}
	}
	if len(states) == 0 {
		result := r.search.result
		result.Status = EvaluationFailure
		result.Failure = &Failure{Message: "There is no state satisfying the initial state predicate.", ToolCode: 2118, ExitCode: 255, SuppressStatistics: true}
		return nil, &result, nil
	}
	return states, nil, nil
}

func (r *simulationRun) complete(state evaluator.State) bool {
	for _, name := range r.model.names {
		if _, ok := state[name]; !ok {
			return false
		}
	}
	return true
}

func (r *simulationRun) trace(initial []evaluator.State) (*Result, error) {
	if err := r.model.ctx.Err(); err != nil {
		return nil, err
	}
	r.current = r.worker.randomState(initial)
	r.search.nodes = []stateNode{{state: r.current, parent: -1, depth: 1}}
	r.model.tlcRuntime.level = 1
	r.model.tlcRuntime.traceCurrent = 0
	r.model.tlcRuntime.traceIndices = nil
	r.deadlocked = false
	var err error
	r.inModel, err = r.model.constraintsHold(r.model.cfg.Constraints, r.current, nil)
	if err != nil {
		return nil, err
	}
	if r.model.tlcRuntime.stopped {
		return nil, nil
	}
	return r.traceSteps()
}

func (r *simulationRun) traceSteps() (*Result, error) {
	visited := 0
	for index := 0; index < r.depth; index++ {
		visited = index + 1
		if !r.inModel {
			break
		}
		result, err := r.stepWithStop(index)
		if err != nil || result != nil || r.model.tlcRuntime.stopped {
			return result, err
		}
		if r.deadlocked {
			break
		}
	}
	return r.finishTrace(visited)
}

func (r *simulationRun) finishTrace(visited int) (*Result, error) {
	result, err := r.checkTraceLiveness(visited)
	if result != nil || err != nil || r.options.OnTrace == nil {
		return result, err
	}
	var trace []TraceStep
	if visited > 0 {
		trace = r.search.trace(visited - 1)
	}
	r.orderTrace(trace)
	return nil, r.options.OnTrace(r.traceIndex, trace)
}

func (r *simulationRun) orderTrace(trace []TraceStep) {
	if len(trace) == 0 || len(r.model.names) < 2 {
		return
	}
	if r.traceOrder == nil {
		r.traceOrder = r.model.symmetryVariableOrder()
	}
	applyTraceStepOrder(trace, r.traceOrder)
}

func (r *simulationRun) stepWithStop(index int) (*Result, error) {
	result, err := r.step(index)
	if err != nil || result != nil && result.Failure != nil {
		return result, err
	}
	// TLC 1.8 discards a safety violation reported by a stopped simulation
	// worker, while an evaluation failure still reaches the caller.
	if r.model.tlcRuntime.stopped {
		r.search.result.Status = Success
		r.search.result.Violation = nil
		return nil, nil
	}
	return result, nil
}

func (r *simulationRun) step(index int) (*Result, error) {
	if err := r.model.ctx.Err(); err != nil {
		return nil, err
	}
	r.model.tlcRuntime.level = index + 1
	r.model.tlcRuntime.traceCurrent = index
	states, err := r.worker.nextStates(r.current)
	if err != nil {
		return r.nextGenerationFailure(err, index)
	}
	if len(states) == 0 {
		return r.deadlock(index), nil
	}
	if result, err := r.checkSuccessors(states, index); err != nil || result != nil {
		return result, err
	}
	next := r.worker.randomState(states)
	r.inModel, err = r.search.satisfiesConstraints(next, index)
	if err != nil {
		return nil, err
	}
	r.search.nodes = append(r.search.nodes, stateNode{state: next, parent: index, action: r.worker.selected.name, source: r.worker.selected.expr, depth: index + 2})
	r.model.tlcRuntime.rememberTraceState(next, index+1)
	r.current = next
	return nil, nil
}

func (r *simulationRun) nextGenerationFailure(err error, index int) (*Result, error) {
	failure, ok := errors.AsType[*evaluator.RuntimeError](err)
	if !ok || failure.ToolCode != 2247 {
		return nil, err
	}
	result := r.search.result
	result.Status = EvaluationFailure
	result.Failure = &Failure{Message: failure.Message, ToolCode: 2247, ExitCode: 255, Trace: r.search.trace(index)}
	return &result, nil
}

func (r *simulationRun) deadlock(index int) *Result {
	if !r.model.cfg.CheckDeadlock {
		r.inModel = false
		r.deadlocked = true
		return nil
	}
	r.search.result.Status = Deadlock
	r.search.result.Violation = &Violation{Name: "deadlock", Trace: r.search.trace(index)}
	return &r.search.result
}

func (r *simulationRun) checkSuccessors(states []evaluator.State, index int) (*Result, error) {
	for _, state := range states {
		r.search.result.GeneratedStates++
		if !r.complete(state) {
			r.search.result.Status = EvaluationFailure
			r.search.result.Failure = &Failure{Message: "Successor state is not completely specified by the next-state action.", Trace: r.search.trace(index), ExitCode: 255, ToolCode: 2109, SuppressStatistics: true}
			return &r.search.result, nil
		}
		r.search.actionSource = r.worker.selected.expr
		r.search.candidateInModel = true
		more, err := r.search.checkNewCandidate(state, index, r.worker.selected.name)
		if err != nil {
			return nil, err
		}
		if !more {
			r.adjustFailure(state, false)
			return &r.search.result, nil
		}
	}
	return nil, nil
}

func (r *simulationRun) adjustFailure(state evaluator.State, initial bool) {
	failure := r.search.result.Failure
	if failure == nil {
		return
	}
	if initial {
		failure.ToolCode = 2102
		failure.ExitCode = 255
		failure.InitialState = state
	}
	if failure.ToolCode == 2113 {
		failure.ExitCode = 13
	}
}
