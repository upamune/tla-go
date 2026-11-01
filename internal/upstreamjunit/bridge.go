package upstreamjunit

import (
	_ "embed"
	"encoding/base64"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

//go:embed Bridge.java
var bridgeJava string

type bridgeRecord struct {
	code   string
	values []string
}

var (
	bridgeStats    = regexp.MustCompile(`^([0-9]+) states generated, ([0-9]+) distinct states found, ([0-9]+) states left on queue\.$`)
	bridgeFinished = regexp.MustCompile(`^Finished in (.+) at \([0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}\)$`)
	bridgeInitial  = regexp.MustCompile(`^Finished computing initial states: ([0-9]+) distinct state(s?) generated at [0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}\.$`)
	bridgeCoverage = regexp.MustCompile(`^(.+): ([0-9]+)$`)
)

// projectBridge is deliberately incomplete: every unrepresentable message is
// returned as an omission, including unknown IDs. Its records cannot establish
// an overall pass. Messages retains repetitions within each code.
func projectBridge(output string) ([]bridgeRecord, []string, error) {
	messages, err := Messages(output)
	if err != nil {
		return nil, nil, err
	}
	var records []bridgeRecord
	var omissions []string
	ids := make([]string, 0, len(messages))
	for id := range messages {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		for _, body := range messages[id] {
			record, omission := projectMessage(id, body)
			if omission != "" {
				omissions = append(omissions, omission)
			} else {
				records = append(records, record)
			}
		}
	}
	return records, omissions, nil
}

func projectMessage(id, body string) (bridgeRecord, string) {
	var m []string
	switch id {
	case "2186":
		m = bridgeFinished.FindStringSubmatch(body)
	case "2190":
		m = bridgeInitial.FindStringSubmatch(body)
	case "2199":
		m = bridgeStats.FindStringSubmatch(body)
	case "2221":
		m = bridgeCoverage.FindStringSubmatch(body)
	default:
		return bridgeRecord{}, "unsupported message " + id
	}
	if m == nil {
		return bridgeRecord{}, "unsupported arguments for message " + id
	}
	return bridgeRecord{id, m[1:]}, ""
}

func encodeBridge(records []bridgeRecord) (string, error) {
	var b strings.Builder
	for _, record := range records {
		switch record.code {
		case "2186", "2190", "2199", "1000", "2221":
		default:
			return "", fmt.Errorf("unsupported message %s", record.code)
		}
		b.WriteString(record.code)
		for _, v := range record.values {
			b.WriteByte('\t')
			b.WriteString(base64.StdEncoding.EncodeToString([]byte(v)))
		}
		b.WriteByte('\n')
	}
	return b.String(), nil
}

// bridgeVerdict never reports a pass until a complete typed recorder and the
// upstream lifecycle are implemented. A successful assertion alone is unsafe.
func bridgeVerdict(assertionErr error, omissions []string) error {
	return fmt.Errorf("unsupported bridge: assertion=%v; omissions=%v; incomplete recorder, coverage, initial settings and lifecycle", assertionErr, omissions)
}
