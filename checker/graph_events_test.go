package checker

import (
	"context"
	"errors"
	"testing"
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
