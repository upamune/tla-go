package upstreamjunit

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/upamune/tla-go/internal/recorderdebug"
)

// coverageRecorderRecords uses tool output only as a cross-check: values fed
// to the upstream assertion come from Go's typed recorder. The recorder is
// incomplete, so agreement does not establish an upstream JUnit pass.
func coverageRecorderRecords(projected []bridgeRecord, events []recorderdebug.Event) ([]bridgeRecord, error) {
	if len(events) < 2 || !coverageSANYEvent(events[0], 2220) || !coverageSANYEvent(events[1], 2219) {
		return nil, fmt.Errorf("missing recorder SANY events")
	}
	events = events[2:]
	if len(projected) != len(events) {
		return nil, fmt.Errorf("coverage recorder events do not match tool messages: %d projected, %d observed", len(projected), len(events))
	}
	result := make([]bridgeRecord, 0, len(events))
	for i, event := range events {
		record, err := coverageRecorderRecord(projected[i], event)
		if err != nil {
			return nil, err
		}
		result = append(result, record)
	}
	return result, nil
}

func coverageRecorderRecord(projected bridgeRecord, event recorderdebug.Event) (bridgeRecord, error) {
	if !coverageRecordEvent(event) {
		return bridgeRecord{}, fmt.Errorf("unsupported coverage recorder event")
	}
	code := strconv.Itoa(event.Code)
	if projected.code != code || !slices.Equal(projected.values, event.Values) {
		return bridgeRecord{}, fmt.Errorf("coverage recorder event does not match tool message")
	}
	return bridgeRecord{code: code, values: slices.Clone(event.Values)}, nil
}

func coverageRecordEvent(event recorderdebug.Event) bool {
	return event.Kind == "record" && event.Array == "strings" && !event.Complete && event.Version == 0 && event.Reason == ""
}

func coverageSANYEvent(event recorderdebug.Event, code int) bool {
	return event.Kind == "record" && event.Code == code && event.Array == "strings" && !event.Complete && event.Version == 0 && event.Reason == "" && len(event.Values) == 0
}
