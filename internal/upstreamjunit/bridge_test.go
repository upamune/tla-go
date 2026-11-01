package upstreamjunit

import (
	"fmt"
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
