package evaluator

import (
	"testing"

	"github.com/upamune/tla-go/value"
)

func TestActiveCallsCompareStateSnapshots(t *testing.T) {
	zero, _ := value.Integer(0)
	one, _ := value.Integer(1)
	current := State{"x": zero}
	next := State{"x": zero}
	e := evaluator{context: Context{Current: current, Next: next}, active: map[string][]activeCall{}}
	e.pushActiveCall("F")
	current["x"], next["x"] = one, one
	e.pushActiveCall("F")
	current["x"] = zero
	seen, err := e.recursiveStateSeen("F")
	if err != nil || seen {
		t.Fatalf("different next state matched: %v, %v", seen, err)
	}
	current["x"] = one
	seen, err = e.recursiveStateSeen("F")
	if err != nil || !seen {
		t.Fatalf("matching later call was missed: %v, %v", seen, err)
	}
	e.popActiveCall("F")
	seen, err = e.recursiveStateSeen("F")
	if err != nil || seen {
		t.Fatalf("popped call remained active: %v, %v", seen, err)
	}
}

func TestActiveCallsDistinguishNilAndEmptyStates(t *testing.T) {
	e := evaluator{active: map[string][]activeCall{}}
	e.pushActiveCall("F")
	e.context.Current = State{}
	seen, err := e.recursiveStateSeen("F")
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
	e := evaluator{context: Context{Current: State{"x": value.Interval(1, 3)}}, active: map[string][]activeCall{}}
	e.pushActiveCall("F")
	e.context.Current = State{"x": set}
	seen, err := e.recursiveStateSeen("F")
	if err != nil || !seen {
		t.Fatalf("equivalent values did not match: %v, %v", seen, err)
	}
}
