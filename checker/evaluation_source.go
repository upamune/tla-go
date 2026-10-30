package checker

import (
	"errors"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

// Action decomposition skips the outer expression when evaluating a guard.
// Add it back only when it encloses the outermost recorded evaluation frame.
func addEnclosingActionSource(err error, source *parser.Expression) {
	failure, ok := errors.AsType[*evaluator.RuntimeError](err)
	if !ok || failure.Fingerprint != nil || source == nil {
		return
	}
	frames := failure.Sources
	if failure.Unexpected != nil {
		frames = failure.Unexpected.Sources
	}
	if len(frames) == 0 {
		return
	}
	if enclosesSource(source.Location.Range, frames[len(frames)-1].Location.Range) {
		evaluator.AddUnexpectedSource(err, source)
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
