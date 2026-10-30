package checker

import (
	"fmt"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/value"
)

func (r *tlcRuntime) CounterExample() (value.Value, error) {
	var trace []TraceStep
	if r.counterExample != nil {
		trace = r.counterExample.Trace
	}
	states := make([]value.Value, len(trace))
	for i, step := range trace {
		state, err := r.counterExampleState(i, step)
		if err != nil {
			return value.Value{}, err
		}
		states[i] = state
	}
	actions, err := r.counterExampleActions(trace, states)
	if err != nil {
		return value.Value{}, err
	}
	stateSet, err := value.Set(states)
	if err != nil {
		return value.Value{}, err
	}
	actionSet, err := value.Set(actions)
	if err != nil {
		return value.Value{}, err
	}
	return makeRecord(map[string]value.Value{"state": stateSet, "action": actionSet})
}

func (r *tlcRuntime) counterExampleState(index int, step TraceStep) (value.Value, error) {
	record, err := r.stateRecordWithOrder(step.State, step.StateOrder)
	if err != nil {
		return value.Value{}, err
	}
	number, err := value.Integer(int64(index + 1))
	if err != nil {
		return value.Value{}, err
	}
	return value.Tuple([]value.Value{number, record})
}

func (r *tlcRuntime) stateRecord(state evaluator.State) (value.Value, error) {
	return r.stateRecordWithOrder(state, nil)
}

func (r *tlcRuntime) stateRecordWithOrder(state evaluator.State, order []string) (value.Value, error) {
	pairs := make([]value.Pair, 0, len(state))
	for name, v := range state {
		pairs = append(pairs, value.Pair{Key: value.String(name), Value: v})
	}
	return value.FunctionWithFieldOrder(pairs, order)
}

func (r *tlcRuntime) counterExampleActions(trace []TraceStep, states []value.Value) ([]value.Value, error) {
	var actions []value.Value
	for i := 1; i < len(states); i++ {
		action, err := counterExampleAction(states[i-1], trace[i].Action, states[i])
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}
	return r.counterExampleLoop(trace, states, actions)
}

func (r *tlcRuntime) counterExampleLoop(trace []TraceStep, states, actions []value.Value) ([]value.Value, error) {
	v := r.counterExample
	if v == nil || v.LoopStart == nil || len(states) == 0 {
		return actions, nil
	}
	loop := *v.LoopStart
	if loop < 0 || loop >= len(states) {
		return nil, fmt.Errorf("counterexample loop index %d out of range", loop)
	}
	name := v.LoopAction
	if loop == len(states)-1 {
		name = trace[len(trace)-1].Action
	}
	action, err := counterExampleAction(states[len(states)-1], name, states[loop])
	if err != nil {
		return nil, err
	}
	return append(actions, action), nil
}

func counterExampleAction(from value.Value, name string, to value.Value) (value.Value, error) {
	if name == "" {
		name = "UnnamedAction"
	}
	unnamed, err := unnamedAction()
	if err != nil {
		return value.Value{}, err
	}
	location, err := unnamed.Lookup(value.String("location"))
	if err != nil {
		return value.Value{}, err
	}
	action, err := makeRecord(map[string]value.Value{"name": value.String(name), "location": location})
	if err != nil {
		return value.Value{}, err
	}
	return value.Tuple([]value.Value{from, action, to})
}
