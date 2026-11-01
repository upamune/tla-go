package upstreamjunit

import (
	"reflect"
	"testing"

	"github.com/upamune/tla-go/internal/recorderdebug"
)

func TestEmptyRecorderUsesDirectValues(t *testing.T) {
	projected := []bridgeRecord{{"2199", []string{"999", "0", "0"}}, {"2186", []string{"text"}}}
	events := []recorderdebug.Event{{Code: 2199, Values: []string{"0", "0", "0"}}, {Code: 2186, Values: []string{"1ms"}}}
	got, err := replaceEmptyRecorderValues(projected, events)
	want := []bridgeRecord{{"2199", []string{"0", "0", "0"}}, {"2186", []string{"1ms"}}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("records=%v error=%v", got, err)
	}
	for _, bad := range [][]recorderdebug.Event{nil, events[:1], {events[1], events[0]}, append(events, events[0])} {
		if _, err := replaceEmptyRecorderValues(projected, bad); err == nil {
			t.Fatalf("accepted incomplete observations: %+v", bad)
		}
	}
	if _, err := replaceEmptyRecorderValues(projected[:1], events); err == nil {
		t.Fatal("accepted a missing tool message")
	}
}
