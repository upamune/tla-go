package checker

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"testing"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/value"
)

const scalarViewCollisionSource = `---- MODULE M ----
EXTENDS Integers, TLC
VARIABLES x,y,z
Init == x=0 /\ y=0 /\ z=0
Next == \/ x=0 /\ x'=-1130370423 /\ y'=1673453457 /\ z'=0
        \/ x#0 /\ x'=0 /\ y'=0 /\ z'=0
P == []<>(x=0)
V == <<x,y,z>>
====`

// The selected VIEW values collide under Java FP64 polynomial 0, but not 1.
// The default API and the tool-oriented option must agree on the verdict.
func TestSimulationScalarViewFingerprintCollision(t *testing.T) {
	for _, tc := range []struct {
		name string
		fp   int
		want Status
	}{
		{"collision", 0, Success},
		{"different-polynomial", 1, LivenessViolation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, omit := range []bool{false, true} {
				opts := Options{Seed: 1, Fingerprint: tc.fp, Simulation: &SimulationOptions{Traces: 1, Depth: 3, OmitLassoPropertyNames: omit}}
				result, err := CheckWithOptions(context.Background(), scalarViewCollisionSource, "INIT Init NEXT Next PROPERTY P VIEW V CHECK_DEADLOCK FALSE", opts)
				if err != nil || result.Status != tc.want {
					t.Fatalf("omit=%t: status=%s error=%v, want %s", omit, result.Status, err, tc.want)
				}
			}
		})
	}
}

func TestSimulationScalarViewFingerprintCollisionValue(t *testing.T) {
	m := prepareSimulationTest(t, scalarViewCollisionSource, "INIT Init NEXT Next PROPERTY P VIEW V")
	expr, err := m.definition("V")
	if err != nil {
		t.Fatal(err)
	}
	r := simulationRun{model: m, liveViewExpr: expr}
	zero, _ := value.Integer(0)
	x, _ := value.Integer(-1130370423)
	y, _ := value.Integer(1673453457)
	for _, state := range []evaluator.State{{"x": zero, "y": zero, "z": zero}, {"x": x, "y": y, "z": zero}} {
		got, err := r.javaViewLiveFingerprint(state)
		if err != nil || got != "b89a76409771b44d" {
			t.Fatalf("state=%v fingerprint=%s error=%v", state, got, err)
		}
	}
}

func TestSimulationScalarViewFingerprintEligibility(t *testing.T) {
	for _, tc := range []struct {
		name, view, values string
		alias, want        bool
	}{
		{"scalar", "x", "int", false, true},
		{"tuple", "<<x,y>>", "int", false, true},
		{"boolean", "<<x,y>>", "bool", false, true},
		{"view-operator", "x+0", "int", false, false},
		{"nested-tuple", "<<x,<<y>>>>", "int", false, false},
		{"string-state", "x", "string", false, false},
		{"alias", "x", "int", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS Integers\nVARIABLES x,y\nInit == x=0 /\\ y=0\nNext == UNCHANGED <<x,y>>\nV == " + tc.view + "\n===="
			m := prepareSimulationTest(t, source, "INIT Init NEXT Next VIEW V")
			if tc.alias {
				m.cfg.Alias = "Unused"
			}
			x, _ := value.Integer(0)
			if tc.values == "bool" {
				x = value.Bool(true)
			} else if tc.values == "string" {
				x = value.String("s")
			}
			run := simulationRun{model: m}
			if got := run.supportsJavaViewLiveKeys([]evaluator.State{{"x": x, "y": x}}); got != tc.want {
				t.Fatalf("eligible=%t, want %t", got, tc.want)
			}
		})
	}
}

func TestJavaViewGraphFingerprintsEverySolutionInOrder(t *testing.T) {
	x0, _ := value.Integer(0)
	x1, _ := value.Integer(1)
	states := []evaluator.State{{"x": x0}, {"x": x1}}
	var events []string
	trace := newJavaViewLiveTrace(states, true, func(state evaluator.State) (string, error) {
		n, _ := state["x"].Int()
		events = append(events, fmt.Sprintf("fingerprint %d", n))
		return fmt.Sprint(n), nil
	})
	leaf := &liveNode{kind: liveState, leaf: &liveLeaf{tag: 1, eval: func(state, _ evaluator.State) (bool, error) {
		n, _ := state["x"].Int()
		events = append(events, fmt.Sprintf("check %d", n))
		return true, nil
	}}}
	for range 2 {
		if _, err := newLiveTraceGraph(trace, &liveSolution{checkState: []*liveNode{leaf}}); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"fingerprint 0", "check 0", "fingerprint 1", "check 0", "fingerprint 0", "check 0", "fingerprint 1", "check 0"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events=%q, want %q", events, want)
	}
}

func TestSimulationScalarViewFingerprintMatchesGeneral(t *testing.T) {
	for _, view := range []string{"x", "<<x,y>>"} {
		for _, symmetry := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/symmetry=%t", view, symmetry), func(t *testing.T) {
				source := "---- MODULE M ----\nEXTENDS Integers,TLC\nCONSTANTS a,b\nVARIABLES x,y\nInit == x=0 /\\ y=0\nNext == UNCHANGED <<x,y>>\nV == " + view + "\nSym == Permutations({a,b})\n===="
				cfg := "CONSTANTS a=a b=b INIT Init NEXT Next VIEW V"
				if symmetry {
					cfg += " SYMMETRY Sym"
				}
				m := prepareSimulationTest(t, source, cfg)
				expr, err := m.definition("V")
				if err != nil {
					t.Fatal(err)
				}
				r := simulationRun{model: m, liveViewExpr: expr}
				n, _ := value.Integer(-7)
				for _, state := range []evaluator.State{{"x": n, "y": value.Bool(true)}, {"x": value.Model("b"), "y": value.Model("a")}} {
					got, err := r.javaViewLiveFingerprint(state)
					want, generalErr := m.javaFingerprint(state, m.symmetryVariableOrder())
					if err != nil || generalErr != nil || got != strconv.FormatUint(want, 16) {
						t.Fatalf("state=%v got=%s err=%v want=%x err=%v", state, got, err, want, generalErr)
					}
				}
			})
		}
	}
}
