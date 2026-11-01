package checker

import (
	"context"
	"reflect"
	"testing"
)

func TestFairIncrementalExcludesHigherOrderOperators(t *testing.T) {
	for _, tc := range []struct {
		name, definitions, next string
		eligible                bool
	}{
		{"first order control", "", `x' \in {0,1} /\ PrintT(x)`, true},
		{"mutable operators as arguments", `Do(F(_,_),G(_)) == F(1, G(1)+1)`, `x' \in {0,1} /\ Do(TLCSet,TLCGet) /\ PrintT(x)`, false},
		{"pure operator parameters", "Do(F(_)) == F(1)\nInc(a) == a+1", `x' \in {0,1} /\ Do(Inc) = 2 /\ PrintT(x)`, false},
		{"local operator parameters", "", `x' \in {0,1} /\ (LET Do(F(_)) == F(1) Inc(a) == a+1 IN Do(Inc)=2) /\ PrintT(x)`, false},
		{"random operator as argument", `Do(F(_)) == F({0,1})`, `x' \in {0,1} /\ Do(RandomElement) \in {0,1} /\ PrintT(x)`, false},
		{"JavaTime as identifier", "", `x' \in {0,1} /\ PrintT(JavaTime)`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x=0\n" + tc.definitions + "\nNext == " + tc.next + "\nSpec == Init /\\ [][Next]_x /\\ WF_x(Next)\nP == []<>(x=0)\n===="
			m := prepareSimulationTest(t, source, "SPECIFICATION Spec PROPERTY P")
			s := newSearch(m)
			if s.fairIncrementalOneSolution != tc.eligible {
				t.Fatalf("single-solution path eligible = %v; want %v", s.fairIncrementalOneSolution, tc.eligible)
			}
		})
	}
}

// Snapshot from the previous search path; this model's higher-order TLCSet and
// TLCGet aliases must not enable per-successor re-evaluation of mutable state.
func TestFairIncrementalHigherOrderKeepsPreviousBehavior(t *testing.T) {
	const source = `---- MODULE M ----
EXTENDS Naturals, TLC
VARIABLE x
Init == /\ TLCSet(1,0) /\ x = 0
Do(F(_,_),G(_)) == F(1, G(1) + 1)
Read(H(_)) == H(1)
Next == /\ x' \in {0,1} /\ Do(TLCSet, TLCGet) /\ Read(TLCGet) < 7 /\ PrintT(<<"tick", x, x', Read(TLCGet)>>)
Spec == Init /\ [][Next]_x /\ WF_x(Next)
P == []<>(x=0)
====`
	var output []string
	result, err := CheckWithOptions(context.Background(), source, "SPECIFICATION Spec PROPERTY P", Options{OnPrint: func(s string) { output = append(output, s) }})
	want := []string{`<<"tick", 0, 0, 1>>`, `<<"tick", 0, 1, 2>>`, `<<"tick", 0, 0, 3>>`, `<<"tick", 0, 1, 4>>`, `<<"tick", 0, 0, 5>>`, `<<"tick", 0, 1, 6>>`}
	if err != nil || result.Status != Deadlock || result.GeneratedStates != 3 || result.DistinctStates != 2 || !reflect.DeepEqual(output, want) {
		t.Fatalf("result %+v error %v output %q", result, err, output)
	}
}

func TestFairEnabledCacheDoesNotGrowPerSuccessor(t *testing.T) {
	const source = `---- MODULE M ----
EXTENDS Naturals, TLC
VARIABLE x
Init == x=0
Next == x' \in 0..15 /\ PrintT(x)
Spec == Init /\ [][Next]_x /\ WF_x(Next)
P == []<>(x>=0)
====`
	m := prepareSimulationTest(t, source, "SPECIFICATION Spec PROPERTY P")
	m.program = newEvaluatorProgram(m.module, func(string) {}, m.tlcRuntime)
	result, err := m.check()
	if err != nil || result.Status != Success || result.DistinctStates != 16 {
		t.Fatalf("result %+v, %v", result, err)
	}
	s := m.tlcRuntime.search
	if !s.fairIncrementalOneSolution {
		t.Fatal("expected scoped fairness path")
	}
	count := 0
	for key := range s.ltlValues {
		if key.formula.fairEnabled {
			if key.choice != -1 {
				t.Fatalf("cached enabledness separately for edge %d", key.choice)
			}
			count++
		}
	}
	if count != result.DistinctStates {
		t.Fatalf("cached enabled values %d, states %d", count, result.DistinctStates)
	}
}
