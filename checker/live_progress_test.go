package checker

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

const progressSource = `---- MODULE M ----
EXTENDS Naturals
VARIABLE x
Init == x = 0
A == /\ x < 2
     /\ x' = x + 1
B == x' = x
Next == A \/ B
====`
const progressConfig = "INIT Init\nNEXT Next\nCHECK_DEADLOCK FALSE\n"

func TestLiveProgressBlockedSuccessor(t *testing.T) {
	var p LiveProgress
	entered, release := make(chan struct{}), make(chan struct{})
	stop, observed := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(observed)
		for {
			select {
			case <-stop:
				return
			default:
				_ = p.Snapshot()
			}
		}
	}()
	done := make(chan Result, 1)
	errs := make(chan error, 1)
	go func() {
		r, err := CheckWithOptions(context.Background(), progressSource, progressConfig, Options{Coverage: true, LiveProgress: &p, OnGraph: func(e GraphEvent) error {
			if !e.Initial && e.New && e.Depth == 2 {
				close(entered)
				<-release
			}
			return nil
		}})
		done <- r
		errs <- err
	}()
	select {
	case <-entered:
	case err := <-errs:
		t.Fatalf("check exited before successor: %v", err)
	}
	want := ProgressSnapshot{GeneratedStates: 2, DistinctStates: 2, QueuedStates: 0, Depth: 2, InitialComplete: true}
	for range 1000 {
		if got := p.Snapshot(); got != want {
			t.Fatalf("blocked snapshot = %+v, want %+v", got, want)
		}
	}
	close(release)
	r := <-done
	close(stop)
	<-observed
	if err := <-errs; err != nil {
		t.Fatal(err)
	}
	got := p.Snapshot()
	if got.GeneratedStates != r.GeneratedStates || got.DistinctStates != r.DistinctStates || got.QueuedStates != r.QueuedStates || got.Depth != r.Depth {
		t.Fatalf("snapshot %+v result %+v", got, r)
	}
}

func TestLiveProgressEarlyAndCancel(t *testing.T) {
	t.Run("initial failure", func(t *testing.T) {
		var p LiveProgress
		source := "---- MODULE M ----\nVARIABLE x\nInit == x = 0\nA == x' = x\nB == x' = x\nNext == A \\/ B\nInv == x = 1\n===="
		r, err := CheckWithOptions(context.Background(), source, "INIT Init\nNEXT Next\nINVARIANT Inv\n", Options{Coverage: true, LiveProgress: &p})
		if err != nil || r.Status != InvariantViolation {
			t.Fatalf("%+v %v", r, err)
		}
		s := p.Snapshot()
		if s.InitialComplete || s.GeneratedStates != r.GeneratedStates || s.DistinctStates != r.DistinctStates {
			t.Fatalf("snapshot %+v result %+v", s, r)
		}
	})
	t.Run("successor failure", func(t *testing.T) {
		var p LiveProgress
		source := strings.Replace(progressSource, "Next == A \\/ B", "Next == A \\/ B\nInv == x = 0", 1)
		r, err := CheckWithOptions(context.Background(), source, progressConfig+"INVARIANT Inv\n", Options{Coverage: true, LiveProgress: &p})
		s := p.Snapshot()
		if err != nil || r.Status != InvariantViolation || !s.InitialComplete || s.GeneratedStates != r.GeneratedStates || s.DistinctStates != r.DistinctStates || s.QueuedStates != r.QueuedStates || s.Depth != r.Depth {
			t.Fatalf("result %+v snapshot %+v error %v", r, s, err)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		var p LiveProgress
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
		go func() {
			_, err := CheckWithOptions(ctx, progressSource, progressConfig, Options{Coverage: true, LiveProgress: &p, OnSearch: func(e SearchEvent) {
				if e.Kind == InitialComplete {
					close(entered)
					<-release
				}
			}})
			done <- err
		}()
		select {
		case <-entered:
		case err := <-done:
			t.Fatalf("check exited before initialization: %v", err)
		}
		before := p.Snapshot()
		cancel()
		close(release)
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatalf("error %v", err)
		}
		if got := p.Snapshot(); got != before || !got.InitialComplete || got.QueuedStates != 1 {
			t.Fatalf("before %+v after %+v", before, got)
		}
	})
	t.Run("empty", func(t *testing.T) {
		var p LiveProgress
		r, err := CheckWithOptions(context.Background(), "---- MODULE M ----\nASSUME TRUE\n====", "", Options{Coverage: true, LiveProgress: &p})
		if err != nil || p.Snapshot() != (ProgressSnapshot{Depth: r.Depth, InitialComplete: true}) {
			t.Fatalf("result %+v snapshot %+v error %v", r, p.Snapshot(), err)
		}
	})
}

func TestLiveProgressCounterOverflowMatchesSearch(t *testing.T) {
	var p LiveProgress
	s := search{model: &model{progress: &p}, result: Result{GeneratedStates: math.MaxInt}}
	s.countGenerated("", false)
	if p.Snapshot().GeneratedStates != s.result.GeneratedStates || s.result.GeneratedStates != math.MinInt {
		t.Fatalf("snapshot %+v result %+v", p.Snapshot(), s.result)
	}
}

func TestLiveProgressDoesNotChangeResultsOrEvents(t *testing.T) {
	var baselineEvents, observedEvents []SearchEvent
	base, err := CheckWithOptions(context.Background(), progressSource, progressConfig, Options{Coverage: true, OnSearch: func(e SearchEvent) { baselineEvents = append(baselineEvents, e) }})
	if err != nil {
		t.Fatal(err)
	}
	var p LiveProgress
	got, err := CheckWithOptions(context.Background(), progressSource, progressConfig, Options{Coverage: true, LiveProgress: &p, OnSearch: func(e SearchEvent) { observedEvents = append(observedEvents, e) }})
	if err != nil || !reflect.DeepEqual(base, got) || !reflect.DeepEqual(baselineEvents, observedEvents) {
		t.Fatalf("baseline %+v, observed %+v; events %v / %v; error %v", base, got, baselineEvents, observedEvents, err)
	}
	if err := validateCoverageMode(Options{LiveProgress: &p}); err == nil {
		t.Fatal("live progress accepted without coverage")
	}
	if err := checkpointIdentityOptionsSupported(Options{LiveProgress: &p}); err == nil {
		t.Fatal("checkpoint accepted live progress")
	}
}

func BenchmarkCoverageLiveProgress(b *testing.B) {
	for b.Loop() {
		var p LiveProgress
		r, err := CheckWithOptions(context.Background(), progressSource, progressConfig, Options{Coverage: true, LiveProgress: &p})
		if err != nil || r.Status != Success {
			b.Fatalf("%+v %v", r, err)
		}
	}
}
