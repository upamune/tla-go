package checker

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
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

func TestPureRecurrenceSkipsUnstableFairness(t *testing.T) {
	for _, tc := range []struct {
		name, next string
		cached     bool
	}{
		{"pure", `x'=(x+1)%2`, true},
		{"output", `x'=(x+1)%2 /\ PrintT(x)`, false},
		{"mutable TLC state", `x'=(x+1)%2 /\ TLCGet("level") >= 0`, false},
		{"random", `x'=RandomElement({0,1})`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x=0\nNext == " + tc.next + "\nSpec == Init /\\ [][Next]_x /\\ WF_x(Next)\nP == []<>(x=0)\n===="
			m := prepareRecurrenceScopeTest(t, source, "SPECIFICATION Spec PROPERTY P")
			if len(m.livenessOrder) != 1 || (m.livenessOrder[0].cacheAtom != nil) != tc.cached {
				t.Fatalf("liveness properties %+v; want cached %t", m.livenessOrder, tc.cached)
			}
		})
	}
}

func TestPureRecurrenceSkipsHigherOrderEffects(t *testing.T) {
	const source = "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x=0\nNext == x'=x\nDo(F(_), n) == F(n)\nP == []<>(Do(PrintT, x))\n===="
	m := prepareRecurrenceScopeTest(t, source, "INIT Init NEXT Next PROPERTY P")
	if len(m.livenessOrder) != 1 || m.livenessOrder[0].cacheAtom != nil {
		t.Fatalf("higher-order effect was admitted to recurrence caching: %+v", m.livenessOrder)
	}
	var plain, observed []string
	for _, opts := range []Options{
		{OnPrint: func(s string) { plain = append(plain, s) }},
		{OnPrint: func(s string) { observed = append(observed, s) }, OnSearch: func(SearchEvent) {}},
	} {
		if _, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next PROPERTY P", opts); err != nil {
			t.Fatal(err)
		}
	}
	if len(plain) == 0 || !reflect.DeepEqual(plain, observed) {
		t.Fatalf("effect output plain=%q observed=%q", plain, observed)
	}
}

func prepareRecurrenceScopeTest(t *testing.T, source, configuration string) *model {
	t.Helper()
	module, err := parser.ParseModule(source)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse(configuration)
	if err != nil {
		t.Fatal(err)
	}
	m, err := prepareWithSources(module, cfg, Options{}, map[string]*parser.Module{"M": module})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestPureRecurrenceSourceProvenance(t *testing.T) {
	for _, tc := range []struct {
		name, extensions, extra, next string
		substitution, missing         bool
		want                          bool
	}{
		{"pure", "Naturals, TLC", "", `x'=(x+1)%3`, false, false, true},
		{"unknown builtin", "Naturals, TLC", "", `x'=FutureBuiltin(x, 1)`, false, false, false},
		{"JSON I/O", "Naturals, Json", "", `x'=x /\ JsonSerialize("path", x)`, false, false, false},
		{"higher order LET", "Naturals, TLC", "", `LET Do(F(_)) == F(x) IN x'=Do(PrintT)`, false, false, false},
		{"instance", "Naturals, TLC", "I == INSTANCE TLC", `x'=x`, false, false, false},
		{"effectful assumption", "Naturals, TLC", "ASSUME PrintT(TRUE)", `x'=x`, false, false, false},
		{"substitution", "Naturals, TLC", "", `x'=x`, true, false, false},
		{"missing source", "Naturals, TLC", "", `x'=x`, false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS " + tc.extensions + "\nVARIABLE x\nInit == x=0\nNext == " + tc.next + "\n" + tc.extra + "\nP == []<>(x=0)\n===="
			module, err := parser.ParseModule(source)
			if err != nil {
				t.Fatal(err)
			}
			m := &model{module: module, sourceUnits: map[string]*parser.Module{"M": module}, variables: map[string]bool{"x": true}, defs: map[string]parser.Definition{}}
			for _, def := range module.Definitions {
				m.defs[def.Name] = def
			}
			if tc.substitution {
				m.sourceConfig.Substitutions = []config.Substitution{{Name: "N", Target: "Other"}}
			}
			if tc.missing {
				m.sourceUnits = nil
			}
			if got := m.pureRecurrenceSourceSafe(); got != tc.want {
				t.Fatalf("source safety = %t; want %t", got, tc.want)
			}
		})
	}
}

func TestPureRecurrenceRejectsImpureOriginalSource(t *testing.T) {
	const clean = "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x=0\nNext == x'=x\nP == []<>(x=0)\n===="
	resolved, err := parser.ParseModule(clean)
	if err != nil {
		t.Fatal(err)
	}
	original, err := parser.ParseModule(strings.Replace(clean, "VARIABLE x", "ASSUME PrintT(TRUE)\nVARIABLE x", 1))
	if err != nil {
		t.Fatal(err)
	}
	m := &model{module: resolved, sourceUnits: map[string]*parser.Module{"M": original}, variables: map[string]bool{"x": true}, defs: map[string]parser.Definition{}}
	for _, def := range resolved.Definitions {
		m.defs[def.Name] = def
	}
	if m.pureRecurrenceSourceSafe() {
		t.Fatal("source effect hidden by resolved module was accepted")
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
