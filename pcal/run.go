package pcal

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
)

// Exit codes of the Java translator: System.exit(-1) is reported as 255,
// and an uncaught Java error as 1.
const (
	ExitOK    = 0
	ExitError = 255
	ExitBug   = 1
)

func (p *Params) eol() string {
	if p.UnixEOL || runtime.GOOS != "windows" {
		return "\n"
	}
	return "\r\n"
}

// Run runs the translator with pcal.trans command-line arguments. It
// rewrites File.tla, keeps the previous version as File.old, and writes
// File.cfg unless -nocfg is given. Messages go to stdout as in Java.
func Run(args []string, stdout, stderr io.Writer) int {
	return RunDir("", args, stdout, stderr)
}

// RunDir is like Run but resolves relative file names against dir.
func RunDir(dir string, args []string, stdout, stderr io.Writer) (code int) {
	log := &logger{w: stdout}
	defer func() {
		if r := recover(); r != nil {
			code = reportCrash(r, log, stderr)
		}
	}()
	log.info("pcal.trans Version " + Version + " of " + ModDate)
	p := NewParams()
	p.Dir = dir
	switch p.parseArgs(args, log) {
	case statusExitOK:
		return ExitOK
	case statusError:
		return ExitError
	}
	if p.TLCTranslation() {
		log.error(errTLCTranslation)
		return ExitError
	}
	return runFile(p, log)
}

// reportCrash reports a failure that Java does not catch, such as a
// PcalDebug.ReportBug error, and returns the exit code of the Java runtime.
// Other failures are internal errors whose stack is printed to stderr.
func reportCrash(r any, log *logger, stderr io.Writer) int {
	bug, ok := r.(*bugError)
	if !ok {
		fmt.Fprintf(stderr, "internal error in the PlusCal translator: %v\n%s", r, debug.Stack())
		return ExitBug
	}
	log.info("")
	log.info(bug.msg)
	log.info("")
	fmt.Fprintln(stderr, "Exception in thread \"main\" java.lang.Error: "+bug.msg)
	return ExitBug
}

func runFile(p *Params, log *logger) int {
	tlaPath := p.InputFile + ".tla"
	input, err := readJavaLines(resolve(p.Dir, tlaPath))
	if err != nil {
		log.error(readError(tlaPath, err))
		return ExitError
	}
	t := &translation{p: p, log: log}
	output, ok := t.perform(input)
	if !ok {
		return ExitError
	}
	old := resolve(p.Dir, p.InputFile+".old")
	_ = os.Remove(old)
	_ = os.Rename(resolve(p.Dir, tlaPath), old)
	if err := writeJavaLines(resolve(p.Dir, tlaPath), output, p.eol()); err != nil {
		log.error("Could not write file " + tlaPath)
		return ExitError
	}
	log.info("New file " + tlaPath + " written.")
	if p.NoCfg {
		return ExitOK
	}
	return writeCfg(p, log, t.hasDefaultInit)
}

func readError(path string, err error) string {
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission) {
		return "Input file " + path + " not found."
	}
	return "Error reading file " + path + "."
}

// writeCfg writes the configuration file, keeping the user's lines after
// the delimiter comment.
func writeCfg(p *Params, log *logger, hasDefaultInit bool) int {
	path := p.InputFile + ".cfg"
	cfg := []string{cfgDelimiter}
	if fileExists(resolve(p.Dir, path)) {
		lines, err := readJavaLines(resolve(p.Dir, path))
		if os.IsPermission(err) {
			log.info("File " + p.InputFile + ".cfg is read only, new version not written.")
			return ExitOK
		}
		if err != nil {
			log.error(readError(path, err))
			return ExitError
		}
		cfg = lines
	}
	cfg = cfgLines(cfg, p, hasDefaultInit)
	if hasSpecification(cfg) {
		log.info("File " + p.InputFile + ".cfg already contains SPECIFICATION statement,\n   so new one not written.")
	} else {
		cfg = append([]string{"SPECIFICATION Spec"}, cfg...)
	}
	if err := writeJavaLines(resolve(p.Dir, path), cfg, p.eol()); err != nil {
		log.error("Could not write file " + path)
		return ExitError
	}
	log.info("New file " + path + " written.")
	return ExitOK
}

// cfgLines drops the lines before the delimiter, which a previous run
// wrote, and adds the statements the translation needs.
func cfgLines(cfg []string, p *Params, hasDefaultInit bool) []string {
	j := 0
	for j < len(cfg) && !strings.Contains(cfg[j], cfgDelimiter) {
		j++
	}
	if j < len(cfg) {
		cfg = cfg[j:]
	} else {
		cfg = append([]string{cfgDelimiter}, cfg...)
	}
	if p.TLCTranslation() || hasDefaultInit {
		cfg = append([]string{"CONSTANT defaultInitValue = defaultInitValue"}, cfg...)
	}
	if p.CheckTermination {
		cfg = append([]string{"PROPERTY Termination"}, cfg...)
	}
	return cfg
}

func hasSpecification(cfg []string) bool {
	for _, line := range cfg {
		spec := strings.Index(line, "SPECIFICATION")
		comment := strings.Index(line, `\*`)
		if spec != -1 && (comment == -1 || comment > spec) {
			return true
		}
	}
	return false
}

// javaFilePath normalizes a path like java.io.File: it removes duplicate
// and trailing separators.
func javaFilePath(name string) string {
	sep := string(filepath.Separator)
	for strings.Contains(name, sep+sep) {
		name = strings.ReplaceAll(name, sep+sep, sep)
	}
	if len(name) > 1 {
		name = strings.TrimSuffix(name, sep)
	}
	return name
}
