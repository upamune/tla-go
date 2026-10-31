// Package pcal translates PlusCal algorithms to TLA+, following the Java
// translator pcal.trans in the TLA+ tools.
package pcal

import (
	"fmt"
	"hash/crc32"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const (
	beginTranslation = `\* BEGIN TRANSLATION`
	endTranslation   = `\* END TRANSLATION`
	checksumTemplate = `(chksum(pcal) = "%s" /\ chksum(tla) = "%s")`
	cfgDelimiter     = `\* Add statements after this line.`
)

// checksumPattern is Validator.CHECKSUM_PATTERN, with Java's \s spelled out.
var checksumPattern = regexp.MustCompile(`\\\* BEGIN TRANSLATION[ \t\n\x0B\f\r]+\([ \t\n\x0B\f\r]*((?i)ch(ec)?ksum\(p(lus)?cal\)(?-i))[ \t\n\x0B\f\r]*(=[ \t\n\x0B\f\r]*"(?P<pcalchecksum>[0-9A-Fa-f]*)"|\\in[ \t\n\x0B\f\r]*STRING)[ \t\n\x0B\f\r]*/\\[ \t\n\x0B\f\r]*((?i)ch(ec)?ksum\(tla\+?\)(?-i))[ \t\n\x0B\f\r]*(=[ \t\n\x0B\f\r]*"(?P<tlachecksum>[0-9A-Fa-f]*)"|\\in[ \t\n\x0B\f\r]*STRING)[ \t\n\x0B\f\r]*\)`)

// checksum is Validator.checksum: the CRC-32 of s in lowercase hex.
func checksum(s string) string { return strconv.FormatUint(uint64(crc32.ChecksumIEEE([]byte(s))), 16) }

// translation holds the state of one run of the translator.
type translation struct {
	p   *Params
	log *logger
	// hasDefaultInit records whether a variable has no initial value.
	hasDefaultInit bool
}

// Translate translates the algorithm in the lines of a .tla file and returns
// the lines of the new file, or false after reporting an error to out.
func Translate(lines []string, p *Params, out io.Writer) ([]string, bool) {
	t := &translation{p: p, log: &logger{w: out}}
	return t.perform(lines)
}

func (t *translation) perform(spec []string) ([]string, bool) {
	untab := removeTabs(spec)
	t.processOptionsStatement(untab)
	output := slices.Clone(spec)
	begin := findTokenPair(untab, 0, "BEGIN", "TRANSLATION")
	if begin != -1 {
		end := findTokenPair(untab, begin+1, "END", "TRANSLATION")
		if end == -1 {
			t.log.error("No line containing `END TRANSLATION")
			return nil, false
		}
		output = slices.Delete(output, begin+1, end)
		untab = slices.Delete(untab, begin+1, end)
	}
	algLine, algCol, fair, ok := findAlgorithm(untab)
	if !ok {
		t.log.error("Beginning of algorithm string --algorithm not found.")
		return nil, false
	}
	if begin == -1 {
		if begin, ok = t.insertTranslationMarkers(&output, &untab, algLine, algCol); !ok {
			return nil, false
		}
	}
	translation, ast, ok := t.translate(untab, len(output), algLine, algCol, fair)
	if !ok {
		return nil, false
	}
	output[begin] = updateChecksums(output[begin], translation, func() string {
		return astString(ast, t.p.Debug, t.p.inputVersion < version15)
	})
	output = slices.Insert(output, begin+1, translation...)
	t.log.info("Translation completed.")
	return output, true
}

// findAlgorithm returns the position after "--algorithm" or "--fair".
func findAlgorithm(lines []string) (line, col int, fair, ok bool) {
	for i, s := range lines {
		if c := jindex(s, "--algorithm"); c != -1 {
			return i, c + len("--algorithm"), false, true
		}
		if c := jindex(s, "--fair"); c != -1 {
			return i, c + len("--fair"), true, true
		}
	}
	return 0, 0, false, false
}

// insertTranslationMarkers adds BEGIN and END TRANSLATION lines after the
// comment containing the algorithm and returns the BEGIN line.
func (t *translation) insertTranslationMarkers(output, untab *[]string, algLine, algCol int) (int, bool) {
	ecLine, ecCol, found := endOfComment(*untab, algLine, algCol)
	if !found {
		t.log.error("Algorithm not in properly terminated comment")
		return 0, false
	}
	endStuff := jtrim(jsubstr((*untab)[ecLine], ecCol))
	if endStuff != "" && !strings.HasPrefix(endStuff, `\*`) {
		t.log.error("Text on same line following `*)' that ends the \n   comment containing the algorithm.")
		return 0, false
	}
	*output = slices.Insert(*output, ecLine+1, beginTranslation+" "+fmt.Sprintf(checksumTemplate, "ffffffff", "ffffffff"), endTranslation+" ")
	*untab = slices.Insert(*untab, ecLine+1, beginTranslation, endTranslation)
	return ecLine + 1, true
}

// endOfComment returns the position after the "*)" that closes the comment
// containing the algorithm.
func endOfComment(lines []string, line, col int) (int, int, bool) {
	depth := 1
	for ; line < len(lines); line, col = line+1, 0 {
		if end, ok := scanComment(toUnits(lines[line]), col, &depth); ok {
			return line, end, true
		}
	}
	return 0, 0, false
}

// scanComment scans a line for comment delimiters and returns the position
// after the one that closes the outermost comment.
func scanComment(u []uint16, col int, depth *int) (int, bool) {
	for col < len(u)-1 {
		switch {
		case u[col] == '(' && u[col+1] == '*':
			*depth++
			col += 2
		case u[col] == '*' && u[col+1] == ')':
			*depth--
			col += 2
			if *depth == 0 {
				return col, true
			}
		default:
			col++
		}
	}
	return col, false
}

// translate parses and translates the algorithm. It returns the translation
// and the parsed AST.
func (t *translation) translate(untab []string, size, algLine, algCol int, fair bool) ([]string, node, bool) {
	if err := catch(func() { uncomment(untab, algLine, algCol) }, nil); err != nil {
		t.log.report(err)
		return nil, nil, false
	}
	lines := make([][]uint16, len(untab))
	for i, s := range untab {
		lines[i] = toUnits(s)
	}
	ps := newParser(t.p, t.log, newCharReader(lines, algLine, algCol, size, 0), algLine, algCol)
	ast, err := ps.parse(fair)
	if err != nil {
		t.log.report(err)
		return nil, nil, false
	}
	t.log.info("Parsing completed.")
	t.hasDefaultInit = ps.hasDefaultInit
	if t.p.WriteAST {
		t.writeAST(ast)
		return nil, nil, false
	}
	if t.p.TLCTranslation() {
		t.log.error(errTLCTranslation)
		return nil, nil, false
	}
	code, err := t.generate(ast, ps)
	if err != nil {
		t.log.report(err)
		return nil, nil, false
	}
	return code, ast, true
}

// errTLCTranslation explains why the -spec options are rejected.
const errTLCTranslation = "The -spec, -myspec, -spec2, and -myspec2 options, which run TLC on\n" +
	"    PlusCal's TLA+ specification, are not supported by tla-go"

// generate renames conflicting identifiers and generates the TLA+
// translation, as PCalTLAGenerator does.
func (t *translation) generate(ast node, ps *parser) ([]string, *Error) {
	st, err := t.removeNameConflicts(ast)
	if err != nil {
		return nil, err
	}
	xast, err := explode(ast, st, ps.omitPC)
	if err != nil {
		return nil, err
	}
	g := &generator{params: t.p, omitPC: ps.omitPC, omitStutteringWhenDone: ps.omitStutteringWhenDone, hasDefaultInit: ps.hasDefaultInit, st: st}
	code, err := g.generate(xast, st.disambiguateReport)
	if err != nil {
		return nil, err
	}
	if ps.hasDefaultInit {
		if err := st.checkForDefaultInitValue(); err != nil {
			return nil, err
		}
	}
	return code, nil
}

// removeNameConflicts builds the symbol table and renames identifiers that
// would clash in the translation.
func (t *translation) removeNameConflicts(ast node) (*symTab, *Error) {
	st, err := newSymTab(ast)
	if err != nil {
		return nil, err
	}
	st.disambiguate()
	if len(st.disambiguateReport) > 0 {
		t.log.warning("symbols were renamed.")
	}
	if err := catch(func() { fixIDs(ast, st, t.log) }, nil); err != nil {
		return nil, &Error{Message: err.Message}
	}
	return st, nil
}

// updateChecksums fills in the checksums of a BEGIN TRANSLATION line that
// requests them.
func updateChecksums(line string, translation []string, astText func() string) string {
	m := checksumPattern.FindStringSubmatchIndex(line)
	if m == nil {
		return line
	}
	pcal := 2 * checksumPattern.SubexpIndex("pcalchecksum")
	tla := 2 * checksumPattern.SubexpIndex("tlachecksum")
	result := line
	if m[tla] >= 0 {
		result = result[:m[tla]] + checksum(strings.Join(translation, "")) + result[m[tla+1]:]
	}
	if m[pcal] >= 0 {
		result = result[:m[pcal]] + checksum(astText()) + result[m[pcal+1]:]
	}
	return result
}

func (t *translation) writeAST(ast node) {
	lines := []string{
		"------ MODULE AST -------",
		"EXTENDS TLC",
		"fairness == \"" + t.p.FairnessOption + "\"",
		" ",
		"ast == ",
		astString(ast, t.p.Debug, t.p.inputVersion < version15),
		"==========================",
	}
	if err := writeJavaLines("AST.tla", lines, t.p.eol()); err != nil {
		t.log.error("Could not write file AST.tla")
		return
	}
	t.log.info("Wrote file AST.tla.")
}
