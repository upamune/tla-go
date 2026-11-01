package checker

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestModelLivenessDiagnosticDoesNotRepeatEffects(t *testing.T) {
	const source = "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 2\nP == []<>(x = 0 /\\ PrintT(\"effect\"))\n===="
	var baseline, observed []string
	without := Options{OnPrint: func(s string) { baseline = append(baseline, s) }}
	with := Options{OnPrint: func(s string) { observed = append(observed, s) }, OnSearch: func(SearchEvent) {}}
	for _, opts := range []Options{without, with} {
		if _, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next PROPERTY P", opts); err != nil {
			t.Fatal(err)
		}
	}
	if len(baseline) == 0 || !reflect.DeepEqual(baseline, observed) {
		t.Fatalf("without diagnostic %q, with diagnostic %q", baseline, observed)
	}
}

func TestTemporalCheckFailureInvalidatesStart(t *testing.T) {
	const source = "---- MODULE M ----\nEXTENDS Integers\nVARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 3\nP == <>(1 \\div (x - 1) = 0)\n===="
	var events []SearchEvent
	_, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE", Options{OnSearch: func(e SearchEvent) { events = append(events, e) }})
	if err == nil {
		t.Fatal("expected liveness evaluation failure")
	}
	var temporal []SearchEventKind
	for _, event := range events {
		if event.Kind == TemporalCheckStart || event.Kind == TemporalCheckUncertain {
			temporal = append(temporal, event.Kind)
		}
	}
	if !reflect.DeepEqual(temporal, []SearchEventKind{TemporalCheckStart, TemporalCheckUncertain}) {
		t.Fatalf("temporal events = %v", temporal)
	}
}

func TestPureRecurrenceFailureDuringExploration(t *testing.T) {
	for _, property := range []string{"[]<>(1 \\div (x - 1) = 0)", "<>[](1 \\div (x - 1) = 0)"} {
		t.Run(property, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS Integers\nVARIABLE x\nInit == x = 0\nNext == x' = (x + 1) % 3\nP == " + property + "\n===="
			var events []SearchEvent
			result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE", Options{OnSearch: func(e SearchEvent) { events = append(events, e) }})
			if err != nil || result.Status != EvaluationFailure || result.Failure == nil || result.Failure.NestedToolCode != 2179 || result.GeneratedStates != 3 || result.DistinctStates != 3 || result.QueuedStates != 1 {
				t.Fatalf("result=%+v error=%v", result, err)
			}
			for _, event := range events {
				if event.Kind == TemporalCheckStart || event.Kind == TemporalCheckUncertain {
					t.Fatalf("unexpected final temporal check event: %+v", event)
				}
			}
		})
	}
}

func TestTableauProgressDoesNotReplayEffects(t *testing.T) {
	const source = "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x = 0\nNext == /\\ x' = (x + 1) % 2 /\\ PrintT(x)\nP == <>(x = 2)\n===="
	var baseline, observed []string
	var events []SearchEvent
	without := Options{OnPrint: func(s string) { baseline = append(baseline, s) }}
	with := Options{OnPrint: func(s string) { observed = append(observed, s) }, OnSearch: func(e SearchEvent) { events = append(events, e) }}
	for _, opts := range []Options{without, with} {
		result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next PROPERTY P", opts)
		if err != nil || result.Status != LivenessViolation {
			t.Fatalf("got %+v, %v", result, err)
		}
	}
	if len(baseline) == 0 || !reflect.DeepEqual(baseline, observed) {
		t.Fatalf("without listener %q, with listener %q", baseline, observed)
	}
	for _, event := range events {
		if event.Kind == TemporalCheckStart {
			t.Fatalf("reported an unsafe tableau count: %+v", event)
		}
	}
}

func TestEffectfulFairnessTableauProgressRemainsUnsupported(t *testing.T) {
	const source = "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x = 0\nNext == /\\ x' \\in {0, 1} /\\ PrintT(\"effect\")\nSpec == Init /\\ [][Next]_x /\\ WF_x(Next)\nP == <>(x = 2)\n===="
	var baseline, observed []string
	var events []SearchEvent
	for _, opts := range []Options{
		{OnPrint: func(s string) { baseline = append(baseline, s) }},
		{OnPrint: func(s string) { observed = append(observed, s) }, OnSearch: func(e SearchEvent) { events = append(events, e) }},
	} {
		result, err := CheckWithOptions(context.Background(), source, "SPECIFICATION Spec PROPERTY P", opts)
		if err != nil || result.Status != LivenessViolation {
			t.Fatalf("got %+v, %v", result, err)
		}
	}
	if len(baseline) == 0 || !reflect.DeepEqual(baseline, observed) {
		t.Fatalf("without listener %q, with listener %q", baseline, observed)
	}
	foundModelLiveness := false
	for _, event := range events {
		foundModelLiveness = foundModelLiveness || event.Kind == ModelCheckingLiveness
		if event.Kind == TemporalCheckStart {
			t.Fatalf("reported unsafe tableau count: %+v", event)
		}
	}
	if !foundModelLiveness {
		t.Fatal("missing model liveness solution count")
	}
}

func TestTableauProgressRejectsEffectsReachableFromReplay(t *testing.T) {
	for _, tc := range []struct{ name, next, extra, config string }{
		{"initial used by next", "Init /\\ UNCHANGED x", "", ""},
		{"state constraint", "UNCHANGED x", "C == PrintT(\"constraint\")", " CONSTRAINT C"},
		{"action constraint", "UNCHANGED x", "C == PrintT(\"constraint\")", " ACTION_CONSTRAINT C"},
		{"dynamic next", "TLCGet(\"distinct\") = 1 /\\ UNCHANGED x", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "---- MODULE M ----\nEXTENDS TLC\nVARIABLE x\nInit == x = 0 /\\ PrintT(\"init\")\nNext == " + tc.next + "\n" + tc.extra + "\nP == <>(x = 1)\n===="
			var events []SearchEvent
			var baseline, observed []string
			for _, opts := range []Options{
				{OnPrint: func(s string) { baseline = append(baseline, s) }},
				{OnPrint: func(s string) { observed = append(observed, s) }, OnSearch: func(e SearchEvent) { events = append(events, e) }},
			} {
				if _, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next PROPERTY P"+tc.config, opts); err != nil {
					t.Fatal(err)
				}
			}
			if !reflect.DeepEqual(baseline, observed) {
				t.Fatalf("without listener %q, with listener %q", baseline, observed)
			}
			for _, event := range events {
				if event.Kind == TemporalCheckStart {
					t.Fatalf("reported unsafe tableau count: %+v", event)
				}
			}
		})
	}
}

func TestTableauProgressInitialPredicateReuse(t *testing.T) {
	for _, tc := range []struct {
		name, init, next, extra, config string
		wantProgress                    bool
	}{
		{"initial only", `x = 0 /\ PrintT("init")`, "UNCHANGED x", "", "", true},
		{"state constraint", `x = 0 /\ PrintT("init")`, "UNCHANGED x", "", " CONSTRAINT Init", false},
		{"action constraint", `x = 0 /\ PrintT("init")`, "UNCHANGED x", "", " ACTION_CONSTRAINT Init", false},
		{"invariant", `x = 0 /\ PrintT("init")`, "UNCHANGED x", "", " INVARIANT Init", false},
		{"dynamic constraint", `x = 0 /\ TLCGet("distinct") >= 0`, "UNCHANGED x", "", " CONSTRAINT Init", false},
		{"constraint alias", `x = 0 /\ PrintT("init")`, "UNCHANGED x", "C == Init", " CONSTRAINT C", false},
		{"nested alias", `x = 0 /\ PrintT("init")`, "UNCHANGED x", "A == Init\nC == A", " ACTION_CONSTRAINT C", false},
		{"action alias", `x = 0 /\ PrintT("init")`, "A /\\ UNCHANGED x", "A == Init", "", false},
		{"instance alias", `x = 0 /\ I!PrintT("init")`, "UNCHANGED x", "I == INSTANCE TLC\nC == Init", " CONSTRAINT C", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pre, extra := "", tc.extra
			if after, ok := strings.CutPrefix(extra, "I == INSTANCE TLC\n"); ok {
				pre, extra = "I == INSTANCE TLC\n", after
			}
			source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\n" + pre + "Init == " + tc.init + "\n" + extra + "\nNext == " + tc.next + "\nP == <>(x = 1)\n===="
			var events []SearchEvent
			var baseline, observed []string
			for _, opts := range []Options{
				{OnPrint: func(s string) { baseline = append(baseline, s) }},
				{OnPrint: func(s string) { observed = append(observed, s) }, OnSearch: func(e SearchEvent) { events = append(events, e) }},
			} {
				result, err := CheckWithOptions(context.Background(), source, "INIT Init NEXT Next PROPERTY P"+tc.config, opts)
				if err != nil || result.Status != LivenessViolation {
					t.Fatalf("got %+v, %v", result, err)
				}
			}
			if !reflect.DeepEqual(baseline, observed) {
				t.Fatalf("without listener %q, with listener %q", baseline, observed)
			}
			found := false
			for _, event := range events {
				found = found || event.Kind == TemporalCheckStart
			}
			if found != tc.wantProgress {
				t.Fatalf("progress = %t, want %t: %+v", found, tc.wantProgress, events)
			}
		})
	}
}

func TestModelLivenessDiagnosticBoundsFairnessExpansion(t *testing.T) {
	source := "---- MODULE M ----\nVARIABLE x\nASSUME FALSE\nInit == x = 0\nNext == x' = 1\nSpec == Init /\\ [][Next]_x" + strings.Repeat(" /\\ SF_x(Next)", 20) + "\nP == <>(x = 1)\n===="
	var events []SearchEvent
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := CheckWithOptions(ctx, source, "SPECIFICATION Spec PROPERTY P", Options{OnSearch: func(e SearchEvent) { events = append(events, e) }})
	if err != nil || result.Status != AssumptionViolation {
		t.Fatalf("got %+v, %v; want assumption violation", result, err)
	}
	for _, event := range events {
		if event.Kind == ModelCheckingLiveness {
			t.Fatalf("reported a count for an oversized normal form: %+v", event)
		}
	}
}
