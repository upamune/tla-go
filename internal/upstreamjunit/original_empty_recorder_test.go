package upstreamjunit

import (
	"fmt"
	"strconv"

	"github.com/upamune/tla-go/internal/recorderdebug"
)

// Use the typed observations for these two calls, while requiring the text
// stream to contain them in the same order. Other calls are still incomplete.
func replaceEmptyRecorderValues(records []bridgeRecord, events []recorderdebug.Event) ([]bridgeRecord, error) {
	if len(events) != 2 || events[0].Code != 2199 || events[1].Code != 2186 {
		return nil, fmt.Errorf("incomplete Empty recorder observations")
	}
	result := make([]bridgeRecord, 0, len(records))
	index := 0
	for _, record := range records {
		if record.code == "2199" || record.code == "2186" {
			if index >= len(events) || record.code != strconv.Itoa(events[index].Code) {
				return nil, fmt.Errorf("recorder observation does not match tool message order")
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
