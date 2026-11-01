package upstreamjunit

import (
	"reflect"
	"strings"
	"testing"
)

func TestRecorderEncodingRejectsLossyValues(t *testing.T) {
	for _, record := range []recorderRecord{
		{code: "9999", array: "strings"},
		{code: "2199", array: "unknown"},
		{code: "2199", array: "strings", values: []recorderValue{{"i", "1"}}},
		{code: "2199", array: "objects", values: []recorderValue{{"state", "x"}}},
		{code: "2199", array: "objects", values: []recorderValue{{"i", "2147483648"}}},
		{code: "2199", array: "objects", values: []recorderValue{{"l", "01"}}},
		{code: "2199", array: "objects", values: []recorderValue{{"b", "TRUE"}}},
		{code: "2199", array: "objects", values: []recorderValue{{"n", "value"}}},
		{code: "2199", array: "objects", values: []recorderValue{{"s", string([]byte{0xff})}}},
	} {
		if _, err := encodeRecorder([]recorderRecord{record}); err == nil {
			t.Errorf("accepted %+v", record)
		}
	}
}

func TestBridgeProjectionPreservesOrderAndDuplicates(t *testing.T) {
	output := msg("2772", "<A line 1>: 1:2") + msg("2221", "  |line 2: 3") + msg("2772", "<B line 3>: 2:4") + msg("2221", "  |line 2: 3") + msg("2775", "  line 4: 3:7") + msg("2194", "The depth of the complete state graph search is 2.")
	records, omissions, err := projectBridge(output)
	want := []bridgeRecord{
		{"2772", []string{"<A line 1>", "1", "2"}},
		{"2221", []string{"|line 2", "3"}},
		{"2772", []string{"<B line 3>", "2", "4"}},
		{"2221", []string{"|line 2", "3"}},
		{"2775", []string{"line 4", "3", "7"}},
		{"2194", []string{"2"}},
	}
	if err != nil || len(omissions) != 1 || !strings.Contains(omissions[0], "do not establish recorder") || !reflect.DeepEqual(records, want) {
		t.Fatalf("records %v, omissions %v, error %v", records, omissions, err)
	}
}

func TestBridgeProjectionNestedOrderIsUnproven(t *testing.T) {
	output := "@!@!@STARTMSG 1000:1 @!@!@\n" + msg("2194", "The depth of the complete state graph search is 2.") + "@!@!@ENDMSG 1000 @!@!@\n"
	_, omissions, err := projectBridge(output)
	if err != nil || !strings.Contains(strings.Join(omissions, ";"), "nested tool messages") {
		t.Fatalf("omissions %v error %v", omissions, err)
	}
	if bridgeVerdict(nil, omissions) == nil {
		t.Fatal("nested projection counted as a full pass")
	}
}

func TestBridgeProjectionPreservesPresenceWithoutInventingArguments(t *testing.T) {
	output := msg("1000", "error one") + msg("2194", "The depth of the complete state graph search is 2.") + msg("2776", "mismatch") + msg("1000", "error two")
	records, omissions, err := projectBridge(output)
	want := []bridgeRecord{{"1000", nil}, {"2194", []string{"2"}}, {"2776", nil}, {"1000", nil}}
	if err != nil || !reflect.DeepEqual(records, want) {
		t.Fatalf("records %v omissions %v error %v", records, omissions, err)
	}
	joined := strings.Join(omissions, ";")
	if strings.Count(joined, "presence only; recorder arguments unavailable for message 1000") != 2 || !strings.Contains(joined, "presence only; recorder arguments unavailable for message 2776") {
		t.Fatalf("missing presence-only omissions: %v", omissions)
	}
	if bridgeVerdict(nil, omissions) == nil {
		t.Fatal("presence-only projection counted as a full pass")
	}
}
