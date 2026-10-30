package evaluator

import (
	"maps"

	"github.com/upamune/tla-go/value"
)

// A state snapshot is only compared when an active operator call has the
// same arguments. Most calls therefore avoid serializing unrelated states.
type activeCall struct {
	current State
	next    State
}

func (e *evaluator) recursiveStateSeen(key string) (bool, error) {
	for _, call := range e.active[key] {
		current, err := equalCallState(call.current, e.context.Current)
		if err != nil {
			return false, err
		}
		if current {
			next, err := equalCallState(call.next, e.context.Next)
			if err != nil || next {
				return next, err
			}
		}
	}
	return false, nil
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

func (e *evaluator) pushActiveCall(key string) {
	e.active[key] = append(e.active[key], activeCall{current: maps.Clone(e.context.Current), next: maps.Clone(e.context.Next)})
}

func (e *evaluator) popActiveCall(key string) {
	stack := e.active[key]
	if len(stack) == 1 {
		delete(e.active, key)
	} else {
		e.active[key] = stack[:len(stack)-1]
	}
}
