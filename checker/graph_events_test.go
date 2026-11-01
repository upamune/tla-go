package checker

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/value"
)

func TestGraphEventsAndErrors(t *testing.T) {
	source := `---- MODULE M ----
EXTENDS Naturals
VARIABLE x
Init == x = 0
Next == x' = (x + 1) % 4
V == x % 2
====`
	cfg := "INIT Init\nNEXT Next\nVIEW V\n"
	var events []GraphEvent
	result, err := CheckWithOptions(context.Background(), source, cfg, Options{OnGraph: func(event GraphEvent) error {
		events = append(events, event)
		return nil
	}})
	if err != nil || result.Status != Success {
		t.Fatalf("check: result %+v, error %v", result, err)
	}
	if len(events) != 3 || !events[0].Initial || !events[0].New || events[2].New {
		t.Fatalf("unexpected graph events: %+v", events)
	}
	if events[2].Fingerprint != events[0].Fingerprint || events[2].Parent != events[1].Fingerprint {
		t.Fatalf("VIEW transition does not point to representative: %+v", events)
	}
	for _, failAt := range []int{1, 2, 3} {
		marker := errors.New("graph callback failed")
		calls := 0
		_, err := CheckWithOptions(context.Background(), source, cfg, Options{OnGraph: func(GraphEvent) error {
			calls++
			if calls == failAt {
				return marker
			}
			return nil
		}})
		if !errors.Is(err, marker) {
			t.Errorf("callback %d: got error %v, want %v", failAt, err, marker)
		}
	}
}

// Java TLC fingerprints the smallest symmetry image, while preserving the
// original state for the graph label (TLCStateMut.fingerPrint, 1.7.4).
func TestGraphFingerprintSymmetry(t *testing.T) {
	a, b := value.Model("A"), value.Model("B")
	algorithm, err := value.NewTLCFingerprintAlgorithm(0)
	if err != nil {
		t.Fatal(err)
	}
	m := &model{fingerprintAlgorithm: algorithm, names: []string{"x"}, symmetryOrder: []string{"x"}, valueRanks: map[string]int{"A": 0, "B": 1}, symmetry: []permutation{{images: map[string]value.Value{"A": b, "B": a}}}}
	s := search{model: m, graphOrder: []string{"x"}}
	state := evaluator.State{"x": b}
	rank := func(text string) (int, bool) { n, ok := m.valueRanks[text]; return n, ok }
	raw, err := algorithm.Fingerprint([]value.Value{b}, rank, m.compareOrderedValues)
	if err != nil {
		t.Fatal(err)
	}
	representative, err := algorithm.Fingerprint([]value.Value{a}, rank, m.compareOrderedValues)
	if err != nil || raw == representative {
		t.Fatalf("invalid control: %x %x %v", raw, representative, err)
	}
	got, err := s.graphFingerprint(state)
	if err != nil || got != raw {
		t.Fatalf("without symmetry: %x, %v; want %x", got, err, raw)
	}
	m.cfg.Symmetry = "Sym"
	got, err = s.graphFingerprint(state)
	if err != nil || got != representative || state["x"] != b {
		t.Fatalf("with symmetry: %x, %v; want %x and unchanged label", got, err, representative)
	}
}

func BenchmarkGraphFingerprintSymmetry(b *testing.B) {
	a, z := value.Model("A"), value.Model("B")
	algorithm, _ := value.NewTLCFingerprintAlgorithm(0)
	m := &model{fingerprintAlgorithm: algorithm, cfg: config.Config{Symmetry: "Sym"}, names: []string{"x"}, symmetryOrder: []string{"x"}, valueRanks: map[string]int{"A": 0, "B": 1}, symmetry: []permutation{{images: map[string]value.Value{"A": z, "B": a}}}}
	s := search{model: m, graphOrder: []string{"x"}}
	state := evaluator.State{"x": z}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := s.graphFingerprint(state); err != nil {
			b.Fatal(err)
		}
	}
}

func TestGraphFingerprintPreservesStateCounts(t *testing.T) {
	for _, tc := range []struct {
		name, source, cfg string
		distinct          int
	}{
		{"symmetry", "testdata/symmetry/SymmetricMessages.tla", "testdata/symmetry/SymmetricMessages.cfg", 80},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, err := os.ReadFile(tc.source)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := os.ReadFile(tc.cfg)
			if err != nil {
				t.Fatal(err)
			}
			plain, err := Check(string(source), string(cfg))
			if err != nil {
				t.Fatal(err)
			}
			graphed, err := CheckWithOptions(context.Background(), string(source), string(cfg), Options{OnGraph: func(GraphEvent) error { return nil }})
			if err != nil || plain.GeneratedStates != graphed.GeneratedStates || plain.DistinctStates != tc.distinct || graphed.DistinctStates != tc.distinct {
				t.Fatalf("plain %+v, graph %+v, error %v", plain, graphed, err)
			}
		})
	}
}

func TestGraphFingerprintCollisionPreservesTLCStateCount(t *testing.T) {
	const source = `---- MODULE UnicodeCollision ----
VARIABLE s
Init == s \in {"é", "ǩ"}
Next == UNCHANGED s
====`
	cfg := "INIT Init\nNEXT Next\n"
	var initial []GraphEvent
	got, err := CheckWithOptions(context.Background(), source, cfg, Options{OnGraph: func(e GraphEvent) error {
		if e.Initial {
			initial = append(initial, e)
		}
		return nil
	}})
	if err != nil || got.DistinctStates != 1 || len(initial) != 1 {
		t.Fatalf("got %+v, initial %d, error %v", got, len(initial), err)
	}
}
