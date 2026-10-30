package checker

import (
	"testing"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func TestLiveSuccessorFingerprintUsesView(t *testing.T) {
	module, err := parser.ParseModule(`---- MODULE M ----
VARIABLES x, y
Init == /\ x = 1 /\ y = 0
Next == UNCHANGED <<x, y>>
V == x
====`)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse("INIT Init NEXT Next VIEW V")
	if err != nil {
		t.Fatal(err)
	}
	m, err := prepare(module, cfg)
	if err != nil {
		t.Fatal(err)
	}
	x, _ := value.Integer(1)
	y, _ := value.Integer(0)
	s := search{model: m, nodes: []stateNode{{state: evaluator.State{"x": x, "y": y}}}, ltlVariableOrder: []string{"x", "y"}, ltlFingerprints: map[int]uint64{}}
	set := liveSuccessorSet{search: &s}
	got, err := set.fingerprint(0)
	if err != nil {
		t.Fatal(err)
	}
	want, err := value.TLCFingerprintWithOrdering([]value.Value{x}, nil, m.compareOrderedValues)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("fingerprint = %x; want view fingerprint %x", got, want)
	}
}

func TestLiveSuccessorFingerprintUsesSymmetryRepresentative(t *testing.T) {
	a, b := value.Model("A"), value.Model("B")
	algorithm, err := value.NewTLCFingerprintAlgorithm(0)
	if err != nil {
		t.Fatal(err)
	}
	m := &model{
		fingerprintAlgorithm: algorithm,
		cfg:                  config.Config{Symmetry: "Sym"},
		names:                []string{"x"},
		symmetryOrder:        []string{"x"},
		valueRanks:           map[string]int{"A": 0, "B": 1},
		symmetry:             []permutation{{images: map[string]value.Value{"A": b, "B": a}}},
	}
	s := search{model: m, nodes: []stateNode{{state: evaluator.State{"x": b}}}, ltlVariableOrder: []string{"x"}, ltlFingerprints: map[int]uint64{}}
	set := liveSuccessorSet{search: &s}
	got, err := set.fingerprint(0)
	if err != nil {
		t.Fatal(err)
	}
	rank := func(text string) (int, bool) { n, ok := m.valueRanks[text]; return n, ok }
	want, err := value.TLCFingerprintWithOrdering([]value.Value{a}, rank, m.compareOrderedValues)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("fingerprint = %x; want representative fingerprint %x", got, want)
	}
}
