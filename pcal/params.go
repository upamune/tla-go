package pcal

import (
	_ "embed"
	"strconv"
	"strings"
	"unicode"
)

// Version and ModDate identify the Java translator this package follows.
const (
	Version = "1.11"
	ModDate = "31 December 2020"
	// versionWeight is Java's numeric form of Version.
	versionWeight = 1902
	version15     = 1500
)

//go:embed help.txt
var helpText string

// Params holds the translator options, as PcalParams does.
type Params struct {
	Debug            bool
	WriteAST         bool
	LabelFlag        bool
	ReportLabels     bool
	LabelRoot        string
	FairnessOption   string
	FairAlgorithm    bool
	CheckTermination bool
	NoCfg            bool
	NoDoneDisjunct   bool
	UnixEOL          bool
	WrapColumn       int
	SSWrapColumn     int
	// InputFile is the input path without its .tla extension.
	InputFile string
	// Dir is the directory against which relative file names are resolved;
	// empty means the current directory.
	Dir string

	specOption, myspecOption, spec2Option, myspec2Option bool
	specFile                                             string
	optionsInFile                                        bool
	inputVersion                                         int
}

// NewParams returns the default options.
func NewParams() *Params {
	return &Params{LabelRoot: "Lbl_", WrapColumn: 78, SSWrapColumn: 45, inputVersion: versionWeight}
}

// TLCTranslation reports whether one of the -spec options was given. These
// options make Java run TLC on PlusCal's own TLA+ specification.
func (p *Params) TLCTranslation() bool {
	return p.specOption || p.myspecOption || p.spec2Option || p.myspec2Option
}

// versionToNumber ports PcalParams.VersionToNumber. Like Java, it computes
// with 32-bit integers that wrap around.
func versionToNumber(s string) int {
	var result int32
	digitsAfter, afterDecimal := 0, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case '0' <= c && c <= '9':
			result = 10*result + int32(c-'0')
			if afterDecimal {
				digitsAfter++
			}
		case c == '.':
			afterDecimal = true
		default:
			return -1
		}
		if digitsAfter > 3 {
			return -1
		}
	}
	for range 3 - digitsAfter {
		result *= 10
	}
	return int(result)
}

// status values of trans.parseAndProcessArguments.
type argStatus int

const (
	statusOK argStatus = iota
	statusExitOK
	statusError
)

// argParser processes command-line or options-statement arguments.
type argParser struct {
	p             *Params
	log           *logger
	args          []string
	next          int
	inFile        bool
	firstFairness bool
	explicitNof   bool
}

// parseArgs ports trans.parseAndProcessArguments. The last argument is the
// input file, or a dummy when the arguments come from an options statement.
func (p *Params) parseArgs(args []string, log *logger) argStatus {
	a := &argParser{p: p, log: log, args: args, inFile: p.optionsInFile, firstFairness: p.optionsInFile}
	return a.run()
}

func (a *argParser) maxArg() int { return len(a.args) - 1 }

func (a *argParser) commandLineError(msg string) argStatus {
	a.log.error("Command-line error: " + msg + ".")
	return statusError
}

func (a *argParser) run() argStatus {
	if a.maxArg() < 0 {
		return a.commandLineError("No arguments specified")
	}
	if !a.inFile && strings.HasPrefix(a.args[a.maxArg()], "-") {
		return a.help()
	}
	if st := a.options(); st != statusOK {
		return st
	}
	if a.inFile {
		return statusOK
	}
	return a.inputFile(a.args[a.maxArg()])
}

// options processes the arguments before the input file.
func (a *argParser) options() argStatus {
	for ; a.next < a.maxArg(); a.next++ {
		if st := a.option(a.args[a.next]); st != statusOK {
			return st
		}
	}
	if a.next > a.maxArg() {
		return a.commandLineError("No input file specified")
	}
	// -termination implies -wf unless a fairness option was given.
	if a.p.CheckTermination && a.p.FairnessOption == "" && !a.explicitNof {
		a.p.FairnessOption = "wf"
	}
	return statusOK
}

func (a *argParser) help() argStatus {
	a.log.info(strings.Join(javaLines(helpText), "\n"))
	return statusExitOK
}

// optionHandler describes an option. cmdOK options are accepted with a
// leading "-" everywhere, and fileOK options also without it in an options
// statement; the others are accepted only on the command line.
type optionHandler struct {
	name   string
	fileOK bool
	cmdOK  bool
	handle func(*argParser) argStatus
}

func flagOption(name string, fileOK, cmdOK bool, set func(*Params)) optionHandler {
	return optionHandler{name, fileOK, cmdOK, func(a *argParser) argStatus {
		set(a.p)
		return statusOK
	}}
}

func specHandler(name string, fileOK bool, message string, set func(*Params)) optionHandler {
	return optionHandler{name, fileOK, fileOK, func(a *argParser) argStatus { return a.specOption(set, message) }}
}

func fairnessHandler(name string) optionHandler {
	return optionHandler{name, true, true, func(a *argParser) argStatus { return a.fairness(name) }}
}

var optionHandlers []optionHandler

func init() {
	optionHandlers = []optionHandler{
		{"help", false, false, (*argParser).help},
		specHandler("writeAST", false, "", func(p *Params) { p.WriteAST = true }),
		specHandler("spec", true, "spec", func(p *Params) { p.specOption = true }),
		specHandler("myspec", true, "myspec", func(p *Params) { p.myspecOption = true }),
		specHandler("spec2", false, "spec", func(p *Params) { p.spec2Option = true }),
		specHandler("myspec2", false, "myspec", func(p *Params) { p.myspec2Option = true }),
		flagOption("debug", false, false, func(p *Params) { p.Debug = true }),
		flagOption("unixEOL", false, false, func(p *Params) { p.UnixEOL = true }),
		flagOption("termination", true, true, func(p *Params) { p.CheckTermination = true }),
		flagOption("nocfg", false, true, func(p *Params) { p.NoCfg = true }),
		flagOption("noDoneDisjunct", true, true, func(p *Params) { p.NoDoneDisjunct = true }),
		fairnessHandler("wf"), fairnessHandler("sf"), fairnessHandler("wfNext"), fairnessHandler("nof"),
		flagOption("label", true, true, func(p *Params) { p.LabelFlag = true }),
		flagOption("reportLabels", false, false, func(p *Params) { p.ReportLabels, p.LabelFlag = true, true }),
		{"labelRoot", true, true, (*argParser).labelRoot},
		{"version", true, true, (*argParser).version},
		{"lineWidth", false, true, (*argParser).lineWidth},
	}
}

// option handles one option. Options marked cmdOK are recognized on the
// command line; the others only when not in an options statement.
func (a *argParser) option(option string) argStatus {
	for _, h := range optionHandlers {
		if a.handles(h, option) {
			return h.handle(a)
		}
	}
	if !a.inFile {
		return a.commandLineError("Unknown command-line option: " + option)
	}
	return a.commandLineError("Unknown or illegal option in options statement: " + option)
}

func (a *argParser) handles(h optionHandler, option string) bool {
	if option == "-"+h.name {
		return h.cmdOK || !a.inFile
	}
	return h.fileOK && a.inFile && option == h.name
}

// argument consumes the argument of an option.
func (a *argParser) argument() (string, bool) {
	a.next++
	if a.next == a.maxArg() {
		return "", false
	}
	return a.args[a.next], true
}

func (a *argParser) specOption(set func(*Params), name string) argStatus {
	set(a.p)
	if a.conflictingSpecOptions() {
		return statusError
	}
	if name == "" {
		return statusOK
	}
	file, ok := a.argument()
	if !ok {
		return a.commandLineError("Specification name must follow `-" + name + "' option")
	}
	a.p.specFile = file
	return statusOK
}

func (a *argParser) conflictingSpecOptions() bool {
	n := 0
	for _, b := range []bool{a.p.specOption, a.p.myspecOption, a.p.spec2Option, a.p.myspec2Option, a.p.WriteAST} {
		if b {
			n++
		}
	}
	if n > 1 {
		a.commandLineError("\nCan have at most one of the options -spec, -myspec, -spec2, -myspec2, writeAST")
		return true
	}
	return false
}

func (a *argParser) fairness(value string) argStatus {
	if a.firstFairness {
		a.p.FairnessOption = ""
		a.firstFairness = false
	}
	if a.p.FairnessOption != "" {
		return a.commandLineError("Can only have one of -wf, -sf, -wfNext, and -nof options")
	}
	a.p.FairnessOption = value
	a.explicitNof = a.explicitNof || value == "nof"
	return statusOK
}

func (a *argParser) labelRoot() argStatus {
	root, ok := a.argument()
	if !ok {
		return a.commandLineError("Label root must follow `-labelRoot' option")
	}
	a.p.LabelRoot = root
	return statusOK
}

func (a *argParser) version() argStatus {
	ver, ok := a.argument()
	if !ok {
		return a.commandLineError("Version number must follow `-version' option")
	}
	vnum := versionToNumber(ver)
	switch {
	case vnum < 0:
		a.log.error("Illegal version " + ver + " specified.")
	case vnum > versionWeight:
		a.log.error("Specified version " + ver + " later than current version " + Version)
	default:
		a.p.inputVersion = vnum
		return statusOK
	}
	return a.commandLineError("Bad version number")
}

func (a *argParser) lineWidth() argStatus {
	arg, ok := a.argument()
	width, isInt := parseJavaInt(arg)
	if !ok || !isInt || width < 60 {
		return a.commandLineError("Integer value at least 60 must follow `-lineWidth' option")
	}
	a.p.WrapColumn = width
	a.p.SSWrapColumn = width - 33
	return statusOK
}

// parseJavaInt mirrors Integer.parseInt, which also accepts the decimal
// digits of other scripts.
func parseJavaInt(s string) (int, bool) {
	n, err := strconv.ParseInt(strings.Map(asciiDigit, s), 10, 32)
	return int(n), err == nil
}

// asciiDigit maps a decimal digit of the Basic Multilingual Plane, which
// Character.digit accepts, to the ASCII digit of the same value. Unicode
// encodes decimal digits in runs of ten that start with zero, and each
// range of unicode.Nd consists of whole runs.
func asciiDigit(r rune) rune {
	for _, rg := range unicode.Nd.R16 {
		if lo := rune(rg.Lo); lo <= r && r <= rune(rg.Hi) {
			return '0' + (r-lo)%10
		}
	}
	return r
}

// inputFile sets InputFile from the last argument and checks that the file
// exists.
func (a *argParser) inputFile(name string) argStatus {
	name = javaFilePath(name)
	base := javaFileName(name)
	if !strings.Contains(base, ".") {
		a.p.InputFile = name
		if !fileExists(resolve(a.p.Dir, name+".tla")) {
			return a.commandLineError("Input file " + name + ".pcal and " + name + ".tla.tla not found")
		}
		return statusOK
	}
	if !strings.HasSuffix(strings.ToLower(base), ".tla") {
		return a.commandLineError("Input file has extension other than tla")
	}
	a.p.InputFile = name[:strings.LastIndex(name, ".")]
	if !fileExists(resolve(a.p.Dir, name)) {
		return a.commandLineError("Input file " + name + " does not exist.")
	}
	return statusOK
}
