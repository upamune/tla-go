package main

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/upamune/tla-go/checker"
	"github.com/upamune/tla-go/evaluator"
)

// compatOptions holds Java TLC options that change how results and user
// output are presented, and options that are accepted for compatibility.
type compatOptions struct {
	diffTrace, terse bool
	cleanup          bool
	userFile         string
	userOutput       *bufio.Writer
	fpmemDeprecated  bool
}

const compatUsage = "Java TLC options: [-difftrace] [-terse] [-userFile file] [-cleanup] [-fpmem num] [-fpbits num] [-gzip] [-modelcheck] [-debug]"

const fpmemDeprecation = "Using -fpmem with an abolute memory value has been deprecated. Please allocate memory for the TLC process via the JVM mechanisms and use -fpmem to set the fraction to be used for fingerprint storage."

var compatFlags = map[string]func(*compatOptions){
	"-difftrace": func(c *compatOptions) { c.diffTrace = true },
	"-terse":     func(c *compatOptions) { c.terse = true },
	"-cleanup":   func(c *compatOptions) { c.cleanup = true },
	// Java TLC uses these for value-stream compression, an explicit default
	// mode, and internal debugging output, none of which tla-go has.
	"-gzip":       func(*compatOptions) {},
	"-modelcheck": func(*compatOptions) {},
	"-debug":      func(*compatOptions) {},
}

var compatValues = map[string]func(*compatOptions, []string) (int, error){
	"-userFile": parseUserFile,
	"-fpmem":    parseFingerprintMemory,
	"-fpbits":   parseFingerprintBits,
}

// parseCompatOption parses the options above and treats any other argument
// as a model path.
func parseCompatOption(opts *options, args []string) (int, error) {
	if set, ok := compatFlags[args[0]]; ok {
		set(&opts.compat)
		return 0, nil
	}
	if parse, ok := compatValues[args[0]]; ok {
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
	size, err := strconv.ParseFloat(strings.TrimSpace(args[1]), 64)
	if err != nil || size < 0 || math.IsNaN(size) {
		return 0, commandLineError("Error: An positive integer or a fraction for fpset memory size/percentage required. But encountered " + args[1])
	}
	c.fpmemDeprecated = size > 1
	return 1, nil
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
// the working directory, and creating the -userFile file. The returned
// function flushes and closes that file.
func startCompatOutput(opts *options, out io.Writer) (func(), error) {
	if opts.compat.fpmemDeprecated {
		fmt.Fprintln(out, fpmemDeprecation)
	}
	if opts.compat.cleanup {
		_ = os.RemoveAll("states")
	}
	if opts.compat.userFile == "" {
		return func() {}, nil
	}
	file, err := os.Create(opts.compat.userFile)
	if err != nil {
		return nil, commandLineError("Error: Failed to create user output log file.")
	}
	opts.compat.userOutput = bufio.NewWriter(file)
	return func() {
		_ = opts.compat.userOutput.Flush()
		_ = file.Close()
	}, nil
}

// configureCompatOutput applies -terse and -userFile to Print and PrintT.
// Java TLC writes them to the user file without the 80-column layout for
// PrintT.
func configureCompatOutput(checkOpts *checker.Options, opts options) {
	checkOpts.PrintStyle = evaluator.PrintStyle{Terse: opts.compat.terse, UnformattedPrintT: opts.compat.userOutput != nil}
	if output := opts.compat.userOutput; output != nil {
		checkOpts.OnPrint = func(line string) { fmt.Fprintln(output, line) }
	}
}
