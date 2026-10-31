package pcal

// endOfInput is the character PcalCharReader returns past the end of input.
const endOfInput = '\t'

// charReader ports PcalCharReader. Lines hold UTF-16 code units, so columns
// match Java's char indices. The translator removes tabs before reading, so
// the virtual and actual columns always agree.
type charReader struct {
	lines             [][]uint16
	line, column      int
	cur               []uint16
	hasCur            bool
	lastLine, lastCol int
}

func newCharReader(lines [][]uint16, firstLine, firstCol, lastLine, lastCol int) *charReader {
	r := &charReader{lines: lines, line: firstLine, column: firstCol, lastLine: lastLine, lastCol: lastCol}
	if firstLine < len(lines) {
		r.cur, r.hasCur = lines[firstLine], true
	}
	return r
}

func (r *charReader) lineNumber() int { return r.line }

func (r *charReader) columnNumber() int { return r.column }

func (r *charReader) atLimit() bool {
	return r.line > r.lastLine || (r.line == r.lastLine && r.column >= r.lastCol)
}

// next returns the next character, '\n' at the end of each line, and
// endOfInput when no input remains.
func (r *charReader) next() uint16 {
	if !r.hasCur || r.atLimit() {
		return endOfInput
	}
	if len(r.cur) == r.column {
		r.line++
		r.column = 0
		r.hasCur = r.line < len(r.lines)
		if r.hasCur {
			r.cur = r.lines[r.line]
		}
		return '\n'
	}
	c := r.cur[r.column]
	r.column++
	return c
}

// backspace undoes the last next call. Like Java, backing over a line end
// positions the reader at the last character of the previous line.
func (r *charReader) backspace() {
	if r.column != 0 {
		r.column--
		return
	}
	if r.line == 0 {
		reportBug("PcalCharReader.backspace trying to move past beginning of reader")
	}
	r.line--
	r.cur, r.hasCur = r.lines[r.line], true
	r.column = max(len(r.cur)-1, 0)
}

// peek skips spaces and line ends and returns the rest of the current line
// followed by "\n", or "\t" at the end of input.
func (r *charReader) peek() string {
	c := r.next()
	for c == ' ' || c == '\n' {
		c = r.next()
	}
	if c == endOfInput {
		return "\t"
	}
	r.backspace()
	return fromUnits(r.cur[r.column:]) + "\n"
}
