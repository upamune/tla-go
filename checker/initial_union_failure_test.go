package checker

import (
	"errors"
	"testing"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

func TestInitialUnionFailureKeyAndFingerprint(t *testing.T) {
	module, err := parser.ParseModule("---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\n====")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse("INIT Init NEXT Next")
	if err != nil {
		t.Fatal(err)
	}
	one, _ := value.Integer(1)
	base, err := value.Set([]value.Value{one})
	if err != nil {
		t.Fatal(err)
	}
	source := &parser.Expression{Kind: "Apply", Text: "UNION"}
	union, err := value.UnionWithSource(base, source)
	if err != nil {
		t.Fatal(err)
	}
	for _, fingerprint := range []bool{false, true} {
		m, err := prepare(module, cfg)
		if err != nil {
			t.Fatal(err)
		}
		m.tlcFingerprint = fingerprint
		state := evaluator.State{"x": union}
		_, err = m.stateKey(state)
		var fp *value.FingerprintError
		if !errors.As(err, &fp) || len(fp.Sources) != 1 || fp.Sources[0] != source {
			t.Fatalf("fingerprint=%t: state key error %v", fingerprint, err)
		}
		s := newSearch(m)
		result, err := s.initialGenerationError(candidateFailure(markUnionKeyFailure(err), state, -1))
		if err != nil || result.Status != EvaluationFailure || result.Failure == nil || result.Failure.ExitCode != 255 || !result.Failure.SuppressStatistics {
			t.Fatalf("fingerprint=%t: result %+v, error %v", fingerprint, result, err)
		}
	}
}

func TestInitialUnionFailureWithoutSource(t *testing.T) {
	module, err := parser.ParseModule("---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == UNCHANGED x\n====")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse("INIT Init NEXT Next")
	if err != nil {
		t.Fatal(err)
	}
	m, err := prepare(module, cfg)
	if err != nil {
		t.Fatal(err)
	}
	one, _ := value.Integer(1)
	base, _ := value.Set([]value.Value{one})
	union, _ := value.Union(base)
	state := evaluator.State{"x": union}
	_, err = m.stateKey(state)
	s := newSearch(m)
	result, err := s.initialGenerationError(candidateFailure(markUnionKeyFailure(err), state, -1))
	if err != nil || result.Failure == nil || result.Failure.ExitCode != 255 || !result.Failure.SuppressStatistics {
		t.Fatalf("result %+v, error %v", result, err)
	}
}
