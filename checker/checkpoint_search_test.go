package checker

import (
	"crypto/sha256"
	"reflect"
	"testing"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

var testCheckpointIdentity = checkpointIdentity{Model: sha256.Sum256([]byte("test model")), Config: sha256.Sum256([]byte("test config"))}

func prepareCheckpointTest(t *testing.T, source, cfg string) *model {
	t.Helper()
	m := prepareSimulationTest(t, source, cfg)
	m.checkpointIdentity = &testCheckpointIdentity
	return m
}

// pauseSearch runs real initial generation and serial expansions, stopping only
// after a completed expansion. It is deliberately available only to tests.
func pauseSearch(t *testing.T, m *model, limit int) *search {
	t.Helper()
	if err := m.checkpointSupported(); err != nil {
		t.Fatal(err)
	}
	s := newSearch(m)
	m.tlcRuntime.started, m.tlcRuntime.search = true, &s
	init, _ := m.definition(m.cfg.Init)
	gen := generator{model: m, initial: true, emit: func(state evaluator.State, action string, source *parser.Expression) (bool, error) {
		return s.visit(state, -1, action, source)
	}}
	more, err := gen.solve(&workList{item: work{expr: init}}, m.partialState(), "")
	if err != nil || !more || s.missingNextAction() {
		t.Fatalf("initial generation: %v %v", more, err)
	}
	s.initialComplete = true
	next, _ := m.definition(m.cfg.Next)
	for s.head < limit && s.head < len(s.nodes) {
		i := s.head
		m.tlcRuntime.level = s.nodes[i].depth
		s.result.QueuedStates--
		more, err := s.expandNode(i, next)
		if err != nil || !more {
			t.Fatalf("expansion %d: %v %v", i, more, err)
		}
		s.head++
	}
	return &s
}

func TestCheckpointSearchResume(t *testing.T) {
	for _, tc := range []struct {
		name, init, next, inv string
		pauses                []int
	}{
		{"empty", `x \in {}`, `x' = x`, `TRUE`, []int{0}},
		{"duplicates", `x = 0 /\ b = FALSE`, `(x' = x /\ b' = b) \/ (x' = x + 1 /\ b' = ~b) \/ (x' = x + 1 /\ b' = ~b)`, `x < 4`, []int{0, 1, 2, 3}},
		{"success", `x = 0`, `(x' = x) \/ (x < 3 /\ x' = x + 1)`, `x <= 3`, []int{0, 1, 3, 4}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vars := "x"
			if tc.name == "duplicates" {
				vars = "x, b"
			}
			source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLES " + vars + "\nInit == " + tc.init + "\nNext == " + tc.next + "\nInv == " + tc.inv + "\n===="
			cfg := "INIT Init NEXT Next INVARIANT Inv CHECK_DEADLOCK FALSE"
			base := prepareCheckpointTest(t, source, cfg)
			want, err := base.check()
			if err != nil {
				t.Fatal(err)
			}
			for _, limit := range tc.pauses {
				m := prepareCheckpointTest(t, source, cfg)
				s := pauseSearch(t, m, limit)
				snapshot, err := s.checkpoint(testCheckpointIdentity)
				if err != nil {
					t.Fatalf("pause %d: %v", limit, err)
				}
				data, err := checkpointEncode(snapshot)
				if err != nil {
					t.Fatal(err)
				}
				snapshot, err = checkpointDecode(data, testCheckpointIdentity)
				if err != nil {
					t.Fatal(err)
				}
				fresh := prepareCheckpointTest(t, source, cfg)
				restored, err := fresh.restoreCheckpoint(snapshot, testCheckpointIdentity)
				if err != nil {
					t.Fatal(err)
				}
				got, err := restored.explore()
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("pause %d:\ngot  %+v\nwant %+v", limit, got, want)
				}
			}
		})
	}
}

func TestCheckpointSearchRejectsInvalid(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = x + 1\nInv == x < 4\n===="
	cfg := "INIT Init NEXT Next INVARIANT Inv CHECK_DEADLOCK FALSE"
	t.Run("initial generation incomplete", func(t *testing.T) {
		m := prepareCheckpointTest(t, source, cfg)
		s := newSearch(m)
		if _, err := s.checkpoint(testCheckpointIdentity); err == nil {
			t.Fatal("accepted search before initial generation")
		}
	})
	makeSnapshot := func() checkpointSnapshot {
		t.Helper()
		s := pauseSearch(t, prepareCheckpointTest(t, source, cfg), 1)
		snap, err := s.checkpoint(testCheckpointIdentity)
		if err != nil {
			t.Fatal(err)
		}
		return snap
	}
	for _, tc := range []struct {
		name   string
		mutate func(*checkpointSnapshot)
	}{
		{"visited key", func(s *checkpointSnapshot) {
			for k := range s.Visited {
				delete(s.Visited, k)
				break
			}
			s.Visited["other"] = true
		}},
		{"duplicate state", func(s *checkpointSnapshot) { s.Nodes[1].State = s.Nodes[0].State }},
		{"missing variable", func(s *checkpointSnapshot) { delete(s.Nodes[0].State, "x") }},
		{"unsupported value", func(s *checkpointSnapshot) { s.Nodes[0].State["x"] = value.Model("bad") }},
		{"wrong head", func(s *checkpointSnapshot) { s.Head++ }},
		{"wrong outdegree distribution", func(s *checkpointSnapshot) { s.Counters.OutDegrees[1]--; s.Counters.OutDegrees[0]++ }},
		{"initial generation incomplete", func(s *checkpointSnapshot) { s.InitialComplete = false }},
		{"wrong identity", func(s *checkpointSnapshot) { s.Identity.Model[0] ^= 1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snap := makeSnapshot()
			tc.mutate(&snap)
			m := prepareCheckpointTest(t, source, cfg)
			if _, err := m.restoreCheckpoint(snap, testCheckpointIdentity); err == nil {
				t.Fatal("accepted invalid snapshot")
			}
		})
	}
	t.Run("used model", func(t *testing.T) {
		m := prepareCheckpointTest(t, source, cfg)
		pauseSearch(t, m, 0)
		if _, err := m.restoreCheckpoint(makeSnapshot(), testCheckpointIdentity); err == nil {
			t.Fatal("accepted used model")
		}
	})
	for _, tc := range []struct{ name, source, cfg string }{
		{"constraint", source, cfg + " CONSTRAINT Inv"},
		{"liveness", source, cfg + " PROPERTY Inv"},
		{"effect", "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x = 0\nNext == /\\ PrintT(x) /\\ x' = x + 1\n====", "INIT Init NEXT Next"},
		{"operator call", "---- MODULE M ----\nVARIABLE x\nInit == x = 0\nStep == x' = x\nNext == Step\n====", "INIT Init NEXT Next"},
		{"set-valued successor", "---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == x' = {1}\n====", "INIT Init NEXT Next"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := prepareCheckpointTest(t, tc.source, tc.cfg)
			if err := m.checkpointSupported(); err == nil {
				t.Fatal("accepted unsupported model")
			}
		})
	}
}
