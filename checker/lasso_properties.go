package checker

import (
	"fmt"
	"slices"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
)

// reportLassoProperties evaluates each configured temporal property on the
// selected infinite behavior, not on other possible cycles in the graph.
func (s *search) reportLassoProperties() (Result, error) {
	v := s.result.Violation
	if !validLasso(v) {
		return Result{}, fmt.Errorf("invalid liveness counterexample")
	}
	if err := s.collapseGeneralTrace(v); err != nil {
		return Result{}, err
	}
	if err := s.replayLassoEffects(v); err != nil {
		return Result{}, err
	}
	names, err := s.completedSearchLassoNames(v)
	if err != nil {
		return Result{}, err
	}
	if len(names) == 0 {
		return s.reportLassoWithoutNames(v)
	}
	v.Properties = names
	v.Name = names[0]
	return s.result, nil
}

func (s *search) reportLassoWithoutNames(v *Violation) (Result, error) {
	// LiveCheck1 can evaluate a new state's predicate on its predecessor.
	// The displayed lasso can thus satisfy the original formula on replay.
	if s.simulationStabilityViolation(v.Name) {
		v.Properties = []string{v.Name}
		return s.result, nil
	}
	if s.lassoRuntimeMayChangeVerdict() || s.model.hasReduction() {
		v.Name, v.Properties = "", []string{}
		return s.result, nil
	}
	return Result{}, fmt.Errorf("liveness counterexample violates no property")
}

func (s *search) simulationStabilityViolation(name string) bool {
	return s.simulationCheckStates != nil && slices.ContainsFunc(s.model.livenessOrder, func(property temporalProperty) bool {
		return property.name == name && property.mode == eventuallyAlways
	})
}

func validLasso(violation *Violation) bool {
	return violation != nil && violation.LoopStart != nil && *violation.LoopStart >= 0 && *violation.LoopStart < len(violation.Trace)
}

func (s *search) completedSearchLassoNames(violation *Violation) ([]string, error) {
	runtime := s.model.tlcRuntime
	previousGraph, previousLevel, previousCurrent := runtime.levelGraph, runtime.level, runtime.traceCurrent
	if previousGraph != nil && s.lassoRuntimeMayChangeVerdict() {
		runtime.levelGraph, runtime.level, runtime.traceCurrent = nil, 0, -1
		defer func() {
			runtime.levelGraph, runtime.level, runtime.traceCurrent = previousGraph, previousLevel, previousCurrent
		}()
	}
	return s.violatedLassoNames(violation)
}

// TLC checks recurrence predicates while expanding the graph but names
// violated properties after the search, when runtime values may have changed.
func (s *search) lassoRuntimeMayChangeVerdict() bool {
	if s.model.tlcRuntime.levelGraph == nil {
		return false
	}
	return slices.ContainsFunc(s.model.livenessOrder, s.model.propertyUsesTLC)
}

func (s *search) violatedLassoNames(v *Violation) ([]string, error) {
	names := make([]string, 0, len(s.model.cfg.Properties))
	seen := map[string]bool{}
	for _, property := range s.model.livenessOrder {
		if seen[property.name] {
			continue
		}
		violated, err := s.lassoViolates(property, v)
		if err != nil {
			return nil, err
		}
		if violated {
			seen[property.name] = true
			names = append(names, property.name)
		}
	}
	return names, nil
}

func (s *search) lassoViolates(property temporalProperty, violation *Violation) (bool, error) {
	if property.mode == generalTemporal {
		return s.lassoViolatesGeneral(property, violation)
	}
	if property.mode == guardedAlways {
		return s.guardedBehaviorViolates(property, violation.Trace, *violation.LoopStart)
	}
	if property.mode == orderedEventually {
		return s.lassoViolatesOrder(property, violation)
	}
	return s.lassoViolatesOther(property, violation)
}

func (s *search) lassoViolatesOther(property temporalProperty, violation *Violation) (bool, error) {
	if property.mode == leadsToStaying {
		return s.lassoViolatesLeadsToStaying(property, violation)
	}
	if property.mode == staysOrOftenState || property.mode == staysOrOftenAction {
		return s.lassoViolatesStaysOrOften(property, violation)
	}
	if property.mode == oftenImpliesState || property.mode == oftenImpliesAction {
		return s.lassoViolatesOftenImplication(property, violation)
	}
	if property.mode == actionOften {
		return s.lassoViolatesAction(property, violation)
	}
	return s.lassoViolatesState(property, violation)
}

func (s *search) lassoViolatesState(p temporalProperty, v *Violation) (bool, error) {
	summary := lassoStateSummary{allLoopFalse: true}
	for i, step := range v.Trace {
		satisfied, err := summary.observe(s, p, step.State, i >= *v.LoopStart)
		if err != nil {
			return false, err
		}
		if satisfied {
			return false, nil
		}
	}
	return lassoStateVerdict(p.mode, summary.allLoopFalse, summary.someLoopFalse, summary.pending), nil
}

type lassoStateSummary struct{ allLoopFalse, someLoopFalse, pending bool }

func (summary *lassoStateSummary) observe(s *search, p temporalProperty, state evaluator.State, inLoop bool) (bool, error) {
	if err := s.model.ctx.Err(); err != nil {
		return false, err
	}
	holds, err := s.model.holdsTemporalState(p, state)
	if err != nil {
		return false, err
	}
	if inLoop {
		summary.allLoopFalse = summary.allLoopFalse && !holds
		summary.someLoopFalse = summary.someLoopFalse || !holds
	}
	if p.mode == leadsTo {
		summary.pending, err = s.lassoPendingTrigger(p, state, holds, summary.pending)
	}
	return p.mode == eventually && holds, err
}

func (s *search) lassoPendingTrigger(p temporalProperty, state evaluator.State, holds, pending bool) (bool, error) {
	trigger, err := s.model.holdsProperty(p.trigger, state, p.locals)
	return !holds && (pending || trigger), err
}

func lassoStateVerdict(mode temporalMode, allLoopFalse, someLoopFalse, pending bool) bool {
	switch mode {
	case eventually, often:
		return allLoopFalse
	case eventuallyAlways:
		return someLoopFalse
	case leadsTo:
		return allLoopFalse && pending
	}
	return false
}

func (s *search) lassoViolatesAction(p temporalProperty, v *Violation) (bool, error) {
	loop := *v.LoopStart
	for i := loop; i < len(v.Trace); i++ {
		ok, err := s.holdsLassoAction(p.predicate, p.locals, v, i)
		if err != nil || ok {
			return false, err
		}
	}
	return true, nil
}

func (s *search) holdsLassoAction(expr *parser.Expression, locals map[string]value.Value, v *Violation, index int) (bool, error) {
	next := v.Trace[*v.LoopStart].State
	if index+1 < len(v.Trace) {
		next = v.Trace[index+1].State
	}
	ctx := evaluator.Context{Current: v.Trace[index].State, Next: next, Constants: s.model.constants, Locals: locals, Enabled: s.model.enabledCallback}
	result, err := s.model.program.Evaluate(expr, ctx)
	if err != nil {
		return false, err
	}
	ok, valid := result.Bool()
	if !valid {
		return false, fmt.Errorf("temporal action predicate must be boolean, got %s", result)
	}
	return ok, nil
}
