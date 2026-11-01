package checker

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/upamune/tla-go/evaluator"
	"github.com/upamune/tla-go/syntax/parser"
	"github.com/upamune/tla-go/value"
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
		s.ltlPureStateOnly = s.pureRecurrenceGroups()
		if !s.ltlPureStateOnly {
			s.ltlValues = map[ltlValueKey]bool{}
			s.ltlSetMultiplier = 1
			s.ltlFingerprints = map[int]uint64{}
			s.ltlVariableOrder = s.model.symmetryVariableOrder()
		}
	}
	s.fairIncrementalOneSolution = s.canEvaluateFairIncrementally()
}

func (s *search) pureRecurrenceGroups() bool {
	if len(s.model.livenessOrder) != 1 || len(s.model.fairness) != 0 || s.model.hasReduction() || s.model.debug != nil {
		return false
	}
	for _, property := range s.model.livenessOrder {
		if property.cacheAtom == nil || s.model.propertyUsesTLC(property) {
			return false
		}
	}
	return true
}

// Re-evaluation must retain separate results for each Java liveness solution.
// Until the cache supports that, restrict this path to a single non-tableau
// solution without reductions or mutable TLC state in the fairness expression.
func (s *search) canEvaluateFairIncrementally() bool {
	m := s.model
	if !s.basicFairIncrementalEligibility() {
		return false
	}
	fair := m.fairness[0]
	if m.fairnessUsesTLCState(fair.action, map[string]bool{}) || m.fairnessUsesTLCState(fair.subscript, map[string]bool{}) {
		return false
	}
	branches, tableau := m.progressBranches()
	return branches == 1 && !tableau
}

func (s *search) basicFairIncrementalEligibility() bool {
	m := s.model
	return len(s.ltlGroups) == 1 && len(m.fairness) == 1 && s.hasCachedFairActions() && !m.hasReduction() && !hasOperatorConstants(m.module.Constants) && m.safeLivenessNormalization()
}

func hasOperatorConstants(constants []parser.Declaration) bool {
	for _, constant := range constants {
		if constant.Arity > 0 {
			return true
		}
	}
	return false
}

func (m *model) fairnessUsesTLCState(expr *parser.Expression, seen map[string]bool) bool {
	if expr == nil {
		return false
	}
	if isUnstableFairOperator(expr) || hasHigherOrderDefinitions(expr.Definitions) || m.fairnessDefinitionUsesTLCState(expr, seen) {
		return true
	}
	found := false
	walkChildren(expr, func(child *parser.Expression) {
		if !found {
			found = m.fairnessUsesTLCState(child, seen)
		}
	})
	return found
}

func isUnstableFairOperator(expr *parser.Expression) bool {
	if expr.Kind != "Apply" && expr.Kind != "Identifier" {
		return false
	}
	switch tlcOperatorName(expr) {
	case "TLCSet", "TLCGet", "TLCGetOrDefault", "TLCGetAndSet", "JavaTime",
		"RandomElement", "RandomSubset", "RandomSetOfSubsets", "TestRandomSetOfSubsets":
		return true
	}
	return false
}

func (m *model) fairnessDefinitionUsesTLCState(expr *parser.Expression, seen map[string]bool) bool {
	if expr.Kind != "Apply" && expr.Kind != "Identifier" {
		return false
	}
	def, ok := m.defs[expr.Text]
	if !ok || seen[expr.Text] {
		return false
	}
	if hasHigherOrderParameters(def) {
		return true
	}
	seen[expr.Text] = true
	defer delete(seen, expr.Text)
	return m.fairnessUsesTLCState(def.Body, seen)
}

func hasHigherOrderDefinitions(defs []parser.Definition) bool {
	return slices.ContainsFunc(defs, hasHigherOrderParameters)
}

func hasHigherOrderParameters(def parser.Definition) bool {
	for _, param := range def.Params {
		if param.Arity > 0 {
			return true
		}
	}
	return false
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
	if m.unsafeIncrementalExpression(property.predicate) {
		return
	}
	pure := !m.propertyUsesTLC(*property)
	if pure && (!m.cachedPureRecurrenceSourceSafe() || m.recurrenceHasConditional(property.predicate, map[string]bool{}) || m.recurrenceHasUnstableFairness()) {
		return
	}
	property.cacheAtom = &ltlFormula{kind: ltlAtom, atom: property.predicate, locals: property.locals, baseLevel: true, pureRecurrence: pure}
}

// TLC can evaluate an IF branch while normalizing liveness before exploration.
// Until that order is implemented, keep conditional predicates on the old path.
func (m *model) recurrenceHasConditional(expr *parser.Expression, seen map[string]bool) bool {
	if expr == nil {
		return false
	}
	if expr.Kind == "If" {
		return true
	}
	if m.recurrenceDefinitionHasConditional(expr, seen) {
		return true
	}
	found := false
	walkChildren(expr, func(child *parser.Expression) {
		if !found {
			found = m.recurrenceHasConditional(child, seen)
		}
	})
	return found
}

func (m *model) recurrenceDefinitionHasConditional(expr *parser.Expression, seen map[string]bool) bool {
	if expr.Kind != "Apply" && expr.Kind != "Identifier" {
		return false
	}
	def, ok := m.defs[expr.Text]
	if !ok {
		return false
	}
	if cached, ok := m.cachedConditionalDefinition(expr.Text); ok {
		return cached
	}
	if seen[expr.Text] {
		return true
	}
	seen[expr.Text] = true
	found := m.recurrenceHasConditional(def.Body, seen)
	delete(seen, expr.Text)
	m.rememberConditionalDefinition(expr.Text, found)
	return found
}

type recurrenceSourceCache struct {
	checked     bool
	safe        bool
	conditional map[string]bool
}

func (m *model) cachedConditionalDefinition(name string) (bool, bool) {
	if m.recurrenceSourceCache == nil {
		return false, false
	}
	result, ok := m.recurrenceSourceCache.conditional[name]
	return result, ok
}

func (m *model) rememberConditionalDefinition(name string, result bool) {
	cache := m.recurrenceSourceCache
	if cache == nil {
		return
	}
	if cache.conditional == nil {
		cache.conditional = map[string]bool{}
	}
	cache.conditional[name] = result
}

func (m *model) cachedPureRecurrenceSourceSafe() bool {
	cache := m.recurrenceSourceCache
	if cache == nil {
		return m.pureRecurrenceSourceSafe()
	}
	if !cache.checked {
		cache.safe = m.pureRecurrenceSourceSafe()
		cache.checked = true
	}
	return cache.safe
}

// Only reuse the liveness-DOT purity check when the original module and
// configuration are available. A missing or unrecognized source falls back.
func (m *model) pureRecurrenceSourceSafe() bool {
	return m.pureLivenessDumpModel() && !hasOperatorConstants(m.module.Constants) && len(m.sourceConfig.Substitutions) == 0 && len(m.cfg.Substitutions) == 0
}

func (m *model) recurrenceHasUnstableFairness() bool {
	return slices.ContainsFunc(m.fairness, func(fair fairnessCondition) bool {
		return m.fairnessHasEffects(fair) || m.fairnessUsesTLCState(fair.action, map[string]bool{}) || m.fairnessUsesTLCState(fair.subscript, map[string]bool{})
	})
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
	if s.ltlPureStateOnly {
		return s.checkPureRecurrences(index)
	}
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

func (s *search) checkPureRecurrences(index int) (bool, error) {
	formula := s.model.livenessOrder[0].cacheAtom
	result, err := s.holdsPureRecurrence(formula, s.nodes[index].state)
	if err != nil {
		return s.incrementalFailure(&incrementalLivenessError{cause: err, formula: formula}, index)
	}
	for len(s.ltlPureStateValues) <= index {
		s.ltlPureStateValues = append(s.ltlPureStateValues, 0)
	}
	s.ltlPureStateValues[index] = 1
	if result {
		s.ltlPureStateValues[index] = 2
	}
	return true, nil
}

func (s *search) cachedPureRecurrence(property temporalProperty, index int) (bool, bool) {
	if !s.ltlPureStateOnly || property.cacheAtom != s.model.livenessOrder[0].cacheAtom || index >= len(s.ltlPureStateValues) || s.ltlPureStateValues[index] == 0 {
		return false, false
	}
	return s.ltlPureStateValues[index] == 2, true
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
	effectful := s.effectfulFairEnabled(formula)
	if formula.kind == ltlAtom {
		key.choice = -1
	}
	if !effectful {
		if value, ok := s.ltlValues[key]; ok {
			return value, nil
		}
	}
	next := s.edgeNext(s.edges[index][choice])
	result, err := s.holdsIncrementalAtom(formula, s.nodes[index].state, next)
	if err != nil {
		return false, &incrementalLivenessError{cause: err, formula: formula}
	}
	result = result != formula.negative
	s.ltlValues[key] = result
	return result, nil
}

func (s *search) holdsIncrementalAtom(formula *ltlFormula, current, next evaluator.State) (bool, error) {
	if formula.pureRecurrence {
		return s.holdsPureRecurrence(formula, current)
	}
	if !s.fairIncrementalOneSolution || !formula.fairness || formula.atom.Kind != "AngleAction" {
		return s.model.holdsLTLAtom(formula, current, next, formula.locals)
	}
	m := s.model
	context := evaluator.Context{Current: current, Next: next, Constants: m.constants, Locals: formula.locals, Enabled: m.enabledCallback}
	return m.holdsFairIncrementalAngle(formula, context)
}

func (s *search) holdsPureRecurrence(formula *ltlFormula, current evaluator.State) (bool, error) {
	m := s.model
	result, err := m.program.Evaluate(formula.atom, evaluator.Context{Current: current, Constants: m.constants, Locals: formula.locals, Enabled: m.enabledCallback})
	if err != nil {
		return false, err
	}
	ok, valid := result.Bool()
	if !valid {
		return false, &evaluator.RuntimeError{Message: "A state predicate was evaluated to a non-boolean value.", ToolCode: 2250}
	}
	return ok, nil
}

func (m *model) holdsFairIncrementalAngle(formula *ltlFormula, context evaluator.Context) (bool, error) {
	currentContext := context
	currentContext.Next = nil
	a, err := m.program.Evaluate(formula.atom.Args[1], currentContext)
	if err != nil {
		return false, err
	}
	currentContext.Current = context.Next
	b, err := m.program.Evaluate(formula.atom.Args[1], currentContext)
	if err != nil {
		return false, err
	}
	same, err := value.Equal(a, b)
	if err != nil || same {
		return false, err
	}
	v, err := m.program.Evaluate(formula.atom.Args[0], context)
	if err != nil {
		return false, err
	}
	result, ok := v.Bool()
	if !ok {
		return false, fmt.Errorf("temporal action predicate must be boolean, got %s", v)
	}
	return result, nil
}

func (s *search) effectfulFairEnabled(formula *ltlFormula) bool {
	if !s.fairIncrementalOneSolution || !formula.fairEnabled {
		return false
	}
	for _, check := range s.fairChecks {
		if check.enabled == formula {
			return true
		}
	}
	return false
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
	if wrapped.formula.pureRecurrence {
		return s.pureRecurrenceFailure(wrapped)
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

func (s *search) pureRecurrenceFailure(err *incrementalLivenessError) (bool, error) {
	failure, ok := asRuntimeError(err.cause)
	if !ok {
		if errors.Is(err.cause, context.Canceled) || errors.Is(err.cause, context.DeadlineExceeded) {
			return false, err
		}
		failure = &evaluator.RuntimeError{Message: err.cause.Error()}
	}
	reported := generalFailure("", failure)
	reported.ExitCode = 255
	s.result.Status = EvaluationFailure
	s.result.Failure = reported
	// Java records the outdegree only after liveness graph construction.
	s.outDegrees[s.lastOutDegree]--
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
