package upstreamjunit

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/upamune/tla-go/internal/recorderdebug"
)

// Use the typed observations for the selected calls, while requiring the text
// stream to contain them in the same order. Other calls are still incomplete.
func replaceEmptyRecorderValues(records []bridgeRecord, events []recorderdebug.Event) ([]bridgeRecord, error) {
	codes := []int{2220, 2219, 2185, 2189, 2190, 2193, 2199, 2194, 2186}
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
	}
	// The upstream setup is non-tool and does not record SANY start/end.
	events, codes = events[2:], codes[2:]
	result := make([]bridgeRecord, 0, len(records))
	index := 0
	for _, record := range records {
		code, _ := strconv.Atoi(record.code)
		if slices.Contains(codes, code) {
			if index >= len(events) || record.code != strconv.Itoa(events[index].Code) {
				return nil, fmt.Errorf("recorder observation does not match tool message order")
			}
			if code == 2193 && !slices.Equal(record.values, events[index].Values) {
				return nil, fmt.Errorf("success observation does not match tool message")
			}
			record.values = events[index].Values
			index++
		}
		result = append(result, record)
	}
	if index != len(events) {
		return nil, fmt.Errorf("missing tool message for recorder observation")
	}
	return result, nil
}
