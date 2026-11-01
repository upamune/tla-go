package upstreamjunit

import (
	"fmt"
	"strconv"
	"strings"
)

// ZeroCoverage projects TestMPRecorder.getZeroCoverage: only 2221 records
// count, including duplicates. Require a complete report to distinguish missing
// coverage instrumentation from a genuinely empty set.
func ZeroCoverage(result Result) ([]string, error) {
	messages, err := Messages(result.Output)
	if err != nil {
		return nil, err
	}
	if len(messages["2201"]) == 0 || len(messages["2202"]) == 0 {
		return nil, fmt.Errorf("coverage report incomplete")
	}
	var zero []string
	for _, entry := range messages["2221"] {
		location, count, ok := strings.Cut(entry, ": ")
		if !ok {
			return nil, fmt.Errorf("invalid coverage value %q", entry)
		}
		n, err := strconv.ParseInt(strings.TrimSpace(count), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid coverage value %q: %w", entry, err)
		}
		if n == 0 {
			zero = append(zero, strings.TrimSpace(strings.ReplaceAll(location, "|", "")))
		}
	}
	return zero, nil
}
