package checker

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/value"
)

func (r *tlcRuntime) Trace(state evaluator.State) (value.Value, error) {
	if err := r.completeTraceState(state); err != nil {
		return value.Value{}, err
	}
	states := r.behavior(state)
	records := make([]value.Value, len(states))
	for i, item := range states {
		record, err := r.stateRecord(item)
		if err != nil {
			return value.Value{}, err
		}
		records[i] = record
	}
	return value.Tuple(records)
}

func (r *tlcRuntime) completeTraceState(state evaluator.State) error {
	var missing []string
	for _, name := range r.model.names {
		if _, ok := state[name]; !ok {
			missing = append(missing, name)
		}
	}
	if state != nil && len(missing) == 0 {
		return nil
	}
	plural := ""
	if len(missing) > 1 {
		plural = "s"
	}
	return &evaluator.RuntimeError{Message: fmt.Sprintf("Attempted to apply the operator overridden by the Java method\npublic static tlc2.value.impl.TupleValue tlc2.module.TLCExt.getTrace(tlc2.tool.impl.Tool,tla2sany.semantic.ExprOrOpArgNode[],tlc2.util.Context,tlc2.tool.TLCState,tlc2.tool.TLCState,int,tlc2.tool.coverage.CostModel) throws java.io.IOException,\nbut it produced the following error:\nIn evaluating TLCExt!Trace, the state is not completely specified yet (variable%s %s undefined).", plural, strings.Join(missing, ", ")), Standard: true}
}

func (r *tlcRuntime) behavior(state evaluator.State) []evaluator.State {
	if r.aliasEvaluating {
		return r.traceOverride
	}
	if r.search == nil {
		return []evaluator.State{state}
	}
	r.initTraceIndices()
	index, found := r.traceIndices[traceStateID(state)]
	if found && traceStateID(state) != traceStateID(r.traceCandidate) {
		return r.tracePath(index)
	}
	return append(r.tracePath(r.traceCurrent), state)
}

func (r *tlcRuntime) tracePath(index int) []evaluator.State {
	if index < 0 {
		return nil
	}
	steps := r.search.trace(index)
	states := make([]evaluator.State, len(steps))
	for i, step := range steps {
		states[i] = step.State
	}
	return states
}

func (r *tlcRuntime) initTraceIndices() {
	if r.traceIndices != nil {
		return
	}
	r.traceIndices = make(map[uintptr]int, len(r.search.nodes))
	for i, node := range r.search.nodes {
		r.traceIndices[traceStateID(node.state)] = i
	}
}

func (r *tlcRuntime) rememberTraceState(state evaluator.State, index int) {
	if r.traceIndices != nil {
		r.traceIndices[traceStateID(state)] = index
	}
}

func traceStateID(state evaluator.State) uintptr {
	if state == nil {
		return 0
	}
	return reflect.ValueOf(state).Pointer()
}
