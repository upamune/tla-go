package pcal

import (
	"errors"
	"runtime"
	"strconv"
)

// pos is the source position of an AST node, as AST.location reports it.
type pos struct {
	line, col           int
	macroLine, macroCol int
}

func (p pos) location() string {
	loc := "line " + strconv.Itoa(p.line) + ", column " + strconv.Itoa(p.col)
	if p.macroLine <= 0 {
		return loc
	}
	return loc + " of macro called at line " + strconv.Itoa(p.macroLine) + ", column " + strconv.Itoa(p.macroCol)
}

// Error is an unrecoverable translation error. Message is the text that Java
// reports after "Unrecoverable error: -- ".
type Error struct {
	Message string
	at      *pos
}

func (e *Error) Error() string { return e.report() }

// report returns the message as PcalDebug.reportError prints it, without the
// surrounding banner.
func (e *Error) report() string {
	if e.at == nil || e.at.line == 0 {
		return e.Message
	}
	return e.Message + "\n    at " + e.at.location()
}

// bugError corresponds to PcalDebug.ReportBug, which Java does not recover.
type bugError struct{ msg string }

func (e *bugError) Error() string { return e.msg }

// fail aborts the current phase with an unpositioned error.
func fail(msg string) { panic(&Error{Message: msg}) }

// failAt aborts the current phase with an error positioned at n.
func failAt(msg string, n node) {
	p := n.base().pos
	panic(&Error{Message: msg, at: &p})
}

// reportBug aborts the translation as PcalDebug.ReportBug does.
func reportBug(msg string) {
	panic(&bugError{msg: "You have discovered a bug in pcal.trans.\n" +
		"Send the following information and the\ninput file to the current maintainer(s).\n\n -- " + msg + "."})
}

func assertBug(ok bool, msg string) {
	if !ok {
		reportBug("Failure of assertion: " + msg)
	}
}

// catch runs f and converts a translation panic into an error. Runtime
// errors are converted with onRuntime when it is not nil; other panics are
// propagated.
func catch(f func(), onRuntime func() *Error) (err *Error) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		err = recovered(r, onRuntime)
	}()
	f()
	return nil
}

func recovered(r any, onRuntime func() *Error) *Error {
	if e, ok := r.(*Error); ok {
		return e
	}
	var re runtime.Error
	if err, ok := r.(error); ok && errors.As(err, &re) && onRuntime != nil {
		return onRuntime()
	}
	panic(r)
}

// rethrow reports an error from a nested phase without its position, as the
// Java code does when it wraps an exception message in a new exception.
func rethrow(f func()) {
	if err := catch(f, nil); err != nil {
		fail(err.Message)
	}
}
