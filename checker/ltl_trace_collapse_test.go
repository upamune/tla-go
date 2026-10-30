package checker

import (
	"reflect"
	"testing"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/value"
)

func TestCollapseGeneralTracePreservesNonconsecutiveStatesAndLoop(t *testing.T) {
	m := &model{names: []string{"x"}, livenessOrder: []temporalProperty{{name: "P", mode: generalTemporal}}}
	s := search{model: m}
	loop := 3
	violation := Violation{Name: "P", LoopStart: &loop, LoopAction: "Next"}
	for i, n := range []int64{0, 0, 1, 1, 0, 2, 2} {
		x, err := value.Integer(n)
		if err != nil {
			t.Fatal(err)
		}
		violation.Trace = append(violation.Trace, TraceStep{Action: string(rune('a' + i)), State: evaluator.State{"x": x}})
	}
	if err := s.collapseGeneralTrace(&violation); err != nil {
		t.Fatal(err)
	}
	var states []int32
	var actions []string
	for _, step := range violation.Trace {
		x, ok := step.State["x"].Int()
		if !ok {
			t.Fatalf("invalid state %+v", step.State)
		}
		states = append(states, x)
		actions = append(actions, step.Action)
	}
	if !reflect.DeepEqual(states, []int32{0, 1, 0, 2}) || !reflect.DeepEqual(actions, []string{"a", "c", "e", "f"}) || *violation.LoopStart != 1 || violation.LoopAction != "Next" {
		t.Fatalf("got states %v, actions %v, loop %d via %s", states, actions, *violation.LoopStart, violation.LoopAction)
	}
}

func TestCollapseGeneralTraceMarksStutteringAfterRepeatedState(t *testing.T) {
	m := &model{names: []string{"x"}, livenessOrder: []temporalProperty{{name: "P", mode: generalTemporal}}}
	s := search{model: m}
	loop := 0
	x, err := value.Integer(1)
	if err != nil {
		t.Fatal(err)
	}
	state := evaluator.State{"x": x}
	violation := Violation{Name: "P", LoopStart: &loop, LoopAction: "Next", Trace: []TraceStep{{State: state}, {State: state}, {State: state}}}
	if err := s.collapseGeneralTrace(&violation); err != nil {
		t.Fatal(err)
	}
	if len(violation.Trace) != 1 || *violation.LoopStart != 0 || violation.LoopAction != "Stuttering" {
		t.Fatalf("unexpected collapsed trace: %+v", violation)
	}
}
