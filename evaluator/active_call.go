package evaluator

import (
	"strings"

	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

// activeKey identifies an operator call. arguments encodes argument values,
// operator arguments and lazy argument sources; it is empty for nullary calls.
type activeKey struct {
	scope     *scope
	name      string
	arguments string
}

// A state is only compared when an active operator call has the same key.
// Most calls therefore avoid serializing unrelated states. Evaluation never
// modifies its context states, and an active call ends with the evaluation
// that started it, so a call refers to those states instead of copying them.
type activeCall struct {
	key     activeKey
	current State
	next    State
}

func activeCallKey(d parser.Definition, args []value.Value, operators map[string]operatorRef, defined *scope, expressions []*parser.Expression, caller *scope) (activeKey, error) {
	key := activeKey{scope: defined, name: d.Name}
	if len(args) == 0 && len(expressions) == 0 && !hasOperatorParameters(d.Params) {
		return key, nil
	}
	var b strings.Builder
	if err := appendArgumentKeys(&b, args); err != nil {
		return activeKey{}, err
	}
	arguments := appendOperatorBindings(b.String(), d, operators)
	if len(expressions) > 0 {
		arguments = lazyRecursiveCallKey(arguments, expressions, caller)
	}
	key.arguments = arguments
	return key, nil
}

func (e *evaluator) recursiveStateSeen(key activeKey) (bool, error) {
	for _, call := range e.active {
		if call.key != key {
			continue
		}
		same, err := e.sameCallStates(call)
		if err != nil || same {
			return same, err
		}
	}
	return false, nil
}

func (e *evaluator) sameCallStates(call activeCall) (bool, error) {
	current, err := equalCallState(call.current, e.context.Current)
	if err != nil || !current {
		return false, err
	}
	return equalCallState(call.next, e.context.Next)
}

func equalCallState(a, b State) (bool, error) {
	if (a == nil) != (b == nil) || len(a) != len(b) {
		return false, nil
	}
	for name, left := range a {
		right, ok := b[name]
		if !ok {
			return false, nil
		}
		same, err := sameCallValue(left, right)
		if err != nil || !same {
			return same, err
		}
	}
	return true, nil
}

func sameCallValue(left, right value.Value) (bool, error) {
	if left == right {
		return true, nil
	}
	leftKey, err := left.Key()
	if err != nil {
		return false, err
	}
	rightKey, err := right.Key()
	return leftKey == rightKey, err
}

func (e *evaluator) pushActiveCall(key activeKey) {
	e.active = append(e.active, activeCall{key: key, current: e.context.Current, next: e.context.Next})
}

func (e *evaluator) popActiveCall() {
	last := len(e.active) - 1
	e.active[last] = activeCall{}
	e.active = e.active[:last]
}
