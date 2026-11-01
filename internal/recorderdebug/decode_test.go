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
