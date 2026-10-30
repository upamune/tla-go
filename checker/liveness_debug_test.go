package checker

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

func TestLivenessDebugDoesNotChangeResult(t *testing.T) {
	const source = `---- MODULE Debug ----
EXTENDS Naturals
VARIABLE x
Init == x = 0
Next == x' = (x + 1) % 2
Spec == Init /\ [][Next]_x /\ WF_x(Next)
P == (x = 0) ~> (x = 2)
====`
	const cfg = "SPECIFICATION Spec PROPERTY P CHECK_DEADLOCK FALSE"
	plain, err := CheckWithOptions(context.Background(), source, cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	debugged, err := CheckWithOptions(context.Background(), source, cfg, Options{OnDebug: func(line string) { lines = append(lines, line) }})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plain, debugged) {
		t.Fatalf("debugging changed result: %+v vs %+v", plain, debugged)
	}
	events := map[string]bool{}
	for _, line := range lines {
		var event struct {
			Event string `json:"event"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		events[event.Event] = true
	}
	for _, name := range []string{"initial", "node", "successor-order", "fair-state", "fair-component", "fair-cycle"} {
		if !events[name] {
			t.Errorf("missing %s event", name)
		}
	}
}

func TestLivenessDebugCycleWitnesses(t *testing.T) {
	var lines []string
	m := &model{debug: func(line string) { lines = append(lines, line) }}
	f := fairSearch{search: &search{model: m}, property: temporalProperty{name: "P"}}
	edge := graphEdge{to: 0, action: "Next"}
	f.debugCycle(1, []cycleWitness{{node: 1}, {node: 2, edge: &edge}})
	want := `{"event":"fair-cycle","property":"P","start":1,"witnesses":[{"node":1},{"node":2,"to":0,"action":"Next"}]}`
	if len(lines) != 1 || lines[0] != want {
		t.Fatalf("got %q, want %s", lines, want)
	}
}

func TestLivenessDebugLeadsToComponent(t *testing.T) {
	const source = `---- MODULE Debug ----
VARIABLE x
Init == x = 0
Next == x' = 1
P == (x = 0) ~> (x = 2)
====`
	const cfg = "INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE"
	plain, err := CheckWithOptions(context.Background(), source, cfg, Options{})
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	debugged, err := CheckWithOptions(context.Background(), source, cfg, Options{OnDebug: func(line string) { lines = append(lines, line) }})
	if err != nil || !reflect.DeepEqual(plain, debugged) {
		t.Fatalf("debugging changed result: %+v vs %+v; %v", plain, debugged, err)
	}
	for _, line := range lines {
		var event struct {
			Event    string `json:"event"`
			Property string `json:"property"`
			Root     int    `json:"root"`
			Nodes    []int  `json:"nodes"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		if event.Event == "leads-to-component" {
			if event.Property != "P" || event.Root < 0 || event.Root > 1 || len(event.Nodes) == 0 {
				t.Fatalf("unexpected component: %+v", event)
			}
			return
		}
	}
	t.Fatal("missing leads-to component event")
}
