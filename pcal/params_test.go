package pcal

import (
	"testing"
	"unicode"
)

// TestDigitRuns checks the property of unicode.Nd that asciiDigit relies on.
func TestDigitRuns(t *testing.T) {
	for _, rg := range unicode.Nd.R16 {
		if rg.Stride != 1 || (rg.Hi-rg.Lo+1)%10 != 0 {
			t.Errorf("range %#x-%#x with stride %d is not a sequence of runs of ten digits", rg.Lo, rg.Hi, rg.Stride)
		}
	}
	for _, r := range "0123456789" {
		if got := asciiDigit(r); got != r {
			t.Errorf("asciiDigit(%q) = %q", r, got)
		}
	}
	for r, want := range map[rune]rune{'٣': '3', '８': '8', '९': '9', 'x': 'x', '𝟖': '𝟖'} {
		if got := asciiDigit(r); got != want {
			t.Errorf("asciiDigit(%q) = %q, want %q", r, got, want)
		}
	}
}
