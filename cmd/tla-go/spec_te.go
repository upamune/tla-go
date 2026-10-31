package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/upamune/tla-go/checker"
	"github.com/upamune/tla-go/config"
	"github.com/upamune/tla-go/syntax/parser"
)

const specTEParseFailure = "A model checking error occurred while parsing tool output; the execution ended before the potential SpecTE generation stage."

// specTE implements Java TLC's -generateSpecTE. After a model-checking error
// with a trace, it writes SpecTE.tla and SpecTE.cfg beside the
// specification, and finally prefixes SpecTE.tla with the tool output of
// the run, as tlc2.TLC does.
type specTE struct {
	monolith  bool
	output    bytes.Buffer
	first     *checker.Result
	tlaPath   string
	tlaText   string
	modules   specTEModules
	errorSeen bool
}

// specTERun is the checked model whose result may produce SpecTE files.
type specTERun struct {
	module               *parser.Module
	specPath, configPath string
	cfg                  config.Config
	loader               checker.ModuleLoader
}

func parseSpecTEOption(opts *options, args []string) (int, error) {
	opts.tool = true
	opts.compat.specTE = &specTE{monolith: true}
	if len(args) > 1 && args[1] == "nomonolith" {
		opts.compat.specTE.monolith = false
		return 1, nil
	}
	return 0, nil
}

// start captures all later tool output for SpecTE.tla.
func (s *specTE) start(out io.Writer) io.Writer {
	out = io.MultiWriter(out, &s.output)
	toolOutput{out: out}.message(1000, 0, "Will generate a SpecTE file pair if error states are encountered.")
	return out
}

// record keeps the first continued violation, whose trace Java TLC uses.
func (s *specTE) record(result checker.Result) {
	if s.first == nil {
		s.first = &result
	}
}

// afterCheck writes the files before the run's final message. Java TLC
// writes SpecTE files only when model checking reports a trace.
func (s *specTE) afterCheck(t toolOutput, result checker.Result, run specTERun, simulate bool) {
	if s.first != nil {
		result, s.first = *s.first, nil
	}
	if result.Violation == nil && result.Failure == nil {
		return
	}
	s.errorSeen = true
	trace, loop := specTEErrorTrace(result)
	if simulate || len(trace) == 0 {
		return
	}
	t.message(1000, 0, "The model check run produced error-states - we will generate the SpecTE files now.")
	if err := s.write(t, specTETrace(trace, loop), run); err != nil {
		t.message(1000, 1, err.Error())
	}
}

// specTEErrorTrace returns the trace Java TLC parses after the first error
// message; a state printed within an initial-state violation is not one.
func specTEErrorTrace(result checker.Result) ([]checker.TraceStep, *int) {
	if result.Violation == nil {
		return resultTrace(result), nil
	}
	if initialViolation(result) {
		return nil, nil
	}
	return result.Violation.Trace, result.Violation.LoopStart
}

func (s *specTE) write(t toolOutput, states []specTEState, run specTERun) error {
	source, err := os.ReadFile(run.specPath)
	if err != nil {
		return err
	}
	cfgSource, err := os.ReadFile(run.configPath)
	if err != nil {
		return err
	}
	dir, err := filepath.Abs(filepath.Dir(run.specPath))
	if err != nil {
		return err
	}
	now := time.Now()
	traceID := fmt.Sprintf("def_ov_%d2000", now.UnixMilli())
	tla, cfg := specTEBuffers(states, run.cfg, specTEBaseName(run.cfg), traceID)
	s.tlaPath = filepath.Join(dir, "SpecTE.tla")
	s.tlaText = copySpecTEModule(string(source), run.module.Name, run.module.Extends, tla)
	s.modules = discoverSpecTEModules(run.module, run.loader)
	if err := writeSpecTEFile(t, s.tlaPath, s.tlaText); err != nil {
		return err
	}
	return writeSpecTEFile(t, filepath.Join(dir, "SpecTE.cfg"), copySpecTEConfig(string(cfgSource), cfg, now))
}

func writeSpecTEFile(t toolOutput, path, text string) error {
	if err := os.WriteFile(path, []byte(text), 0o666); err != nil {
		return err
	}
	t.message(1000, 0, "The file "+path+" has been created.")
	return nil
}

// specTEBaseName names the sub-actions after the next-state relation or
// the specification.
func specTEBaseName(cfg config.Config) string {
	if cfg.Specification != "" {
		return cfg.Specification
	}
	return cfg.Next
}

// finish runs after the final tool message: it prefixes SpecTE.tla with the
// captured output and embeds dependent modules. When the first error had no
// trace, Java TLC reports that it could not parse its output instead (and
// then stops responding, which tla-go does not reproduce).
func (s *specTE) finish(out io.Writer) {
	if s.tlaPath == "" {
		if s.errorSeen {
			toolOutput{out: out}.message(1000, 0, specTEParseFailure)
		}
		return
	}
	text := s.modules.monolith(s.output.String()+s.tlaText, s.monolith)
	if err := os.WriteFile(s.tlaPath, []byte(text), 0o666); err != nil {
		fmt.Fprintln(out, err)
	}
}
