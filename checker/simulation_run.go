package checker

import (
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
}

// simulate checks bounded traces with one worker and selected liveness properties.
func (m *model) simulate(traces, depth int, seed int64) (Result, error) {
	if err := m.validateSimulation(traces, depth); err != nil {
		return Result{}, err
	}
	actions, err := m.simulationActions()
	if err != nil {
		return Result{}, err
	}
	run := simulationRun{model: m, search: newSearch(m), depth: depth}
	run.worker = newSimulationWorker(m, actions, evaluator.NewJavaRandom(seed))
	run.search.result.DistinctStates = -1
	run.search.result.QueuedStates = -1
	m.tlcRuntime.started = true
	m.tlcRuntime.search = &run.search
	if result, failed, err := m.checkAssumptions(); failed || err != nil {
		return result, err
	}
	return run.explore(traces)
}

func (m *model) validateSimulation(traces, depth int) error {
	if !m.supportsSimulationLiveness() {
		return fmt.Errorf("simulation of temporal properties is not yet supported")
	}
	if m.graph != nil || m.debug != nil {
		return fmt.Errorf("graph and liveness debug output are not supported in simulation mode")
	}
	if traces <= 0 || depth < 0 {
		return fmt.Errorf("simulation requires a positive trace count and nonnegative depth")
	}
	return nil
}

func (r *simulationRun) explore(traces int) (Result, error) {
	states, result, err := r.initialStates()
	if result != nil {
		return *result, err
	}
	if err != nil {
		return Result{}, err
	}
	for range traces {
		result, err := r.trace(states)
		if result != nil {
			return *result, err
		}
		if err != nil {
			return Result{}, err
		}
	}
	return r.search.result, nil
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
	r.inModel, err = r.search.constraints(r.model.cfg.Constraints, r.current, nil)
	if err != nil {
		return nil, err
	}
	visited := 0
	for index := 0; index < r.depth; index++ {
		visited = index + 1
		if !r.inModel {
			break
		}
		if result, err := r.step(index); err != nil || result != nil {
			return result, err
		}
		if r.deadlocked {
			break
		}
	}
	return r.checkTraceLiveness(visited)
}

func (r *simulationRun) step(index int) (*Result, error) {
	if err := r.model.ctx.Err(); err != nil {
		return nil, err
	}
	r.model.tlcRuntime.level = index + 1
	r.model.tlcRuntime.traceCurrent = index
	states, err := r.worker.nextStates(r.current)
	if err != nil {
		return nil, err
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
