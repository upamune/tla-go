package checker

import (
	"context"
	"reflect"
	"testing"

	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
)

const (
	observerSource = "---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nInit == x \\in {0, 1, 2}\nNext == (x < 4 /\\ x' = x + 1) \\/ x' = x\n===="
	observerConfig = "INIT Init NEXT Next CHECK_DEADLOCK FALSE"
)

func observedModel(t *testing.T, options Options, observer *checkpointObserver) *model {
	t.Helper()
	cfg, err := config.Parse(observerConfig)
	if err != nil {
		t.Fatal(err)
	}
	m, err := prepareCheckpointModelObserved(context.Background(), observerSource, observerConfig, cfg, options, observer)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestCheckpointObserverRejectsUserCallbacks(t *testing.T) {
	cfg, err := config.Parse(observerConfig)
	if err != nil {
		t.Fatal(err)
	}
	effects := 0
	for name, options := range map[string]Options{
		"search":  {OnSearch: func(SearchEvent) { effects++ }},
		"warning": {OnWarning: func(Warning) { effects++ }},
		"print":   {OnPrint: func(string) { effects++ }},
		"pause":   {OnPause: func() { effects++ }},
		"read":    {ReadFile: func(string) (string, error) { effects++; return "", nil }},
		"write":   {WriteFile: func(string, string) error { effects++; return nil }},
	} {
		t.Run(name, func(t *testing.T) {
			for _, observer := range []*checkpointObserver{nil, {}} {
				_, err := prepareCheckpointModelObserved(context.Background(), observerSource, observerConfig, cfg, options, observer)
				if err == nil || effects != 0 {
					t.Fatalf("callback accepted or invoked: err=%v effects=%d", err, effects)
				}
			}
			module, err := parser.ParseModule(observerSource)
			if err != nil {
				t.Fatal(err)
			}
			m, err := prepareWithOutput(module, cfg, options)
			if err != nil {
				t.Fatal(err)
			}
			if err := m.checkpointSupported(); err == nil || effects != 0 {
				t.Fatalf("checkpoint accepted or invoked callback: err=%v effects=%d", err, effects)
			}
		})
	}
}

func TestCheckpointObserverReportingAndRoundTrip(t *testing.T) {
	observer := &checkpointObserver{}
	full := observedModel(t, Options{}, observer)
	want, err := full.check()
	if err != nil {
		t.Fatal(err)
	}
	var events []SearchEvent
	_, err = CheckWithOptions(context.Background(), observerSource, observerConfig, Options{OnSearch: func(e SearchEvent) { events = append(events, e) }})
	if err != nil {
		t.Fatal(err)
	}
	var recorded []SearchEvent
	for _, record := range observer.records {
		if record.searchEvent != nil {
			recorded = append(recorded, *record.searchEvent)
		}
	}
	if len(events) < 2 || !reflect.DeepEqual(recorded, events) {
		t.Fatalf("events: got %+v, want %+v", recorded, events)
	}
	// Verify all three recording paths retain values and order, including the
	// warning and print paths unavailable in the supported expression subset.
	observer.warning(Warning{Code: 7, Message: "warning"})
	observer.print("line")
	n := len(observer.records)
	if *observer.records[n-2].warning != (Warning{Code: 7, Message: "warning"}) || *observer.records[n-1].print != "line" {
		t.Fatal("reporting records lost or reordered")
	}
	before := &checkpointObserver{}
	m := observedModel(t, Options{}, before)
	s := pauseSearch(t, m, 1)
	snapshot, err := s.checkpoint(*m.checkpointIdentity)
	if err != nil {
		t.Fatal(err)
	}
	data, err := checkpointEncode(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = checkpointDecode(data, *m.checkpointIdentity)
	if err != nil {
		t.Fatal(err)
	}
	after := &checkpointObserver{}
	fresh := observedModel(t, Options{}, after)
	restored, err := fresh.restoreCheckpoint(snapshot, *fresh.checkpointIdentity)
	if err != nil {
		t.Fatal(err)
	}
	got, err := restored.explore()
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("resumed result: got %+v, want %+v, error %v", got, want, err)
	}
	if len(before.records) == 0 {
		t.Fatal("initial progress not observed before checkpoint")
	}
}

// Effectful expressions remain outside the checkpoint language, but exercising
// them verifies that the private callbacks are wired through real evaluation.
func TestCheckpointObserverEvaluationOutput(t *testing.T) {
	source := "---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE f\nInit == /\\ PrintT(42) /\\ f = <<0>>\nNext == f' = [f EXCEPT ![2] = 0]\n===="
	cfg, err := config.Parse(observerConfig)
	if err != nil {
		t.Fatal(err)
	}
	observer := &checkpointObserver{}
	m, err := prepareCheckpointModelObserved(context.Background(), source, observerConfig, cfg, Options{}, observer)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.checkpointSupported(); err == nil {
		t.Fatal("effectful model accepted")
	}
	if _, err := m.check(); err != nil {
		t.Fatal(err)
	}
	var prints []string
	var warnings []Warning
	for _, record := range observer.records {
		if record.print != nil {
			prints = append(prints, *record.print)
		}
		if record.warning != nil {
			warnings = append(warnings, *record.warning)
		}
	}
	var wantPrints []string
	var wantWarnings []Warning
	_, err = CheckWithOptions(context.Background(), source, observerConfig, Options{
		OnPrint:   func(line string) { wantPrints = append(wantPrints, line) },
		OnWarning: func(w Warning) { wantWarnings = append(wantWarnings, w) },
	})
	if err != nil || len(prints) == 0 || len(warnings) == 0 || !reflect.DeepEqual(prints, wantPrints) || !reflect.DeepEqual(warnings, wantWarnings) {
		t.Fatalf("print %v vs %v; warnings %v vs %v; error %v", prints, wantPrints, warnings, wantWarnings, err)
	}
}
