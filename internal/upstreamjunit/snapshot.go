package upstreamjunit

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// SweepRecord describes one lexical candidate, not a JUnit test result.
type SweepRecord struct {
	Class       string   `json:"class"`
	Status      string   `json:"status"`
	Differences []string `json:"differences,omitempty"`
	Bodies      []string `json:"bodies,omitempty"`
	Presence    []string `json:"presence,omitempty"`
	JavaExit    *int     `json:"java_exit,omitempty"`
	GoExit      *int     `json:"go_exit,omitempty"`
	Unavailable string   `json:"unavailable,omitempty"`
}

type SweepCounts struct {
	Candidates    int `json:"candidates"`
	Attempted     int `json:"attempted"`
	Unsafe        int `json:"unsafe"`
	Compared      int `json:"compared"`
	NoDifferences int `json:"no_selected_differences"`
	Differences   int `json:"selected_differences"`
	Unavailable   int `json:"unavailable"`
}
type SweepSnapshot struct {
	Schema         int           `json:"schema"`
	TlaGoCommit    string        `json:"tla_go_commit"`
	UpstreamCommit string        `json:"upstream_commit"`
	JarSHA256      string        `json:"jar_sha256"`
	SourceSHA256   string        `json:"source_log_sha256"`
	Counts         SweepCounts   `json:"counts"`
	Classes        []SweepRecord `json:"classes"`
}

var (
	candidateLine   = regexp.MustCompile(`^unresolved ([A-Za-z0-9_.$]+) spec=`)
	comparedLine    = regexp.MustCompile(`^diagnostic ([A-Za-z0-9_.$]+) differences=\[([^]]*)\] selected bodies=\[([^]]*)\] presence=\[([^]]*)\]; partial comparison, liveness strategy unresolved, not a match; Java exit=(-?[0-9]+) Go exit=(-?[0-9]+)$`)
	unavailableLine = regexp.MustCompile(`^diagnostic ([A-Za-z0-9_.$]+) comparison unavailable: (.*)$`)
	summaryLine     = regexp.MustCompile(`^incomplete sweep: ([0-9]+) candidates with recovered spec, ([0-9]+) diagnostic runs, ([0-9]+) candidates with unsafe or unresolved setup, ([0-9]+) unresolved non-abstract candidates; inventory is not an exhaustive JUnit count$`)
	failureLine     = regexp.MustCompile(`^--- FAIL: TestUpstreamJUnitSweep \([0-9.]+s\)$`)
	numberID        = regexp.MustCompile(`^[0-9]+$`)
	diffID          = regexp.MustCompile(`^(exit|dump:(content|missing)|translation:[A-Za-z0-9_.-]+|message:[0-9]+|presence:[0-9]+)$`)
)

type sweepParser struct {
	records map[string]*SweepRecord
	counts  SweepCounts
	summary bool
}

// ParseSweepLog accepts only a complete full sweep. It never preserves tool output.
func ParseSweepLog(r io.Reader) (SweepSnapshot, error) {
	p := sweepParser{records: map[string]*SweepRecord{}}
	failed := false
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4096), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if failureLine.MatchString(line) {
			failed = true
		}
		_, msg, ok := strings.Cut(line, ": ")
		if !ok {
			continue
		}
		if err := p.consume(msg); err != nil {
			return SweepSnapshot{}, err
		}
	}
	if err := scanner.Err(); err != nil {
		return SweepSnapshot{}, err
	}
	if !p.summary || !failed {
		return SweepSnapshot{}, fmt.Errorf("missing summary or completion marker")
	}
	return p.result()
}

func (p *sweepParser) consume(msg string) error {
	switch {
	case strings.HasPrefix(msg, "unresolved "):
		m := candidateLine.FindStringSubmatch(msg)
		if m == nil {
			return fmt.Errorf("malformed candidate")
		}
		if _, exists := p.records[m[1]]; exists {
			return fmt.Errorf("duplicate candidate %s", m[1])
		}
		p.records[m[1]] = &SweepRecord{Class: m[1], Status: "unsafe_setup"}
	case strings.HasPrefix(msg, "diagnostic "):
		return p.diagnostic(msg)
	case strings.HasPrefix(msg, "incomplete sweep:"):
		return p.readSummary(msg)
	}
	return nil
}

func (p *sweepParser) diagnostic(msg string) error {
	if m := comparedLine.FindStringSubmatch(msg); m != nil {
		return p.compared(m)
	}
	if m := unavailableLine.FindStringSubmatch(msg); m != nil {
		return p.unavailable(m)
	}
	return fmt.Errorf("malformed diagnostic")
}

func (p *sweepParser) compared(m []string) error {
	rec, err := pending(p.records, m[1])
	if err != nil {
		return err
	}
	diffs, err := parseTokens(m[2], diffID)
	if err != nil {
		return err
	}
	bodies, err := parseTokens(m[3], numberID)
	if err != nil {
		return err
	}
	presence, err := parseTokens(m[4], numberID)
	if err != nil {
		return err
	}
	j, err := strconv.Atoi(m[5])
	if err != nil {
		return err
	}
	g, err := strconv.Atoi(m[6])
	if err != nil {
		return err
	}
	rec.Status = "compared"
	rec.Differences = diffs
	rec.Bodies = bodies
	rec.Presence = presence
	rec.JavaExit = &j
	rec.GoExit = &g
	return nil
}

func (p *sweepParser) unavailable(m []string) error {
	rec, err := pending(p.records, m[1])
	if err != nil {
		return err
	}
	switch {
	case m[2] == "java timed out: context deadline exceeded":
		rec.Unavailable = "java_timeout"
	case strings.HasSuffix(m[2], "/tla-go timed out: context deadline exceeded"):
		rec.Unavailable = "go_timeout"
	case m[2] == "go: no TLC tool messages":
		rec.Unavailable = "go_no_tool_messages"
	case m[2] == "translation failed: Java 255 Go 255":
		rec.Unavailable = "translation_failed_java_255_go_255"
	default:
		return fmt.Errorf("unknown unavailable reason for %s", m[1])
	}
	rec.Status = "unavailable"
	return nil
}

func (p *sweepParser) readSummary(msg string) error {
	if p.summary {
		return fmt.Errorf("duplicate summary")
	}
	m := summaryLine.FindStringSubmatch(msg)
	if m == nil {
		return fmt.Errorf("malformed summary")
	}
	nums := make([]int, 4)
	for i := range nums {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return err
		}
		nums[i] = n
	}
	p.counts.Candidates = nums[0]
	p.counts.Attempted = nums[1]
	p.counts.Unsafe = nums[2]
	if nums[3] != p.counts.Candidates {
		return fmt.Errorf("unresolved count differs from candidate count")
	}
	p.summary = true
	return nil
}

func (p *sweepParser) result() (SweepSnapshot, error) {
	out := SweepSnapshot{Counts: p.counts}
	for _, rec := range p.records {
		out.Classes = append(out.Classes, *rec)
		switch rec.Status {
		case "compared":
			out.Counts.Compared++
			if len(rec.Differences) == 0 {
				out.Counts.NoDifferences++
			} else {
				out.Counts.Differences++
			}
		case "unavailable":
			out.Counts.Unavailable++
		}
	}
	sort.Slice(out.Classes, func(i, j int) bool { return out.Classes[i].Class < out.Classes[j].Class })
	c := p.counts
	if len(p.records) != c.Candidates || c.Attempted != out.Counts.Compared+out.Counts.Unavailable || c.Unsafe != c.Candidates-c.Attempted {
		return SweepSnapshot{}, fmt.Errorf("inconsistent source counts")
	}
	return out, nil
}

func pending(records map[string]*SweepRecord, name string) (*SweepRecord, error) {
	rec, ok := records[name]
	if !ok {
		return nil, fmt.Errorf("diagnostic without candidate %s", name)
	}
	if rec.Status != "unsafe_setup" {
		return nil, fmt.Errorf("duplicate diagnostic %s", name)
	}
	return rec, nil
}

func parseTokens(s string, pattern *regexp.Regexp) ([]string, error) {
	var out []string
	seen := map[string]bool{}
	for v := range strings.FieldsSeq(s) {
		if !pattern.MatchString(v) {
			return nil, fmt.Errorf("invalid token %q", v)
		}
		if !seen[v] {
			out = append(out, v)
			seen[v] = true
		}
	}
	sort.Strings(out)
	return out, nil
}
