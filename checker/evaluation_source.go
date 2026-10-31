package checker

import (
	"errors"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

// Action decomposition skips the outer expression when evaluating a guard.
// Add it back only when it encloses the outermost recorded evaluation frame.
func addEnclosingActionSource(err error, source *parser.Expression) {
	if source == nil {
		return
	}
	if _, ok := errors.AsType[*evaluator.UnboundVariable](err); ok && source.Kind == "Or" {
		return
	}
	frames := actionErrorSources(err)
	if len(frames) == 0 {
		return
	}
	if enclosesSource(source.Location.Range, frames[len(frames)-1].Location.Range) {
		evaluator.AddUnexpectedSource(err, source)
	}
}

// actionRoot returns the outermost frame Java TLC keeps for a successor error
// in next, or nil when its action decomposition splits next into disjuncts or
// existential witnesses, which are then the outermost frames.
func (g *generator) actionRoot(next *parser.Expression) *parser.Expression {
	if next.Kind == "Or" || next.Kind == "Exists" && g.actionExistentialCanSplit(next) {
		return nil
	}
	return next
}

func actionErrorSources(err error) []*parser.Expression {
	if missing, ok := errors.AsType[*evaluator.UnboundVariable](err); ok {
		return missing.Sources
	}
	failure, ok := errors.AsType[*evaluator.RuntimeError](err)
	if !ok || failure.Fingerprint != nil {
		return nil
	}
	if failure.Unexpected != nil {
		return failure.Unexpected.Sources
	}
	return failure.Sources
}

// Java TLC generates states by continuation passing, so a conjunct stays on
// its call stack while the remaining conjuncts are evaluated, and later
// evaluation errors include it.
func addConjunctErrorSource(err error, source *parser.Expression) {
	if failure, ok := errors.AsType[*evaluator.RuntimeError](err); ok && (failure.TrackSource || failure.Unexpected != nil) {
		evaluator.AddUnexpectedSource(err, source)
	}
}

// addInlineConjunctionSources adds the frames of SANY's left-nested binary
// conjunctions for an inline chain a /\ b /\ c, innermost first. A bulleted
// list is a single conjunction list and starts before its first conjunct.
func addInlineConjunctionSources(err error, conjunction *parser.Expression) {
	args := conjunction.Args
	if len(args) < 3 || conjunction.Location.Range.Start != args[0].Location.Range.Start {
		return
	}
	frames := actionErrorSources(err)
	if len(frames) == 0 || frames[len(frames)-1] != args[0] {
		return
	}
	for _, last := range args[1 : len(args)-1] {
		chain := *conjunction
		chain.Args = nil
		chain.Location.Range.End = last.Location.Range.End
		addConjunctErrorSource(err, &chain)
	}
}

func addUnboundActionCallSource(err error, call, body *parser.Expression) {
	missing, ok := errors.AsType[*evaluator.UnboundVariable](err)
	if !ok || len(missing.Sources) == 0 || body == nil {
		return
	}
	last := missing.Sources[len(missing.Sources)-1].Location.Range
	if body.Location.Range == last || enclosesSource(body.Location.Range, last) {
		evaluator.AddUnexpectedSource(err, call)
	}
}

func enclosesSource(outer, inner parser.Range) bool {
	if outer == inner || outer.Module != inner.Module || outer.Start.Line == 0 {
		return false
	}
	return positionBefore(outer.Start, inner.Start) && positionBefore(inner.End, outer.End)
}

func positionBefore(a, b parser.Position) bool {
	return a.Line < b.Line || a.Line == b.Line && a.Column <= b.Column
}

func (g *generator) addInitialFailureSource(source *parser.Expression, assignment bool) {
	if !g.initial || g.model.tlcRuntime.search == nil || source == nil {
		return
	}
	failure := g.model.tlcRuntime.search.result.Failure
	if failure == nil || len(failure.evaluationSources) == 0 {
		return
	}
	last := failure.evaluationSources[len(failure.evaluationSources)-1]
	if assignment || enclosesSource(source.Location.Range, last.Location.Range) {
		failure.evaluationSources = append(failure.evaluationSources, source)
	}
}

func (g *generator) addInitialExpandedSource(call, body *parser.Expression) {
	if !g.initial || g.model.tlcRuntime.search == nil || body == nil {
		return
	}
	failure := g.model.tlcRuntime.search.result.Failure
	if failure == nil || len(failure.evaluationSources) == 0 {
		return
	}
	last := failure.evaluationSources[len(failure.evaluationSources)-1].Location.Range
	if body.Location.Range == last || enclosesSource(body.Location.Range, last) {
		g.addInitialFailureSource(call, true)
	}
}

func (s *search) finishInitialEvaluationDetail() {
	failure := s.result.Failure
	if failure != nil && len(failure.evaluationSources) > 0 {
		failure.EvaluationDetail = evaluationDetail(&evaluator.RuntimeError{Sources: failure.evaluationSources})
	}
}
