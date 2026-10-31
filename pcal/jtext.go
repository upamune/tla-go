package pcal

import (
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Java strings are sequences of UTF-16 code units, and the translator's
// column numbers count those units. The helpers in this file let the port
// compute the same columns for non-ASCII input.

func toUnits(s string) []uint16 { return utf16.Encode([]rune(s)) }

func fromUnits(u []uint16) string { return string(utf16.Decode(u)) }

// encodedText returns the text that Java's UTF-8 encoder writes for u. Like
// fromUnits, it decodes surrogate pairs, but it writes an unpaired surrogate
// as '?' rather than U+FFFD. The tokenizer reads one unit at a time, so its
// error messages can quote half of a pair.
func encodedText(u []uint16) string {
	var sb strings.Builder
	for i := 0; i < len(u); i++ {
		r := rune(u[i])
		if utf16.IsSurrogate(r) {
			r, i = surrogatePair(u, i)
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

// surrogatePair decodes the pair that starts at u[i] and returns the index
// of its second unit. For an unpaired surrogate it returns '?' and i.
func surrogatePair(u []uint16, i int) (rune, int) {
	if i+1 < len(u) {
		if r := utf16.DecodeRune(rune(u[i]), rune(u[i+1])); r != utf8.RuneError {
			return r, i + 1
		}
	}
	return '?', i
}

// jlen returns the length of s as Java's String.length would.
func jlen(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}

// jtrim mirrors String.trim, which removes code units up to U+0020.
func jtrim(s string) string {
	return strings.TrimFunc(s, func(r rune) bool { return r <= ' ' })
}

// jindex mirrors String.indexOf and returns a UTF-16 index or -1.
func jindex(s, sub string) int {
	before, _, ok := strings.Cut(s, sub)
	if !ok {
		return -1
	}
	return jlen(before)
}

// jsubstr mirrors String.substring(begin) with a UTF-16 index.
func jsubstr(s string, begin int) string {
	return fromUnits(toUnits(s)[begin:])
}

// decodeJava decodes UTF-8 like Java's default charset, replacing each
// maximal invalid subsequence with U+FFFD.
func decodeJava(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	var sb strings.Builder
	for len(b) > 0 {
		r, n := utf8.DecodeRune(b)
		if r == utf8.RuneError && n <= 1 {
			n = invalidPrefix(b)
		}
		sb.WriteRune(r)
		b = b[n:]
	}
	return sb.String()
}

// invalidPrefix returns the length of the maximal subpart of an ill-formed
// UTF-8 sequence starting at b[0].
func invalidPrefix(b []byte) int {
	need, lo, hi := leadInfo(b[0])
	n := 1
	for n <= need && n < len(b) && b[n] >= lo && b[n] <= hi {
		n++
		lo, hi = 0x80, 0xBF
	}
	return n
}

// utf8Lead describes lead bytes that need the same continuation bytes.
type utf8Lead struct {
	lo, hi      byte
	need        int
	first, last byte
}

var utf8Leads = []utf8Lead{
	{0xC2, 0xDF, 1, 0x80, 0xBF},
	{0xE0, 0xE0, 2, 0xA0, 0xBF},
	{0xED, 0xED, 2, 0x80, 0x9F},
	{0xE1, 0xEF, 2, 0x80, 0xBF},
	{0xF0, 0xF0, 3, 0x90, 0xBF},
	{0xF4, 0xF4, 3, 0x80, 0x8F},
	{0xF1, 0xF3, 3, 0x80, 0xBF},
}

// leadInfo returns the number of continuation bytes required by a lead byte
// and the valid range of the first continuation byte.
func leadInfo(c byte) (int, byte, byte) {
	for _, l := range utf8Leads {
		if c >= l.lo && c <= l.hi {
			return l.need, l.first, l.last
		}
	}
	return 0, 0, 0
}
