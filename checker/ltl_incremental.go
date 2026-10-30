package checker

import (
	"errors"
	"fmt"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
)

type ltlValueKey struct {
	formula *ltlFormula
	base    int
	choice  int
}

type ltlCheckGroup struct {
	actions []*ltlFormula
	states  []*ltlFormula
}

type fairCachedChecks struct {
	enabled, action *ltlFormula
}

func findFairCachedChecks(formula *ltlFormula) fairCachedChecks {
	if formula == nil {
		return fairCachedChecks{}
	}
	if formula.fairEnabled {
		return fairCachedChecks{enabled: formula}
	}
	if formula.fairness {
		return fairCachedChecks{action: formula}
	}
	left := findFairCachedChecks(formula.left)
	right := findFairCachedChecks(formula.right)
	if left.enabled == nil {
		left.enabled = right.enabled
	}
	if left.action == nil {
		left.action = right.action
	}
	return left
}

// Recurrence action checks are evaluated when the source state's successors
// become known, as in TLC's live graph construction.
func (s *search) prepareIncrementalLiveness() {
	assumptions := s.incrementalAssumptions()
	for _, property := range s.model.livenessOrder {
		groups := s.propertyIncrementalGroups(property)
		if len(assumptions) != 0 {
			groups = mergeIncrementalGroups(assumptions, groups)
		}
		if hasIncrementalChecks(groups) {
			s.ltlGroups = append(s.ltlGroups, groups...)
		}
	}
	if len(s.ltlGroups) != 0 {
		s.ltlValues = map[ltlValueKey]bool{}
		s.ltlSetMultiplier = 1
		s.ltlFingerprints = map[int]uint64{}
		s.ltlVariableOrder = s.model.symmetryVariableOrder()
	}
}

func (s *search) propertyIncrementalGroups(property temporalProperty) []ltlCheckGroup {
	if property.mode == generalTemporal && hasFairnessAction(property.general) && !s.model.propertyUsesUnstableTLC(property) {
		return s.incrementalGroups(property.general, false)
	}
	if property.cacheAtom != nil {
		return []ltlCheckGroup{property.recurrenceCacheGroup()}
	}
	return []ltlCheckGroup{{}}
}

func (m *model) prepareRecurrenceCache(property *temporalProperty) {
	if property.mode != often && property.mode != eventuallyAlways {
		return
	}
	if !m.propertyUsesTLC(*property) || m.unsafeIncrementalExpression(property.predicate) {
		return
	}
	property.cacheAtom = &ltlFormula{kind: ltlAtom, atom: property.predicate, locals: property.locals, baseLevel: true}
}

func (property temporalProperty) recurrenceCacheGroup() ltlCheckGroup {
	if property.mode == often {
		return ltlCheckGroup{actions: []*ltlFormula{property.cacheAtom}}
	}
	return ltlCheckGroup{states: []*ltlFormula{property.cacheAtom}}
}

func (s *search) incrementalAssumptions() []ltlCheckGroup {
	if len(s.model.fairness) == 0 {
		return nil
	}
	groups := []ltlCheckGroup{{}}
	s.fairChecks = make([]fairCachedChecks, len(s.model.fairness))
	for index, fair := range s.model.fairness {
		effects := s.model.fairnessHasEffects(fair)
		formula := s.incrementalFairnessFormula(fair, effects)
		if formula != nil {
			if effects {
				s.fairChecks[index] = findFairCachedChecks(formula)
			}
			groups = mergeIncrementalGroups(groups, s.incrementalGroups(formula, effects))
		}
	}
	if !hasIncrementalChecks(groups) {
		return nil
	}
	return groups
}

func hasIncrementalChecks(groups []ltlCheckGroup) bool {
	for _, group := range groups {
		if len(group.actions)+len(group.states) != 0 {
			return true
		}
	}
	return false
}

func (s *search) incrementalFairnessFormula(fair fairnessCondition, effects bool) *ltlFormula {
	if !s.canCacheFairness(fair, effects) {
		return nil
	}
	c := newLTLCompiler(s.model, fair.locals)
	if fair.recurrence {
		kind := ltlAtom
		if fair.recurrenceAction {
			kind = ltlAction
		}
		return c.fairnessRecurrence(c.intern(kind, nil, nil, fair.action, false), true)
	}
	kind := "WeakFairness"
	if fair.strong {
		kind = "StrongFairness"
	}
	expr := &parser.Expression{Kind: kind, Args: []*parser.Expression{fair.subscript, fair.action}, Location: fair.action.Location}
	formula, err := c.compileFairness(expr, false)
	if err != nil {
		return nil
	}
	return formula
}

func (s *search) canCacheFairness(fair fairnessCondition, effects bool) bool {
	if fair.recurrence && effects {
		return false
	}
	return !s.model.unstableFairExpression(fair.action) && !s.model.unstableFairExpression(fair.subscript)
}

func (m *model) fairnessHasEffects(fair fairnessCondition) bool {
	return m.expressionHasEffects(fair.action, map[string]bool{}) || m.expressionHasEffects(fair.subscript, map[string]bool{})
}

func (m *model) unstableFairExpression(expr *parser.Expression) bool {
	return m.expressionUsesUnstableTLC(expr, map[string]bool{}) || m.readsRandom(expr, map[string]bool{})
}

func (m *model) unsafeIncrementalExpression(expr *parser.Expression) bool {
	return m.expressionUsesUnstableTLC(expr, map[string]bool{}) || m.expressionHasEffects(expr, map[string]bool{}) || m.readsRandom(expr, map[string]bool{})
}

func (s *search) incrementalGroups(formula *ltlFormula, effects bool) []ltlCheckGroup {
	if formula == nil {
		return []ltlCheckGroup{{}}
	}
	if formula.kind == ltlOr {
		return append(s.incrementalGroups(formula.left, effects), s.incrementalGroups(formula.right, effects)...)
	}
	if formula.kind == ltlAnd {
		return mergeIncrementalGroups(s.incrementalGroups(formula.left, effects), s.incrementalGroups(formula.right, effects))
	}
	return []ltlCheckGroup{s.incrementalGroup(formula, effects)}
}

func (s *search) incrementalGroup(formula *ltlFormula, effects bool) ltlCheckGroup {
	if !isFairnessRecurrence(formula) || !s.safeIncrementalCheck(formula.left.left, effects) {
		return ltlCheckGroup{}
	}
	body := formula.left.left
	if formula.kind == ltlEventually || containsLTLAction(body) {
		return ltlCheckGroup{actions: []*ltlFormula{body}}
	}
	return ltlCheckGroup{states: []*ltlFormula{body}}
}

func mergeIncrementalGroups(left, right []ltlCheckGroup) []ltlCheckGroup {
	result := make([]ltlCheckGroup, 0, len(left)*len(right))
	for _, l := range left {
		for _, r := range right {
			result = append(result, ltlCheckGroup{actions: append(append([]*ltlFormula(nil), l.actions...), r.actions...), states: append(append([]*ltlFormula(nil), l.states...), r.states...)})
		}
	}
	return result
}

func containsLTLAction(formula *ltlFormula) bool {
	return formula != nil && (formula.kind == ltlAction || containsLTLAction(formula.left) || containsLTLAction(formula.right))
}

func (s *search) safeIncrementalCheck(formula *ltlFormula, effects bool) bool {
	if formula == nil {
		return true
	}
	if formula.atom != nil && ((!effects && s.model.unsafeIncrementalExpression(formula.atom)) || (effects && s.model.expressionUsesUnstableTLC(formula.atom, map[string]bool{}))) {
		return false
	}
	return s.safeIncrementalCheck(formula.left, effects) && s.safeIncrementalCheck(formula.right, effects)
}

func (s *search) checkIncrementalLiveness(index int) (bool, error) {
	if len(s.ltlGroups) == 0 {
		return true, nil
	}
	previous := s.model.tlcRuntime.traceCurrent
	s.model.tlcRuntime.traceCurrent = index
	defer func() { s.model.tlcRuntime.traceCurrent = previous }()
	choices, err := s.incrementalSuccessors(index)
	if err != nil {
		return false, err
	}
	s.debugLivenessSuccessors(index, choices)
	for _, group := range s.ltlGroups {
		if err := s.evaluateIncrementalGroup(group, index, choices); err != nil {
			return s.incrementalFailure(err, index)
		}
	}
	return true, nil
}

func (s *search) evaluateIncrementalGroup(group ltlCheckGroup, index int, choices []int) error {
	for _, choice := range choices {
		for _, check := range group.actions {
			if _, err := s.evaluateIncrementalCheck(check, index, choice); err != nil {
				return err
			}
		}
	}
	for _, check := range group.states {
		if _, err := s.evaluateIncrementalCheck(check, index, choices[0]); err != nil {
			return err
		}
	}
	return nil
}

func (s *search) evaluateIncrementalCheck(formula *ltlFormula, index, choice int) (bool, error) {
	switch formula.kind {
	case ltlTrue:
		return true, nil
	case ltlFalse:
		return false, nil
	case ltlAtom, ltlAction:
		return s.evaluateIncrementalAtom(formula, index, choice)
	case ltlAnd, ltlOr:
		return s.evaluateIncrementalJunction(formula, index, choice)
	default:
		return false, fmt.Errorf("invalid incremental liveness check")
	}
}

func (s *search) evaluateIncrementalJunction(formula *ltlFormula, index, choice int) (bool, error) {
	left, err := s.evaluateIncrementalCheck(formula.left, index, choice)
	if err != nil || (formula.kind == ltlAnd && !left) || (formula.kind == ltlOr && left) {
		return left, err
	}
	return s.evaluateIncrementalCheck(formula.right, index, choice)
}

func (s *search) evaluateIncrementalAtom(formula *ltlFormula, index, choice int) (bool, error) {
	key := ltlValueKey{formula: formula, base: index, choice: choice}
	if formula.kind == ltlAtom {
		key.choice = -1
	}
	if value, ok := s.ltlValues[key]; ok {
		return value, nil
	}
	next := s.edgeNext(s.edges[index][choice])
	result, err := s.model.holdsLTLAtom(formula, s.nodes[index].state, next, formula.locals)
	if err != nil {
		return false, &incrementalLivenessError{cause: err, formula: formula}
	}
	result = result != formula.negative
	s.ltlValues[key] = result
	return result, nil
}

type incrementalLivenessError struct {
	cause   error
	formula *ltlFormula
}

func (e *incrementalLivenessError) Error() string { return e.cause.Error() }
func (e *incrementalLivenessError) Unwrap() error { return e.cause }

func (s *search) incrementalFailure(err error, index int) (bool, error) {
	wrapped, ok := errors.AsType[*incrementalLivenessError](err)
	if !ok {
		return false, err
	}
	message, code, ok := incrementalFailureDetails(wrapped)
	if !ok {
		return false, err
	}
	trace, err := s.recoverFailureTrace(index)
	if err != nil {
		return false, err
	}
	s.result.Status = EvaluationFailure
	s.result.Failure = &Failure{Message: message, Trace: trace, ExitCode: code}
	s.result.OutDegrees, s.result.HasOutDegrees = s.outDegrees, true
	return false, nil
}

func incrementalFailureDetails(err *incrementalLivenessError) (string, int, bool) {
	if missing, ok := errors.AsType[*evaluator.UnboundVariable](err.cause); ok {
		location := err.formula.atom.Location
		return fmt.Sprintf("The variable %s is not assigned at line %d, column %d", missing.Name, location.Line, location.Column), 255, true
	}
	if failure, ok := errors.AsType[*evaluator.RuntimeError](err.cause); ok {
		code := 255
		if failure.ExitCode != 0 {
			code = failure.ExitCode
		}
		return failure.Message, code, true
	}
	return "", 0, false
}
