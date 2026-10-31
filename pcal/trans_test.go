package pcal

import (
	"bytes"
	"strings"
	"testing"
)

func TestTranslateLines(t *testing.T) {
	lines := []string{
		"---- MODULE M ----",
		"(* --algorithm a",
		"variables x = 0;",
		"begin",
		"l: x := 1;",
		"end algorithm *)",
		"====",
	}
	var out bytes.Buffer
	got, ok := Translate(lines, NewParams(), &out)
	if !ok {
		t.Fatalf("translation failed: %s", out.String())
	}
	text := strings.Join(got, "\n")
	for _, want := range []string{
		`\* BEGIN TRANSLATION (chksum(pcal) = "`,
		"l == /\\ pc = \"l\"\n     /\\ x' = 1\n     /\\ pc' = \"Done\"",
		"\\* END TRANSLATION \n====",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("translation lacks %q:\n%s", want, text)
		}
	}
	if out.String() != "Parsing completed.\nTranslation completed.\n" {
		t.Errorf("messages %q", out.String())
	}
}

// The expected message is the one Java pcal.trans 1.11 prints.
func TestTranslateReportsErrors(t *testing.T) {
	var out bytes.Buffer
	_, ok := Translate([]string{"(* --algorithm a begin x := ; end algorithm *)"}, NewParams(), &out)
	want := "\nUnrecoverable error:\n -- Empty right-hand side of assignment at \n    line 1, column 26.\n\n"
	if ok || out.String() != want {
		t.Fatalf("ok %v, messages %q", ok, out.String())
	}
}
