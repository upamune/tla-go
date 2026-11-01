package recorderdebug

import (
	"strings"
	"testing"
)

func TestDecodeHumanFinishDuration(t *testing.T) {
	header := `{"kind":"header","version":1,"complete":false}` + "\n"
	for _, tc := range []struct {
		duration string
		valid    bool
	}{
		{"00s", true},
		{"01min 00s", true},
		{"01h 00min", true},
		{"1d 00h", true},
		{"0ms", true},
		{"10ms", true},
		{"01ms", false},
		{"60s", false},
		{"0d 00h", false},
		{"1d 24h", false},
		{"367d 00h", false},
		{"garbage", false},
	} {
		line := `{"kind":"record","complete":false,"code":2186,"array":"strings","values":["` + tc.duration + `"]}` + "\n"
		_, err := Decode(strings.NewReader(header + line))
		if (err == nil) != tc.valid {
			t.Errorf("duration %q: %v", tc.duration, err)
		}
	}
}

func TestDecodePartialEvents(t *testing.T) {
	header := `{"kind":"header","version":1,"complete":false}` + "\n"
	stats := `{"kind":"record","complete":false,"code":2199,"array":"strings","values":["0","0","0"]}` + "\n"
	finish := `{"kind":"record","complete":false,"code":2186,"array":"strings","values":["1ms"]}` + "\n"
	events, err := Decode(strings.NewReader(header + stats + finish))
	if err != nil || len(events) != 2 || events[0].Code != 2199 || events[1].Code != 2186 {
		t.Fatalf("events=%+v error=%v", events, err)
	}
	for _, bad := range []string{
		"", stats, strings.TrimSuffix(header+stats, "\n"),
		strings.Replace(header, `"complete":false`, `"complete":true`, 1),
		strings.Replace(header, `,"complete":false`, ``, 1),
		strings.Replace(header, `"complete":false`, `"complete":null`, 1),
		strings.Replace(header, `"complete":false`, `"complete":true,"complete":false`, 1),
		strings.Replace(header, `"complete":false`, `"complete":true,"Complete":false`, 1),
		strings.Replace(header, `"complete":false`, `"complete":false,"code":0`, 1),
		header + strings.Replace(stats, `"strings"`, `"objects"`, 1),
		header + strings.Replace(stats, `"complete":false`, `"complete":false,"version":0`, 1),
		header + strings.Replace(stats, `2199`, `1000`, 1),
		header + strings.Replace(stats, `"0","0","0"`, `1,"0","0"`, 1),
		header + strings.Replace(stats, `"0","0","0"`, `null,"0","0"`, 1),
		header + strings.Replace(stats, `"0","0","0"`, `"\ud800","0","0"`, 1),
		header + strings.Replace(stats, `"0","0","0"`, `"01","0","0"`, 1),
		header + stats + `{}` + "\n",
		header + strings.Repeat(" ", 1<<20),
	} {
		if _, err := Decode(strings.NewReader(bad)); err == nil {
			t.Fatalf("accepted invalid recorder stream %q", bad[:min(len(bad), 128)])
		}
	}
}

func TestDecodeRejectsCaseAliases(t *testing.T) {
	header := `{"kind":"header","version":1,"complete":false}` + "\n"
	stats := `{"kind":"record","complete":false,"code":2199,"array":"strings","values":["0","0","0"]}` + "\n"
	finish := `{"kind":"record","complete":false,"code":2186,"array":"strings","values":["1ms"]}` + "\n"
	for _, line := range []struct {
		prefix, body string
		fields       []string
	}{
		{"", header, []string{"kind", "version", "complete"}},
		{header, stats, []string{"kind", "complete", "code", "array", "values"}},
		{header, finish, []string{"kind", "complete", "code", "array", "values"}},
	} {
		for _, field := range line.fields {
			alias := strings.ToUpper(field[:1]) + field[1:]
			key, alternate := `"`+field+`":`, `"`+alias+`":`
			changed := strings.Replace(line.body, key, alternate, 1)
			doubled := strings.TrimSuffix(line.body, "}\n") + "," + alternate + "null}\n"
			for _, body := range []string{changed, doubled} {
				if _, err := Decode(strings.NewReader(line.prefix + body)); err == nil {
					t.Fatalf("accepted case alias %q in %s", field, body)
				}
			}
		}
	}
}

func TestDecodeSearchRecords(t *testing.T) {
	header := `{"kind":"header","version":1,"complete":false}` + "\n"
	for _, record := range []string{
		`{"kind":"record","complete":false,"code":2220,"array":"strings","values":[]}`,
		`{"kind":"record","complete":false,"code":2219,"array":"strings","values":[]}`,
		`{"kind":"record","complete":false,"code":2185,"array":"strings","values":[]}`,
		`{"kind":"record","complete":false,"code":2189,"array":"strings","values":[]}`,
		`{"kind":"record","complete":false,"code":2190,"array":"strings","values":["0","s"]}`,
		`{"kind":"record","complete":false,"code":2190,"array":"strings","values":["1",""]}`,
		`{"kind":"record","complete":false,"code":2191,"array":"strings","values":["2","s","1"]}`,
		`{"kind":"record","complete":false,"code":2193,"array":"strings","values":["val = 5.4E-20"]}`,
		`{"kind":"record","complete":false,"code":2193,"array":"strings","values":["val = 0.0"]}`,
		`{"kind":"record","complete":false,"code":2194,"array":"strings","values":["1"]}`,
		`{"kind":"record","complete":false,"code":2269,"array":"strings","values":["2"]}`,
	} {
		if _, err := Decode(strings.NewReader(header + record + "\n")); err != nil {
			t.Errorf("valid event %s: %v", record, err)
		}
	}
	for _, record := range []string{
		`{"kind":"record","complete":false,"code":2189,"array":"strings","values":null}`,
		`{"kind":"record","complete":false,"code":2189,"array":"strings","values":[""]}`,
		`{"kind":"record","complete":false,"code":2190,"array":"strings","values":["0",""]}`,
		`{"kind":"record","complete":false,"code":2190,"array":"strings","values":["1","s"]}`,
		`{"kind":"record","complete":false,"code":2191,"array":"strings","values":["2","s","x"]}`,
		`{"kind":"record","complete":false,"code":2193,"array":"strings","values":["val = NaN"]}`,
		`{"kind":"record","complete":false,"code":2193,"array":"strings","values":["val = +Inf"]}`,
		`{"kind":"record","complete":false,"code":2193,"array":"strings","values":["val = -1"]}`,
		`{"kind":"record","complete":false,"code":2193,"array":"strings","values":["val = -0.0"]}`,
		`{"kind":"record","complete":false,"code":2193,"array":"strings","values":["val = 1e-20"]}`,
		`{"kind":"record","complete":false,"code":2193,"array":"strings","values":["0.0"]}`,
		`{"kind":"record","complete":false,"code":2194,"array":"strings","values":[null]}`,
	} {
		if _, err := Decode(strings.NewReader(header + record + "\n")); err == nil {
			t.Errorf("accepted invalid event %s", record)
		}
	}
}

func TestDecodeCoverageRecords(t *testing.T) {
	header := `{"kind":"header","version":1,"complete":false}` + "\n"
	for _, record := range []string{
		`{"kind":"record","complete":false,"code":2221,"array":"strings","values":["|line 3, col 1 of module M","0"]}`,
		`{"kind":"record","complete":false,"code":2772,"array":"strings","values":["<Next line 3 of module M>","1","2"]}`,
		`{"kind":"record","complete":false,"code":2773,"array":"strings","values":["<Init line 1 of module M>","1","1"]}`,
		`{"kind":"record","complete":false,"code":2774,"array":"strings","values":["<Inv line 5 of module M>"]}`,
		`{"kind":"record","complete":false,"code":2775,"array":"strings","values":["||line 3 of module M","3","4"]}`,
	} {
		if _, err := Decode(strings.NewReader(header + record + "\n")); err != nil {
			t.Errorf("valid event %s: %v", record, err)
		}
	}
	for _, record := range []string{
		`{"kind":"record","complete":false,"code":2221,"array":"strings","values":["","0"]}`,
		`{"kind":"record","complete":false,"code":2221,"array":"strings","values":["line\nbreak","0"]}`,
		`{"kind":"record","complete":false,"code":2772,"array":"strings","values":["location","-1","2"]}`,
		`{"kind":"record","complete":false,"code":2773,"array":"strings","values":["location","1"]}`,
		`{"kind":"record","complete":false,"code":2774,"array":"objects","values":["location"]}`,
		`{"kind":"record","complete":false,"code":2775,"array":"strings","values":["location","01","2"]}`,
	} {
		if _, err := Decode(strings.NewReader(header + record + "\n")); err == nil {
			t.Errorf("accepted invalid event %s", record)
		}
	}
}

func TestDecodeTemporalRecords(t *testing.T) {
	header := `{"kind":"header","version":1,"complete":false}` + "\n"
	for _, record := range []string{
		`{"kind":"record","complete":false,"code":2212,"array":"strings","values":["1"]}`,
		`{"kind":"record","complete":false,"code":2192,"array":"strings","values":["complete","4",""]}`,
		`{"kind":"record","complete":false,"code":2192,"array":"strings","values":["complete","8","2 branches of "]}`,
	} {
		if _, err := Decode(strings.NewReader(header + record + "\n")); err != nil {
			t.Errorf("valid temporal event %s: %v", record, err)
		}
	}
	for _, record := range []string{
		`{"kind":"record","complete":false,"code":2212,"array":"objects","values":["1"]}`,
		`{"kind":"record","complete":false,"code":2212,"array":"strings","values":["01"]}`,
		`{"kind":"record","complete":false,"code":2192,"array":"strings","values":["current","4",""]}`,
		`{"kind":"record","complete":false,"code":2192,"array":"strings","values":["complete","-1",""]}`,
		`{"kind":"record","complete":false,"code":2192,"array":"strings","values":["complete","4","1 branches of "]}`,
		`{"kind":"record","complete":false,"code":2192,"array":"strings","values":["complete","4","02 branches of "]}`,
		`{"kind":"record","complete":false,"code":2192,"array":"strings","values":["complete","4"]}`,
	} {
		if _, err := Decode(strings.NewReader(header + record + "\n")); err == nil {
			t.Errorf("accepted invalid temporal event %s", record)
		}
	}
}

func TestDecodeSuccessProbabilityOmission(t *testing.T) {
	header := `{"kind":"header","version":1,"complete":false}` + "\n"
	omission := `{"kind":"omission","complete":false,"code":2193,"reason":"observed-fingerprint-probability-unavailable"}` + "\n"
	events, err := Decode(strings.NewReader(header + omission))
	if err != nil || len(events) != 1 || events[0].Kind != "omission" || events[0].Code != 2193 || events[0].Reason != SuccessProbabilityOmission {
		t.Fatalf("events=%+v error=%v", events, err)
	}
	for _, bad := range []string{
		strings.Replace(omission, `2193`, `2199`, 1),
		strings.Replace(omission, `"complete":false`, `"complete":true`, 1),
		strings.Replace(omission, `observed-fingerprint-probability-unavailable`, `unknown`, 1),
		strings.Replace(omission, `"reason"`, `"Reason"`, 1),
		strings.Replace(omission, `"reason"`, `"reason":"fake","reason"`, 1),
		strings.Replace(omission, `"reason"`, `"values":[],"reason"`, 1),
	} {
		if _, err := Decode(strings.NewReader(header + bad)); err == nil {
			t.Fatalf("accepted invalid omission %q", bad)
		}
	}
}

func TestDecodeRejectsInvalidatedTemporalStart(t *testing.T) {
	stream := `{"kind":"header","version":1,"complete":false}` + "\n" +
		`{"kind":"record","complete":false,"code":2192,"array":"strings","values":["complete","3",""]}` + "\n" +
		`{"kind":"omission","complete":false,"code":2192,"reason":"temporal-check-start-invalidated"}` + "\n"
	if events, err := Decode(strings.NewReader(stream)); err == nil || events != nil {
		t.Fatalf("invalidated event accepted: %v, %v", events, err)
	}
}
