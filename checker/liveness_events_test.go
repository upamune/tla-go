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
