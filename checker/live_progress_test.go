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
				_ = p.CoverageSnapshot()
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

func TestLiveCoverageSnapshotDuringCheck(t *testing.T) {
	for _, initial := range []bool{true, false} {
		t.Run(map[bool]string{true: "initial", false: "successor"}[initial], func(t *testing.T) {
			var p LiveProgress
			if p.CoverageSnapshot() != nil {
				t.Fatal("coverage available before initialization")
			}
			entered, release := make(chan struct{}), make(chan struct{})
			done := make(chan Result, 1)
			errs := make(chan error, 1)
			go func() {
				blocked := false
				r, err := CheckWithOptions(context.Background(), progressSource, progressConfig, Options{Coverage: true, LiveProgress: &p, OnGraph: func(e GraphEvent) error {
					if !blocked && e.Initial == initial && (initial || e.New) {
						blocked = true
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
				t.Fatalf("check exited before graph event: %v", err)
			}
			before := p.CoverageSnapshot()
			if len(before) == 0 {
				t.Fatal("missing live coverage")
			}
			before[0].Generated = -1
			if p.CoverageSnapshot()[0].Generated == -1 {
				t.Fatal("snapshot aliases collector")
			}
			close(release)
			r := <-done
			if err := <-errs; err != nil {
				t.Fatal(err)
			}
			if got := p.CoverageSnapshot(); !reflect.DeepEqual(got, r.Coverage) {
				t.Fatalf("final snapshot %+v, result %+v", got, r.Coverage)
			}
			final := p.CoverageSnapshot()
			final[0].Generated = -1
			if !reflect.DeepEqual(p.CoverageSnapshot(), r.Coverage) {
				t.Fatal("final snapshot aliases retained coverage")
			}
		})
	}
}

func TestLiveCoverageSnapshotEarlyFailure(t *testing.T) {
	var p LiveProgress
	_, err := CheckWithOptions(context.Background(), progressSource, progressConfig, Options{LiveProgress: &p})
	if err == nil || p.CoverageSnapshot() != nil {
		t.Fatalf("error %v, coverage %+v", err, p.CoverageSnapshot())
	}
	var unsupported LiveProgress
	_, err = CheckWithOptions(context.Background(), progressSource, progressConfig+"VIEW A\n", Options{Coverage: true, LiveProgress: &unsupported})
	if err == nil || unsupported.CoverageSnapshot() != nil {
		t.Fatalf("configuration error %v, coverage %+v", err, unsupported.CoverageSnapshot())
	}
}

func TestLiveCoverageExistentialEarlyFailure(t *testing.T) {
	source := `---- MODULE M ----
EXTENDS Naturals
VARIABLE x
Init == x = 0
A == /\ x < 0
     /\ \E i \in 1..2 : x' = i
B == x < 0 /\ UNCHANGED x
Next == A \/ B
Inv == x < 0
====`
	for _, tc := range []struct {
		name, config string
		status       Status
		coverage     bool
	}{
		{"initial invariant", "INIT Init NEXT Next INVARIANT Inv", InvariantViolation, true},
		{"deadlock before existential", "INIT Init NEXT Next", Deadlock, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var p LiveProgress
			r, err := CheckWithOptions(context.Background(), source, tc.config, Options{Coverage: true, LiveProgress: &p})
			if err != nil || r.Status != tc.status || (len(r.Coverage) > 0) != tc.coverage || !reflect.DeepEqual(p.CoverageSnapshot(), r.Coverage) {
				t.Fatalf("result %+v, snapshot %+v, error %v", r, p.CoverageSnapshot(), err)
			}
		})
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
		if got := p.CoverageSnapshot(); !reflect.DeepEqual(got, r.Coverage) {
			t.Fatalf("coverage snapshot %+v result %+v", got, r.Coverage)
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
		if len(p.CoverageSnapshot()) == 0 {
			t.Fatal("missing live coverage")
		}
		cancel()
		close(release)
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatalf("error %v", err)
		}
		if got := p.Snapshot(); got != before || !got.InitialComplete || got.QueuedStates != 1 {
			t.Fatalf("before %+v after %+v", before, got)
		}
		if got := p.CoverageSnapshot(); got != nil {
			t.Fatalf("coverage after cancellation %+v", got)
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

func TestLiveCoverageFinalReliability(t *testing.T) {
	source := `---- MODULE M ----
EXTENDS Naturals
VARIABLE x
Init == x = 0
A == /\ x < 2
     /\ \E i \in 1..2 : x' = i
B == x < 0 /\ UNCHANGED x
Next == A \/ B
Inv == x < 2
====`
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
				_ = p.CoverageSnapshot()
			}
		}
	}()
	done := make(chan Result, 1)
	errs := make(chan error, 1)
	go func() {
		blocked := false
		r, err := CheckWithOptions(context.Background(), source, progressConfig+"INVARIANT Inv\n", Options{Coverage: true, LiveProgress: &p, OnGraph: func(e GraphEvent) error {
			if !blocked && !e.Initial && e.New {
				blocked = true
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
		if len(p.CoverageSnapshot()) == 0 {
			t.Fatal("missing live coverage")
		}
		close(release)
	case err := <-errs:
		t.Fatalf("check exited early: %v", err)
	}
	r := <-done
	close(stop)
	<-observed
	if err := <-errs; err != nil || r.Status != InvariantViolation || r.Coverage != nil || p.CoverageSnapshot() != nil {
		t.Fatalf("result %+v, snapshot %+v, error %v", r, p.CoverageSnapshot(), err)
	}
}

func TestLiveCoverageAbnormalExit(t *testing.T) {
	for _, panicCallback := range []bool{false, true} {
		t.Run(map[bool]string{false: "error", true: "panic recovered by caller"}[panicCallback], func(t *testing.T) {
			var p LiveProgress
			func() {
				defer func() {
					if v := recover(); (v != nil) != panicCallback {
						t.Errorf("panic = %v", v)
					}
				}()
				r, err := CheckWithOptions(context.Background(), progressSource, progressConfig, Options{Coverage: true, LiveProgress: &p, OnGraph: func(GraphEvent) error {
					if len(p.CoverageSnapshot()) == 0 {
						t.Error("missing live coverage in callback")
					}
					if panicCallback {
						panic("callback panic")
					}
					return errors.New("callback error")
				}})
				if err == nil || !reflect.DeepEqual(p.CoverageSnapshot(), r.Coverage) {
					t.Errorf("result %+v, error %v, coverage %+v", r, err, p.CoverageSnapshot())
				}
			}()
			if got := p.CoverageSnapshot(); got != nil {
				t.Fatalf("coverage after abnormal exit %+v", got)
			}
		})
	}
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
