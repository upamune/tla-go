package upstreamjunit

import (
	"fmt"
	"io"
	"strconv"

	"github.com/upamune/tla-go/internal/recorderdebug"
)

// decodePartialRecorder transfers directly observed typed calls. The stream
// explicitly excludes unobserved calls, so a successful decode still carries
// an omission and cannot establish an upstream JUnit pass.
func decodePartialRecorder(r io.Reader) ([]recorderRecord, []string, error) {
	events, err := recorderdebug.Decode(r)
	if err != nil {
		return nil, nil, err
	}
	omissions := []string{"recorder stream is partial; unobserved calls remain unknown"}
	records := make([]recorderRecord, 0, len(events))
	for _, event := range events {
		if event.Kind == "omission" {
			omissions = append(omissions, fmt.Sprintf("recorder omitted %d: %s", event.Code, event.Reason))
			continue
		}
		code := strconv.Itoa(event.Code)
		if !recorderCodeSupported(code) {
			return nil, nil, fmt.Errorf("unsupported recorder transfer code %s", code)
		}
		record := recorderRecord{code: code, array: event.Array}
		for _, value := range event.Values {
			record.values = append(record.values, recorderValue{kind: "s", text: value})
		}
		records = append(records, record)
	}
	return records, omissions, nil
}
