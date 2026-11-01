// Package upstreamjunit contains building blocks for a pinned upstream TLC sweep.
package upstreamjunit

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// Result captures observable TLC tool-mode output. A timeout is an infrastructure
// failure, not an exit status or a known compatibility mismatch.
type Result struct {
	ExitCode int
	TimedOut bool
	Output   string
}

var (
	marker             = regexp.MustCompile(`@!@!@(STARTMSG ([0-9]+):[0-9]+|ENDMSG ([0-9]+)) @!@!@`)
	messageTime        = regexp.MustCompile(`( at \(?)[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}(\)?\.?)$`)
	initialCount       = regexp.MustCompile(`^Finished computing initial states: ([0-9]+) distinct states? generated at [0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}\.$`)
	temporalParameters = regexp.MustCompile(`^Checking (.*?)temporal properties for the (\w+) state space with ([0-9]+) total distinct states at \([0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}\)$`)
)

type (
	frame struct {
		id    string
		start int
	}
	messageParser struct {
		output   string
		stack    []frame
		messages map[string][]string
	}
)

// Messages parses complete TLC tool messages, including nested messages.
func Messages(output string) (map[string][]string, error) {
	p := messageParser{output: strings.ReplaceAll(output, "\r\n", "\n"), messages: make(map[string][]string)}
	if err := p.parse(); err != nil {
		return nil, err
	}
	if err := p.validateComplete(); err != nil {
		return nil, err
	}
	return p.messages, nil
}

func (p *messageParser) parse() error {
	for pos := 0; pos < len(p.output); {
		n := strings.Index(p.output[pos:], "@!@!@")
		if n < 0 {
			return p.checkTrailingMarker(pos)
		}
		n += pos
		m := marker.FindStringSubmatchIndex(p.output[n:])
		if m == nil || m[0] != 0 {
			return fmt.Errorf("malformed tool marker at byte %d", n)
		}
		pos = n + m[1]
		if err := p.consume(n, pos, m); err != nil {
			return err
		}
	}
	return nil
}

func (p *messageParser) validateComplete() error {
	if len(p.stack) > 0 {
		return fmt.Errorf("unterminated tool message %s", p.stack[len(p.stack)-1].id)
	}
	if len(p.messages) == 0 {
		return fmt.Errorf("no TLC tool messages")
	}
	return nil
}

// A short sentinel is ambiguous in ordinary text; recognize it only as a
// complete trailing line outside a message. Open messages fail separately.
func (p *messageParser) checkTrailingMarker(pos int) error {
	lastLine := p.output[pos:]
	if i := strings.LastIndexByte(lastLine, '\n'); i >= 0 {
		lastLine = lastLine[i+1:]
	}
	if len(p.stack) == 0 && lastLine != "" && strings.HasPrefix("@!@!@", lastLine) {
		return fmt.Errorf("truncated tool marker at byte %d", len(p.output)-len(lastLine))
	}
	return nil
}

func (p *messageParser) consume(n, end int, m []int) error {
	if m[4] >= 0 {
		id := p.output[n+m[4] : n+m[5]]
		if end >= len(p.output) || p.output[end] != '\n' {
			return fmt.Errorf("missing newline after start %s", id)
		}
		p.stack = append(p.stack, frame{id, end + 1})
		return nil
	}
	return p.close(n, end, p.output[n+m[6]:n+m[7]])
}

func (p *messageParser) close(n, end int, id string) error {
	if end < len(p.output) && p.output[end] != '\n' {
		return fmt.Errorf("missing newline after end %s", id)
	}
	if len(p.stack) == 0 || p.stack[len(p.stack)-1].id != id {
		return fmt.Errorf("unexpected tool message end %s", id)
	}
	f := p.stack[len(p.stack)-1]
	bodyEnd := n
	if n > f.start {
		if p.output[n-1] != '\n' {
			return fmt.Errorf("missing newline before end %s", id)
		}
		bodyEnd--
	}
	p.messages[id] = append(p.messages[id], p.output[f.start:bodyEnd])
	p.stack = p.stack[:len(p.stack)-1]
	return nil
}

// Compare reports differing exit status and selected tool message bodies. The
// caller must explicitly select message IDs; absence on only one side differs.
func Compare(java, goResult Result, ids []string) ([]string, error) {
	if java.TimedOut || goResult.TimedOut {
		return nil, fmt.Errorf("timeout: java=%t go=%t", java.TimedOut, goResult.TimedOut)
	}
	jm, err := Messages(java.Output)
	if err != nil {
		return nil, fmt.Errorf("java: %w", err)
	}
	gm, err := Messages(goResult.Output)
	if err != nil {
		return nil, fmt.Errorf("go: %w", err)
	}
	var differences []string
	if java.ExitCode != goResult.ExitCode {
		differences = append(differences, "exit")
	}
	for _, id := range ids {
		if !slices.Equal(normalizeMessages(id, jm[id]), normalizeMessages(id, gm[id])) {
			differences = append(differences, "message:"+id)
		}
	}
	sort.Strings(differences)
	return differences, nil
}

// MP.java appends now() to these messages; all other text remains significant.
func normalizeMessages(id string, messages []string) []string {
	if id != "2190" && id != "2191" && id != "2192" {
		return messages
	}
	result := make([]string, len(messages))
	for i, body := range messages {
		result[i] = messageTime.ReplaceAllString(body, "${1}<time>${2}")
	}
	return result
}

// ComparePresence handles recorder assertions that do not inspect message text.
func ComparePresence(java, goResult Result, ids []string) ([]string, error) {
	jm, err := Messages(java.Output)
	if err != nil {
		return nil, err
	}
	gm, err := Messages(goResult.Output)
	if err != nil {
		return nil, err
	}
	var differences []string
	for _, id := range ids {
		if (len(jm[id]) > 0) != (len(gm[id]) > 0) {
			differences = append(differences, "presence:"+id)
		}
	}
	return differences, nil
}

// CheckBaseline accepts only the exact mismatch categories recorded for a case.
// An improvement is reported as stale so the baseline cannot quietly expand.
func CheckBaseline(caseID string, differences []string, baseline map[string][]string) error {
	expected, known := baseline[caseID]
	actual := append([]string(nil), differences...)
	expected = append([]string(nil), expected...)
	sort.Strings(actual)
	sort.Strings(expected)
	if len(actual) == 0 && !known {
		return nil
	}
	if known && len(actual) > 0 && slices.Equal(actual, expected) {
		return nil
	}
	return fmt.Errorf("%s: differences %v, baseline %v", caseID, actual, expected)
}

// AssertedObservation describes the subset of literal upstream recorder assertions
// that can be evaluated from tool output. Unsupported assertion forms remain omissions.
type AssertedObservation struct {
	ID      string
	Present bool
	Values  []string
}

var literalAssertion = regexp.MustCompile(`(?s)assert(True|False)\s*\(\s*recorder\.(recordedWithStringValues|recordedWithStringValue|recorded)\s*\(\s*EC\.(\w+)\s*([^)]*)\)\s*\)`)

// ParseAssertions captures literal recorder assertions, preserving true/false.
func ParseAssertions(source, ecSource string) ([]AssertedObservation, []string) {
	codes := map[string]string{}
	for _, m := range ecConst.FindAllStringSubmatch(maskJava(ecSource, true), -1) {
		codes[m[1]] = m[2]
	}
	var observations []AssertedObservation
	var omissions []string
	clean := maskJava(source, false)
	masked := maskJava(source, true)
	for _, idx := range literalAssertion.FindAllStringSubmatchIndex(masked, -1) {
		m := javaSubmatches(clean, idx)
		id, ok := codes[m[3]]
		if !ok {
			omissions = append(omissions, "unknown EC."+m[3])
			continue
		}
		obs := AssertedObservation{ID: id, Present: m[1] == "True"}
		values, valid := assertionValues(m[4])
		if !valid {
			omissions = append(omissions, "nonliteral assertion: EC."+m[3])
			continue
		}
		obs.Values = values
		observations = append(observations, obs)
	}
	return observations, omissions
}

func javaSubmatches(source string, indexes []int) []string {
	matches := make([]string, len(indexes)/2)
	for i := range matches {
		if indexes[2*i] >= 0 {
			matches[i] = source[indexes[2*i]:indexes[2*i+1]]
		}
	}
	return matches
}

func assertionValues(rest string) ([]string, bool) {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return nil, true
	}
	rest = strings.TrimSpace(strings.TrimPrefix(rest, ","))
	var values []string
	for _, expr := range splitJava(rest) {
		v, err := strconv.Unquote(expr)
		if err != nil {
			return nil, false
		}
		values = append(values, v)
	}
	return values, true
}

// CheckAssertions checks presence and numeric TLC statistic/depth values. Other
// parameter formats require the Java recorder representation and are omitted.
func CheckAssertions(result Result, observations []AssertedObservation) ([]string, []string, error) {
	messages, err := Messages(result.Output)
	if err != nil {
		return nil, nil, err
	}
	var failures, omissions []string
	for _, obs := range observations {
		failure, omission := checkObservation(messages[obs.ID], obs)
		if failure != "" {
			failures = append(failures, failure)
		}
		if omission != "" {
			omissions = append(omissions, omission)
		}
	}
	return failures, omissions, nil
}

func checkObservation(entries []string, obs AssertedObservation) (string, string) {
	if len(obs.Values) == 0 {
		if (len(entries) > 0) != obs.Present {
			return "presence:" + obs.ID, ""
		}
		return "", ""
	}
	return checkValues(entries, obs)
}

func checkValues(entries []string, obs AssertedObservation) (string, string) {
	if obs.ID == "2190" && len(obs.Values) == 1 {
		return checkInitialCount(entries, obs)
	}
	if obs.ID == "2192" && len(obs.Values) <= 3 {
		return checkTemporalParameters(entries, obs)
	}
	if obs.ID != "2194" && obs.ID != "2199" {
		return "", "recorder parameters: " + obs.ID
	}
	return checkNumericObservation(entries, obs), ""
}

func checkNumericObservation(entries []string, obs AssertedObservation) string {
	return assertionResult(len(entries) > 0 && matchingNumbers(entries[0], obs.Values), obs)
}

func checkInitialCount(entries []string, obs AssertedObservation) (string, string) {
	if len(entries) > 0 {
		match := initialCount.FindStringSubmatch(entries[0])
		if match == nil {
			return "", "unrecognized initial-state message: 2190"
		}
		return assertionResult(match[1] == obs.Values[0], obs), ""
	}
	return assertionResult(false, obs), ""
}

func checkTemporalParameters(entries []string, obs AssertedObservation) (string, string) {
	if len(entries) == 0 {
		return assertionResult(false, obs), ""
	}
	m := temporalParameters.FindStringSubmatch(entries[0])
	if m == nil {
		return "", "unrecognized temporal message: 2192"
	}
	params := []string{m[2], m[3], m[1]}
	return assertionResult(slices.Equal(params[:len(obs.Values)], obs.Values), obs), ""
}

func assertionResult(found bool, obs AssertedObservation) string {
	if found != obs.Present {
		return "values:" + obs.ID
	}
	return ""
}

func matchingNumbers(entry string, values []string) bool {
	numbers := regexp.MustCompile(`\b[0-9][0-9,]*\b`).FindAllString(entry, -1)
	var clean []string
	for _, number := range numbers {
		clean = append(clean, strings.ReplaceAll(number, ",", ""))
	}
	return len(values) <= len(clean) && slices.Equal(clean[:len(values)], values)
}
