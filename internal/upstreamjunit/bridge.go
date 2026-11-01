package upstreamjunit

import (
	_ "embed"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

//go:embed Bridge.java
var bridgeJava string

//go:embed GoEmptyTest.java
var goEmptyTestJava string

//go:embed GoSuiteTest10.java
var goSuiteTest10Java string

//go:embed GoBCoverageTest.java
var goBCoverageTestJava string

//go:embed RecorderFixture.java
var recorderFixtureJava string

//go:embed CaptureRecorder.java
var captureRecorderJava string

//go:embed OriginalEmptyTLC.java
var originalEmptyTLCJava string

//go:embed OriginalEmptyArgsFixture.java
var originalEmptyArgsFixtureJava string

type bridgeRecord struct {
	code   string
	values []string
}

var (
	bridgeStats    = regexp.MustCompile(`^([0-9]+) states generated, ([0-9]+) distinct states found, ([0-9]+) states left on queue\.$`)
	bridgeFinished = regexp.MustCompile(`^Finished in (.+) at \([0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}\)$`)
	bridgeInitial  = regexp.MustCompile(`^Finished computing initial states: ([0-9]+) distinct state(s?) generated at [0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}\.$`)
	bridgeDepth    = regexp.MustCompile(`^The depth of the complete state graph search is ([0-9]+)\.$`)
	bridgeCoverage = regexp.MustCompile(`^  (.+): ([0-9]+)$`)
	bridgeCost     = regexp.MustCompile(`^  (.+): ([0-9]+):([0-9]+)$`)
	bridgeAction   = regexp.MustCompile(`^(<[^\n]+>): ([0-9]+):([0-9]+)$`)
	bridgeProperty = regexp.MustCompile(`^(<[^\n]+>)$`)
)

// projectBridge is deliberately incomplete: every unrepresentable message is
// returned as an omission, including unknown IDs. Its records cannot establish
// an overall pass. Messages retains repetitions within each code.
func projectBridge(output string) ([]bridgeRecord, []string, error) {
	p := messageParser{output: strings.ReplaceAll(output, "\r\n", "\n"), messages: make(map[string][]string), captureOrder: true}
	if err := p.parse(); err != nil {
		return nil, nil, err
	}
	if err := p.validateComplete(); err != nil {
		return nil, nil, err
	}
	var records []bridgeRecord
	omissions := []string{"tool messages do not establish recorder array types or all recorder calls"}
	if p.nested {
		omissions = append(omissions, "nested tool messages do not establish recorder call order")
	}
	slices.SortFunc(p.ordered, func(a, b orderedMessage) int { return a.order - b.order })
	for _, message := range p.ordered {
		record, omission := projectMessage(message.id, message.body)
		if omission != "" {
			omissions = append(omissions, omission)
		}
		if record.code != "" {
			records = append(records, record)
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
	case "2194":
		m = bridgeDepth.FindStringSubmatch(body)
	case "2221", "2772", "2773", "2774", "2775":
		m = bridgeCoverageMatch(id, body)
	case "1000", "2776":
		// The upstream assertions check only presence. The placeholder array has
		// no inferred arguments; its type and Java's hidden calls are unknown.
		return bridgeRecord{id, nil}, "presence only; recorder arguments unavailable for message " + id
	default:
		return bridgeRecord{}, "unsupported message " + id
	}
	if m == nil {
		return bridgeRecord{}, "unsupported arguments for message " + id
	}
	return bridgeRecord{id, m[1:]}, ""
}

func bridgeCoverageMatch(id, body string) []string {
	switch id {
	case "2221":
		return bridgeCoverage.FindStringSubmatch(body)
	case "2772", "2773":
		return bridgeAction.FindStringSubmatch(body)
	case "2774":
		return bridgeProperty.FindStringSubmatch(body)
	case "2775":
		return bridgeCost.FindStringSubmatch(body)
	}
	return nil
}

// bridgeVerdict never reports a pass until a complete typed recorder and the
// upstream lifecycle are implemented. A successful assertion alone is unsafe.
func bridgeVerdict(assertionErr error, omissions []string) error {
	return fmt.Errorf("unsupported bridge: assertion=%v; omissions=%v; incomplete recorder, coverage, initial settings and lifecycle", assertionErr, omissions)
}
