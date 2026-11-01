package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/upamune/tla-go/checker"
	"github.com/upamune/tla-go/evaluator"
)

// compatOptions holds Java TLC options that change how results and user
// output are presented, and options that are accepted for compatibility.
type compatOptions struct {
	diffTrace, terse bool
	debug            bool
	cleanup          bool
	userFile         string
	userFileTool     bool // -tool preceded -userFile, so its error is message 1102.
	userOutput       *bufio.Writer
	fpmemDeprecated  bool
	specTE           *specTE
}

const compatUsage = "Java TLC options: [-difftrace] [-terse] [-userFile file] [-cleanup] [-fpmem num] [-fpbits num] [-gzip] [-modelcheck] [-debug] [-generateSpecTE [nomonolith]]"

const fpmemDeprecation = "Using -fpmem with an abolute memory value has been deprecated. Please allocate memory for the TLC process via the JVM mechanisms and use -fpmem to set the fraction to be used for fingerprint storage."

var compatFlags = map[string]func(*compatOptions){
	"-difftrace": func(c *compatOptions) { c.diffTrace = true },
	"-terse":     func(c *compatOptions) { c.terse = true },
	"-cleanup":   func(c *compatOptions) { c.cleanup = true },
	// Java TLC uses these for value-stream compression, an explicit default
	// mode, and internal debugging output, none of which tla-go has.
	"-gzip":       func(*compatOptions) {},
	"-modelcheck": func(*compatOptions) {},
	"-debug":      func(c *compatOptions) { c.debug = true },
}

var compatValues = map[string]func(*compatOptions, []string) (int, error){
	"-userFile": parseUserFile,
	"-fpmem":    parseFingerprintMemory,
	"-fpbits":   parseFingerprintBits,
}

// parseCompatOption parses the options above and treats any other argument
// as a model path.
func parseCompatOption(opts *options, args []string) (int, error) {
	if args[0] == "-generateSpecTE" {
		return parseSpecTEOption(opts, args)
	}
	if set, ok := compatFlags[args[0]]; ok {
		set(&opts.compat)
		return 0, nil
	}
	if parse, ok := compatValues[args[0]]; ok {
		opts.compat.userFileTool = opts.compat.userFileTool || args[0] == "-userFile" && opts.tool
		return parse(&opts.compat, args)
	}
	return 0, parsePath(opts, args[0])
}

func parseUserFile(c *compatOptions, args []string) (int, error) {
	if len(args) < 2 {
		return 0, commandLineError("Error: need to specify the full qualified file.")
	}
	c.userFile = args[1]
	return 1, nil
}

// parseFingerprintMemory validates -fpmem as Java TLC does. The value only
// sizes Java's fingerprint set; a value above 1 is a deprecated absolute
// size, which Java reports before starting.
func parseFingerprintMemory(c *compatOptions, args []string) (int, error) {
	if len(args) < 2 {
		return 0, commandLineError("Error: fpset memory size required.")
	}
	size, ok := parseJavaDouble(args[1])
	if !ok || size < 0 || math.IsNaN(size) {
		return 0, commandLineError("Error: An positive integer or a fraction for fpset memory size/percentage required. But encountered " + args[1])
	}
	c.fpmemDeprecated = size > 1
	return 1, nil
}

// javaDouble matches the trimmed strings Java's Double.parseDouble accepts,
// following the grammar documented for Double.valueOf.
var javaDouble = regexp.MustCompile(`^[+-]?(NaN|Infinity|(\d+\.?\d*([eE][+-]?\d+)?|\.\d+([eE][+-]?\d+)?|0[xX]([[:xdigit:]]+\.?|[[:xdigit:]]*\.[[:xdigit:]]+)[pP][+-]?\d+)[fFdD]?)$`)

// parseJavaDouble parses a number as Double.parseDouble does: surrounding
// control characters and spaces are ignored, a float-type suffix is
// allowed, and values beyond the double range become infinite.
func parseJavaDouble(text string) (float64, bool) {
	text = strings.TrimFunc(text, func(r rune) bool { return r <= ' ' })
	if !javaDouble.MatchString(text) {
		return 0, false
	}
	if !strings.HasSuffix(text, "NaN") && !strings.HasSuffix(text, "Infinity") {
		text = strings.TrimRight(text, "fFdD")
	}
	number, err := strconv.ParseFloat(text, 64)
	return number, err == nil || errors.Is(err, strconv.ErrRange)
}

// parseFingerprintBits validates -fpbits, which only selects the layout of
// Java TLC's fingerprint set.
func parseFingerprintBits(_ *compatOptions, args []string) (int, error) {
	if len(args) < 2 {
		return 0, commandLineError("Error: fpbits required.")
	}
	bits, err := strconv.Atoi(args[1])
	if err != nil {
		return 0, commandLineError("Error: An integer for fpbits required. But encountered " + args[1])
	}
	if bits < 0 || bits > 30 {
		return 0, commandLineError("Error: Value in interval [0, 30] for fpbits required. But encountered " + args[1])
	}
	return 1, nil
}

// startCompatOutput performs the effects Java TLC has while reading its
// parameters: the -fpmem notice, -cleanup removing the states directory in
// the working directory, creating the -userFile file, and capturing output
// for -generateSpecTE. It returns the writer for the run and a function that
// completes these effects after the run.
func startCompatOutput(opts *options, out io.Writer) (io.Writer, func(), error) {
	if opts.compat.fpmemDeprecated {
		fmt.Fprintln(out, fpmemDeprecation)
	}
	if opts.compat.cleanup && opts.recoverDir == "" {
		_ = os.RemoveAll("states")
	}
	closeUserFile, err := openUserFile(&opts.compat)
	if err != nil {
		return nil, nil, err
	}
	specTE, runOut := opts.compat.specTE, out
	if specTE != nil {
		runOut = specTE.start(out)
	}
	return runOut, func() {
		closeUserFile()
		if specTE != nil {
			specTE.finish(out)
		}
	}, nil
}

func openUserFile(c *compatOptions) (func(), error) {
	if c.userFile == "" {
		return func() {}, nil
	}
	file, err := os.Create(c.userFile)
	if err != nil {
		return nil, commandLineError("Error: Failed to create user output log file.")
	}
	c.userOutput = bufio.NewWriter(file)
	return func() {
		_ = c.userOutput.Flush()
		_ = file.Close()
	}, nil
}

// afterCheck generates SpecTE files for -generateSpecTE before the run's
// final tool message. unparsed reports a mode whose trace messages Java
// TLC's SpecTE parser cannot read.
func (c compatOptions) afterCheck(t toolOutput, result checker.Result, run specTERun, unparsed bool) {
	if c.specTE != nil {
		c.specTE.afterCheck(t, result, run, unparsed)
	}
}

// configureCompatOutput applies -terse and -userFile to Print and PrintT.
// Java TLC writes them to the user file without the 80-column layout for
// PrintT.
func configureCompatOutput(checkOpts *checker.Options, opts options) {
	checkOpts.PrintStyle = evaluator.PrintStyle{Terse: opts.compat.terse, UnformattedPrintT: opts.compat.userOutput != nil}
	if output := opts.compat.userOutput; output != nil {
		checkOpts.OnPrint = func(line string) { fmt.Fprintln(output, line) }
	}
	if specTE, report := opts.compat.specTE, checkOpts.OnViolation; specTE != nil && report != nil {
		checkOpts.OnViolation = func(result checker.Result) {
			specTE.record(result)
			report(result)
		}
	}
}
