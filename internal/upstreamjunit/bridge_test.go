package upstreamjunit

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestBridgeProjectionRejectsUnsupported(t *testing.T) {
	output := "@!@!@STARTMSG 2199:0 @!@!@\n2 states generated, 1 distinct states found, 0 states left on queue.\n@!@!@ENDMSG 2199 @!@!@\n" +
		"@!@!@STARTMSG 9999:0 @!@!@\nunknown\n@!@!@ENDMSG 9999 @!@!@\n" +
		"@!@!@STARTMSG 2190:0 @!@!@\nmalformed\n@!@!@ENDMSG 2190 @!@!@\n"
	records, omissions, err := projectBridge(output)
	if err != nil || len(records) != 1 || len(omissions) != 3 || !strings.Contains(strings.Join(omissions, ","), "9999") {
		t.Fatalf("records=%v omissions=%v err=%v", records, omissions, err)
	}
	if _, err := encodeBridge([]bridgeRecord{{code: "9999"}}); err == nil {
		t.Fatal("unknown record accepted")
	}
}

func TestBridgeProjectionTruncatedMarkers(t *testing.T) {
	valid := msg("2199", "2 states generated, 1 distinct states found, 0 states left on queue.")
	for _, marker := range []string{"@!@!@STARTMSG 2199:0 @!@!@", "@!@!@ENDMSG 2199 @!@!@"} {
		for n := 1; n < len(marker); n++ {
			t.Run(fmt.Sprintf("%s/%d", marker[5:8], n), func(t *testing.T) {
				if _, _, err := projectBridge(valid + marker[:n]); err == nil {
					t.Fatalf("accepted truncated marker %q", marker[:n])
				}
			})
		}
	}
	// A cut-off end marker inside an open message must also fail.
	start := "@!@!@STARTMSG 2199:0 @!@!@\nbody\n"
	end := "@!@!@ENDMSG 2199 @!@!@"
	for n := 0; n < len(end); n++ {
		if _, _, err := projectBridge(start + end[:n]); err == nil {
			t.Errorf("accepted truncated end %q", end[:n])
		}
	}
}

func TestBridgeProjectionOrdinaryMarkerLikeText(t *testing.T) {
	for _, body := range []string{
		"an email ends in @", "symbols @!@! in the middle", "@!@!\nordinary text",
		"STARTMSG 1:0 and ENDMSG 1", "@!@?", "text ending in @!@!",
	} {
		t.Run(body, func(t *testing.T) {
			output := msg("9999", body) + "ordinary text ending in @!@!"
			_, omissions, err := projectBridge(output)
			if err != nil || len(omissions) != 2 {
				t.Fatalf("omissions=%v err=%v", omissions, err)
			}
		})
	}
}

func TestBridgeCannotPassWithoutCompleteness(t *testing.T) {
	if err := bridgeVerdict(nil, nil); err == nil {
		t.Fatal("successful method without coverage and lifecycle was counted as a pass")
	}
	if err := bridgeVerdict(nil, []string{"missing observation"}); err == nil {
		t.Fatal("missing observation was counted as a pass")
	}
}

func TestBridgeInitialProgressProjection(t *testing.T) {
	for _, tc := range []struct {
		body string
		want bool
	}{
		{"Computed 2 initial states...", true},
		{"Computed two initial states...", false},
		{"Computed 2 initial states.", false},
	} {
		records, omissions, err := projectBridge(msg("2269", tc.body))
		if err != nil || (len(records) == 1) != tc.want {
			t.Fatalf("body %q: records=%v omissions=%v error=%v", tc.body, records, omissions, err)
		}
		if tc.want && (records[0].code != "2269" || len(records[0].values) != 1 || records[0].values[0] != "2") {
			t.Fatalf("body %q: records=%v", tc.body, records)
		}
	}
}

func TestBridgeTemporalProjection(t *testing.T) {
	for _, tc := range []struct {
		id, body string
		values   []string
	}{
		{"2212", "Implied-temporal checking--satisfiability problem has 2 branches.", []string{"2"}},
		{"2192", "Checking temporal properties for the complete state space with 4 total distinct states at (2026-11-02 01:02:03)", []string{"complete", "4", ""}},
		{"2192", "Checking 2 branches of temporal properties for the complete state space with 8 total distinct states at (2026-11-02 01:02:03)", []string{"complete", "8", "2 branches of "}},
	} {
		record, omission := projectMessage(tc.id, tc.body)
		if record.code != tc.id || !slices.Equal(record.values, tc.values) || omission != "" {
			t.Fatalf("body %q: record=%+v omission=%q", tc.body, record, omission)
		}
	}
	for _, tc := range []struct{ id, body string }{
		{"2212", "Implied-temporal checking--satisfiability problem has two branches."},
		{"2192", "Checking temporal properties for the current state space with 4 total distinct states at (2026-11-02 01:02:03)"},
	} {
		if record, omission := projectMessage(tc.id, tc.body); record.code != "" || omission == "" {
			t.Fatalf("accepted unsupported temporal message: %+v %q", record, omission)
		}
	}
}
