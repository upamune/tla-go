package upstreamjunit

import "strings"

// maskJava preserves offsets and line endings when hiding comments and literals.
func maskJava(source string, literals bool) string {
	out := []byte(source)
	for i := 0; i < len(source); {
		end, literal := javaTokenEnd(source, i)
		if end == i {
			i++
			continue
		}
		if !literal || literals {
			blankJava(out, i, end)
		}
		i = end
	}
	return string(out)
}

func javaTokenEnd(s string, i int) (int, bool) {
	if s[i] == '"' || s[i] == '\'' {
		return javaLiteralEnd(s, i), true
	}
	if s[i] != '/' || i+1 == len(s) {
		return i, false
	}
	return javaCommentEnd(s, i), false
}

func javaCommentEnd(s string, i int) int {
	switch s[i+1] {
	case '/':
		if n := strings.IndexAny(s[i+2:], "\r\n"); n >= 0 {
			return i + 2 + n
		}
		return len(s)
	case '*':
		if n := strings.Index(s[i+2:], "*/"); n >= 0 {
			return i + 2 + n + 2
		}
		return len(s)
	}
	return i
}

func javaLiteralEnd(s string, i int) int {
	if strings.HasPrefix(s[i:], `"""`) {
		return javaTextBlockEnd(s, i+3)
	}
	quote := s[i]
	for i++; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			continue
		}
		if s[i] == quote {
			return i + 1
		}
	}
	return len(s)
}

func javaTextBlockEnd(s string, i int) int {
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			i += 2
			continue
		}
		if strings.HasPrefix(s[i:], `"""`) {
			return i + 3
		}
		i++
	}
	return len(s)
}

func blankJava(out []byte, start, end int) {
	for i := start; i < end; i++ {
		if out[i] != '\n' && out[i] != '\r' {
			out[i] = ' '
		}
	}
}
