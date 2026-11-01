package upstreamjunit

import (
	"fmt"
	"regexp"
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

// Take values from the original non-tool mode, cross-checking the comparable
// typed calls against the separately observed tool execution. Duration has a
// different Java format in each mode and must not be inferred from tool text.
var emptyHumanDuration = regexp.MustCompile(`^[0-5][0-9]s$`)

func emptyHumanRecorderRecords(tool []bridgeRecord, events []recorderdebug.Event) ([]bridgeRecord, error) {
	if len(tool) != len(events) || len(events) != 7 {
		return nil, fmt.Errorf("incomplete human Empty recorder observations")
	}
	result := make([]bridgeRecord, 0, len(events))
	for i, event := range events {
		if event.Kind != "record" || event.Array != "strings" || event.Complete || event.Version != 0 || event.Reason != "" || event.Code != mustEmptyRecordCode(tool[i]) {
			return nil, fmt.Errorf("unexpected human Empty recorder observation")
		}
		if event.Code != 2186 && !slices.Equal(event.Values, tool[i].values) {
			return nil, fmt.Errorf("human recorder observation differs from tool mode")
		}
		if event.Code == 2186 && (len(event.Values) != 1 || !emptyHumanDuration.MatchString(event.Values[0])) {
			return nil, fmt.Errorf("invalid human Empty finish duration")
		}
		result = append(result, bridgeRecord{code: tool[i].code, values: slices.Clone(event.Values)})
	}
	return result, nil
}

func mustEmptyRecordCode(record bridgeRecord) int {
	code, _ := strconv.Atoi(record.code)
	return code
}
