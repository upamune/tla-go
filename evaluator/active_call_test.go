package evaluator

import (
	"testing"

	"github.com/upamune/tla-go/value"
)

func TestActiveCallsCompareStates(t *testing.T) {
	zero, _ := value.Integer(0)
	one, _ := value.Integer(1)
	key := activeKey{name: "F"}
	e := evaluator{context: Context{Current: State{"x": zero}, Next: State{"x": zero}}}
	e.pushActiveCall(key)
	e.context = Context{Current: State{"x": one}, Next: State{"x": one}}
	e.pushActiveCall(key)
	e.context = Context{Current: State{"x": zero}, Next: State{"x": one}}
	seen, err := e.recursiveStateSeen(key)
	if err != nil || seen {
		t.Fatalf("different next state matched: %v, %v", seen, err)
	}
	e.context = Context{Current: State{"x": one}, Next: State{"x": one}}
	seen, err = e.recursiveStateSeen(key)
	if err != nil || !seen {
		t.Fatalf("matching later call was missed: %v, %v", seen, err)
	}
	if seen, err = e.recursiveStateSeen(activeKey{name: "G"}); err != nil || seen {
		t.Fatalf("call with another key matched: %v, %v", seen, err)
	}
	e.popActiveCall()
	seen, err = e.recursiveStateSeen(key)
	if err != nil || seen {
		t.Fatalf("popped call remained active: %v, %v", seen, err)
	}
}

func TestActiveCallsDistinguishNilAndEmptyStates(t *testing.T) {
	e := evaluator{}
	e.pushActiveCall(activeKey{name: "F"})
	e.context.Current = State{}
	seen, err := e.recursiveStateSeen(activeKey{name: "F"})
	if err != nil || seen {
		t.Fatalf("nil and empty states matched: %v, %v", seen, err)
	}
}

func TestActiveCallsCompareEquivalentSetRepresentations(t *testing.T) {
	one, _ := value.Integer(1)
	two, _ := value.Integer(2)
	three, _ := value.Integer(3)
	set, err := value.Set([]value.Value{three, one, two})
	if err != nil {
		t.Fatal(err)
	}
	e := evaluator{context: Context{Current: State{"x": value.Interval(1, 3)}}}
	e.pushActiveCall(activeKey{name: "F"})
	e.context.Current = State{"x": set}
	seen, err := e.recursiveStateSeen(activeKey{name: "F"})
	if err != nil || !seen {
		t.Fatalf("equivalent values did not match: %v, %v", seen, err)
	}
}
