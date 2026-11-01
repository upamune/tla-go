package upstreamjunit

import (
	"reflect"
	"strings"
	"testing"

	"github.com/upamune/tla-go/internal/recorderdebug"
)

func TestEmptyRecorderUsesDirectValues(t *testing.T) {
	projected := []bridgeRecord{{"2185", nil}, {"2189", nil}, {"2190", []string{"999", "s"}}, {"2193", []string{"val = 0.0"}}, {"2199", []string{"999", "0", "0"}}, {"2194", []string{"99"}}, {"2186", []string{"text"}}}
	events := []recorderdebug.Event{{Code: 2220}, {Code: 2219}, {Code: 2185}, {Code: 2189}, {Code: 2190, Values: []string{"0", "s"}}, {Code: 2193, Values: []string{"val = 0.0"}}, {Code: 2199, Values: []string{"0", "0", "0"}}, {Code: 2194, Values: []string{"1"}}, {Code: 2186, Values: []string{"1ms"}}}
	got, err := replaceEmptyRecorderValues(projected, events)
	want := []bridgeRecord{{"2185", nil}, {"2189", nil}, {"2190", []string{"0", "s"}}, {"2193", []string{"val = 0.0"}}, {"2199", []string{"0", "0", "0"}}, {"2194", []string{"1"}}, {"2186", []string{"1ms"}}}
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
	projected[3].values = []string{"val = 1.0"}
	if _, err := replaceEmptyRecorderValues(projected, events); err == nil {
		t.Fatal("accepted a changed success probability")
	}
}

func TestEmptyRecorderRejectsOmission(t *testing.T) {
	events := []recorderdebug.Event{{Code: 2220}, {Code: 2219}, {Code: 2185}, {Code: 2189}, {Code: 2190}, {Kind: "omission", Code: 2193, Reason: recorderdebug.SuccessProbabilityOmission}, {Code: 2199}, {Code: 2194}, {Code: 2186}}
	if _, err := replaceEmptyRecorderValues(nil, events); err == nil || !strings.Contains(err.Error(), recorderdebug.SuccessProbabilityOmission) {
		t.Fatalf("unexpected omission error: %v", err)
	}
}
