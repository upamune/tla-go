package recorderdebug

import (
	"strings"
	"testing"
)

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
