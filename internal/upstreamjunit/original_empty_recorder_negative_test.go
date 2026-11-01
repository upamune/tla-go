package upstreamjunit

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/upamune/tla-go/internal/recorderdebug"
)

func emptyRecorderFixture() ([]bridgeRecord, []recorderdebug.Event) {
	projected := []bridgeRecord{{"2185", nil}, {"2189", nil}, {"2190", []string{"0", "s"}}, {"2193", []string{"val = 0.0"}}, {"2199", []string{"0", "0", "0"}}, {"2194", []string{"1"}}, {"2186", []string{"1ms"}}}
	events := []recorderdebug.Event{
		{Kind: "record", Array: "strings", Code: 2220},
		{Kind: "record", Array: "strings", Code: 2219},
		{Kind: "record", Array: "strings", Code: 2185},
		{Kind: "record", Array: "strings", Code: 2189},
		{Kind: "record", Array: "strings", Code: 2190, Values: []string{"0", "s"}},
		{Kind: "record", Array: "strings", Code: 2193, Values: []string{"val = 0.0"}},
		{Kind: "record", Array: "strings", Code: 2199, Values: []string{"0", "0", "0"}},
		{Kind: "record", Array: "strings", Code: 2194, Values: []string{"1"}},
		{Kind: "record", Array: "strings", Code: 2186, Values: []string{"1ms"}},
	}
	return projected, events
}

func TestEmptyRecorderUsesDirectValues(t *testing.T) {
	projected, events := emptyRecorderFixture()
	got, err := emptyRecorderRecords(projected, events)
	if err != nil || !reflect.DeepEqual(got, projected) {
		t.Fatalf("records=%v error=%v", got, err)
	}
	// Assertion records must own the typed observation rather than alias it.
	events[6].Values[0] = "999"
	projected[4].values[1] = "888"
	if !slices.Equal(got[4].values, []string{"0", "0", "0"}) {
		t.Fatalf("records changed with inputs: %v", got)
	}
}

func TestEmptyRecorderRejectsMismatches(t *testing.T) {
	for _, mutate := range []func(*[]bridgeRecord, *[]recorderdebug.Event){
		func(_ *[]bridgeRecord, e *[]recorderdebug.Event) { *e = nil },
		func(_ *[]bridgeRecord, e *[]recorderdebug.Event) { *e = (*e)[:1] },
		func(_ *[]bridgeRecord, e *[]recorderdebug.Event) { (*e)[0], (*e)[1] = (*e)[1], (*e)[0] },
		func(_ *[]bridgeRecord, e *[]recorderdebug.Event) { *e = append(*e, (*e)[0]) },
		func(_ *[]bridgeRecord, e *[]recorderdebug.Event) { (*e)[4].Array = "objects" },
		func(_ *[]bridgeRecord, e *[]recorderdebug.Event) { (*e)[4].Kind = "other" },
		func(_ *[]bridgeRecord, e *[]recorderdebug.Event) { (*e)[4].Values = nil },
		func(_ *[]bridgeRecord, e *[]recorderdebug.Event) { (*e)[4].Complete = true },
		func(p *[]bridgeRecord, _ *[]recorderdebug.Event) { *p = (*p)[:1] },
		func(p *[]bridgeRecord, _ *[]recorderdebug.Event) { *p = append(*p, bridgeRecord{code: "1000"}) },
		func(p *[]bridgeRecord, _ *[]recorderdebug.Event) { (*p)[4].values[0] = "999" },
		func(_ *[]bridgeRecord, e *[]recorderdebug.Event) { (*e)[5].Values[0] = "val = 1.0" },
	} {
		projected, events := emptyRecorderFixture()
		mutate(&projected, &events)
		if _, err := emptyRecorderRecords(projected, events); err == nil {
			t.Fatalf("accepted mismatched observations: %v %+v", projected, events)
		}
	}
}

func TestEmptyRecorderRejectsOmission(t *testing.T) {
	projected, events := emptyRecorderFixture()
	events[5] = recorderdebug.Event{Kind: "omission", Code: 2193, Reason: recorderdebug.SuccessProbabilityOmission}
	if _, err := emptyRecorderRecords(projected, events); err == nil || !strings.Contains(err.Error(), recorderdebug.SuccessProbabilityOmission) {
		t.Fatalf("unexpected omission error: %v", err)
	}
}
