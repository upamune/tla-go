package upstreamjunit

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/upamune/tla-go/internal/recorderdebug"
)

// Build the assertion input solely from Go's typed recorder events. The tool
// projection is an independent cross-check, not a source of assertion values.
// The recorder remains partial, including on successful Empty runs.
func emptyRecorderRecords(projected []bridgeRecord, events []recorderdebug.Event) ([]bridgeRecord, error) {
	codes := []int{2220, 2219, 2185, 2189, 2190, 2193, 2199, 2194, 2186}
	arities := []int{0, 0, 0, 0, 2, 1, 3, 1, 1}
	if len(events) != len(codes) {
		return nil, fmt.Errorf("incomplete Empty recorder observations")
	}
	for i, code := range codes {
		if events[i].Kind == "omission" {
			return nil, fmt.Errorf("incomplete Empty recorder observation: %s", events[i].Reason)
		}
		if events[i].Code != code {
			return nil, fmt.Errorf("unexpected Empty recorder observation order")
		}
		if events[i].Kind != "record" || events[i].Array != "strings" || events[i].Complete || events[i].Version != 0 || events[i].Reason != "" || len(events[i].Values) != arities[i] {
			return nil, fmt.Errorf("unsupported Empty recorder observation")
		}
	}
	// The upstream setup is non-tool and does not record SANY start/end.
	events = events[2:]
	if len(projected) != len(events) {
		return nil, fmt.Errorf("recorder observations do not match tool messages")
	}
	result := make([]bridgeRecord, 0, len(events))
	for i, event := range events {
		if projected[i].code != strconv.Itoa(event.Code) || !slices.Equal(projected[i].values, event.Values) {
			return nil, fmt.Errorf("recorder observation does not match tool message")
		}
		result = append(result, bridgeRecord{code: strconv.Itoa(event.Code), values: slices.Clone(event.Values)})
	}
	return result, nil
}
