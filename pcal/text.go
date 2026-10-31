package pcal

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// logger prints messages as Java's PcalDebug does.
type logger struct{ w io.Writer }

func (l *logger) info(s string) { _, _ = io.WriteString(l.w, s+"\n") }

func (l *logger) warning(s string) { l.info("Warning: " + s) }

func (l *logger) error(s string) { l.info("\nUnrecoverable error:\n -- " + s + ".\n") }

func (l *logger) report(e *Error) { l.error(e.report()) }

// javaLines splits text into lines like BufferedReader.readLine, which
// accepts "\n", "\r", and "\r\n" line ends.
func javaLines(text string) []string {
	var lines []string
	for text != "" {
		i := strings.IndexAny(text, "\r\n")
		if i < 0 {
			return append(lines, text)
		}
		lines = append(lines, text[:i])
		if text[i] == '\r' && i+1 < len(text) && text[i+1] == '\n' {
			i++
		}
		text = text[i+1:]
	}
	return lines
}

// resolve returns the path of a file name relative to the directory dir;
// an empty dir stands for the current directory.
func resolve(dir, name string) string {
	if dir == "" || filepath.IsAbs(name) {
		return name
	}
	return filepath.Join(dir, name)
}

func readJavaLines(path string) ([]string, error) {
	data, err := readFile(path)
	if err != nil {
		return nil, err
	}
	return javaLines(decodeJava(data)), nil
}

// openError is a failure to open a file for reading. Java's FileInputStream
// reports each of them as a FileNotFoundException, even for a directory.
type openError struct{ err error }

func (e *openError) Error() string { return e.err.Error() }

func (e *openError) Unwrap() error { return e.err }

var errIsDirectory = errors.New("is a directory")

// readFile reads a file and wraps the failures to open it in openError.
func readFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, &openError{err}
	}
	defer f.Close()
	if info, err := f.Stat(); err == nil && info.IsDir() {
		return nil, &openError{&fs.PathError{Op: "open", Path: path, Err: errIsDirectory}}
	}
	return io.ReadAll(f)
}

func writeJavaLines(path string, lines []string, eol string) error {
	var sb strings.Builder
	for _, line := range lines {
		sb.WriteString(line)
		sb.WriteString(eol)
	}
	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// removeTabs replaces tabs by spaces up to the next multiple of 8 and adds a
// trailing space to every line, as trans.removeTabs does.
func removeTabs(input []string) []string {
	result := make([]string, len(input))
	for i, line := range input {
		var sb strings.Builder
		n := 0
		for _, r := range line {
			if r == '\t' {
				pad := 8 - n%8
				sb.WriteString(spaces(pad))
				n += pad
				continue
			}
			sb.WriteRune(r)
			n += jlen(string(r))
		}
		sb.WriteString(" ")
		result[i] = sb.String()
	}
	return result
}

// findTokenPair returns the first line at or after from that contains tok1
// followed by spaces and tok2, or -1.
func findTokenPair(lines []string, from int, tok1, tok2 string) int {
	for i := from; i < len(lines); i++ {
		line := lines[i]
		col := strings.Index(line, tok1)
		if col == -1 {
			continue
		}
		next := col + len(tok1)
		for next < len(line) && line[next] == ' ' {
			next++
		}
		if next < len(line) && next == strings.Index(line, tok2) {
			return i
		}
	}
	return -1
}

// uncomment removes comments from the algorithm, replacing (* *) comments
// by spaces so that tokens keep their columns. It stops at the "*)" that
// ends the comment containing the algorithm.
func uncomment(lines []string, line, col int) {
	u := &uncommenter{}
	prefix := toUnits(lines[line])[:col]
	for ; !u.done && line < len(lines); line++ {
		out := u.line(append([]uint16(nil), prefix...), toUnits(lines[line]), col)
		if u.inString {
			fail("Unterminated string in line " + strconv.Itoa(line+1))
		}
		lines[line] = fromUnits(out)
		prefix, col = nil, 0
	}
}

type uncommenter struct {
	depth    int
	inString bool
	done     bool
}

func (u *uncommenter) line(out, in []uint16, col int) []uint16 {
	u.inString = false
	for ; !u.done && col < len(in); col++ {
		c := in[col]
		next := uint16(0)
		if col < len(in)-1 {
			next = in[col+1]
		}
		var emitted []uint16
		emitted, col = u.char(in, col, c, next)
		out = append(out, emitted...)
	}
	return out
}

// char processes the character at col and returns the characters to emit
// and the column of the last character consumed.
func (u *uncommenter) char(in []uint16, col int, c, next uint16) ([]uint16, int) {
	if !u.inString && c == '(' && next == '*' {
		u.depth++
		return []uint16{' ', ' '}, col + 1
	}
	if !u.inString && c == '*' && next == ')' {
		return u.closeComment(), col + 1
	}
	if u.depth != 0 {
		return []uint16{' '}, col
	}
	return u.code(in, col, c, next)
}

func (u *uncommenter) closeComment() []uint16 {
	if u.depth == 0 {
		u.done = true
		return []uint16{'*', ')'}
	}
	u.depth--
	return []uint16{' ', ' '}
}

func (u *uncommenter) code(in []uint16, col int, c, next uint16) ([]uint16, int) {
	if u.inString {
		if c == '"' {
			u.inString = false
		} else if c == '\\' && col < len(in)-1 {
			return []uint16{c, next}, col + 1
		}
		return []uint16{c}, col
	}
	if c == '\\' && next == '*' {
		return []uint16{' '}, len(in)
	}
	if c == '"' {
		u.inString = true
	}
	return []uint16{c}, col
}
