package upstreamjunit

import (
	"os"
	"slices"
	"testing"
	"time"

	"github.com/upamune/tla-go/internal/recorderdebug"
)

func runCoverageRecorder(t *testing.T, binary, modelDir, recorderPath string, args []string) (Result, []bridgeRecord, []string) {
	t.Helper()
	result, err := runCommandWithEnv(modelDir, 30*time.Second, []string{"TLA_GO_DEBUG_RECORDER=" + recorderPath}, binary, args...)
	if err != nil || result.ExitCode != 0 || result.TimedOut {
		t.Fatalf("coverage invocation: result %+v error %v", result, err)
	}
	messages, err := Messages(result.Output)
	if err != nil || len(messages["2201"]) != 1 || len(messages["2202"]) != 1 || len(messages["2776"]) != 0 {
		t.Fatalf("coverage report incomplete: error %v output %s", err, result.Output)
	}
	projected, omissions, err := projectBridge(result.Output)
	if err != nil {
		t.Fatal(err)
	}
	recorderFile, err := os.Open(recorderPath)
	if err != nil {
		t.Fatal(err)
	}
	defer recorderFile.Close()
	events, err := recorderdebug.Decode(recorderFile)
	if err != nil {
		t.Fatal(err)
	}
	records, err := coverageRecorderRecords(projected, events)
	if err != nil {
		t.Fatal(err)
	}
	return result, records, omissions
}

func TestCoverageRecorderRecords(t *testing.T) {
	projected := []bridgeRecord{{"2772", []string{"<Next>", "1", "2"}}, {"2221", []string{"|line 3", "4"}}}
	events := []recorderdebug.Event{
		{Kind: "record", Array: "strings", Code: 2220},
		{Kind: "record", Array: "strings", Code: 2219},
		{Kind: "record", Array: "strings", Code: 2772, Values: []string{"<Next>", "1", "2"}},
		{Kind: "record", Array: "strings", Code: 2221, Values: []string{"|line 3", "4"}},
	}
	records, err := coverageRecorderRecords(projected, events)
	if err != nil || len(records) != 2 || !slices.Equal(records[0].values, events[2].Values) {
		t.Fatalf("records=%+v error=%v", records, err)
	}
	events[2].Values[0] = "changed"
	if records[0].values[0] != "<Next>" {
		t.Fatal("record aliases the event values")
	}
	events[2].Values[0] = "<Next>"
	changedProjection := slices.Clone(projected)
	changedProjection[0].values = []string{"<Next>", "999", "2"}
	if _, err := coverageRecorderRecords(changedProjection, events); err == nil {
		t.Fatal("accepted a tool projection that disagrees with the typed event")
	}
	for _, bad := range [][]recorderdebug.Event{
		events[:1], events[:3], append(slices.Clone(events), events[3]),
		{events[1], events[0], events[2], events[3]},
		{{Kind: "omission", Code: 2220}, events[1], events[2], events[3]},
		{events[0], events[1], events[3], events[2]},
		{events[0], events[1], {Kind: "record", Array: "objects", Code: 2772, Values: events[2].Values}, events[3]},
		{events[0], events[1], {Kind: "omission", Code: 2772}, events[3]},
		{events[0], events[1], {Kind: "record", Array: "strings", Code: 2772, Values: []string{"<Next>", "999", "2"}}, events[3]},
	} {
		if _, err := coverageRecorderRecords(projected, bad); err == nil {
			t.Fatalf("accepted inconsistent events %+v", bad)
		}
	}
}
